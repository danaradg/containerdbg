// Copyright 2021 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ebpf

import (
	"encoding/binary"
	"os"
	"time"
	"net"
	"syscall"

	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"google.golang.org/protobuf/types/known/timestamppb"
	"velostrata-internal.googlesource.com/containerdbg.git/pkg/events/api"
	"velostrata-internal.googlesource.com/containerdbg.git/proto"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc $BPF_CLANG -cflags $BPF_CFLAGS netstate netstate_filter.c -- -I./headers
//go:generate ../../hack/add_license.sh ./netstate_bpfeb.go
//go:generate ../../hack/add_license.sh ./netstate_bpfel.go

type NetstateFilter struct {
	log    logr.Logger
	objs   netstateObjects
	reader *perfReader

	tracepoints tracepointCollection
}

var _ api.EventsSource = &NetstateFilter{}

type NetstateEvent struct {
	NetNs   uint32
	PID     uint32
	TS      uint64
	Comm    [16]byte
	// SrcAddr is the local address.
	SrcAddr net.IP
	// DstAddr is the remote address.
	DstAddr net.IP
	// Last state is the state transition - should probably be changed to somthing more useful
	LastState uint16
	// AddrFam is the address family, 2 is AF_INET (IPv4), 10 is AF_INET6 (IPv6).
	AddrFam uint32
	// SrcPort is the local port (uint16 in C struct).
	// Note, network byte order is big-endian.
	SrcPort uint16
	// DstPort is the remote port (uint16 in C struct).
	// Note, network byte order is big-endian.
	DstPort uint16
}

func (o *NetstateFilter) Load(log logr.Logger) (err error) {
	o.log = log
	if err := loadNetstateObjects(&o.objs, GetManagerInstance().GetDefaultCollectionOptions()); err != nil {
		return err
	}
	defer closeOnError(&o.objs, err)

	o.tracepoints = tracepointCollection{
		Tracepoints: []tracepoint{
			{
				Group:   "sock",
				Name:    "inet_sock_set_state",
				Program: o.objs.TraceInetSockSetState,
			},
		},
	}

	err = o.tracepoints.Load()
	if err != nil {
		return err
	}

	rd, err := perf.NewReader(o.objs.Pb, os.Getpagesize()*2048)
	o.reader = NewPerfReader(o.log, rd, func(sample []byte) (*proto.Event, error) {

		event := NetstateEvent{
			NetNs:		binary.LittleEndian.Uint32(sample[:4]),
			PID:		binary.LittleEndian.Uint32(sample[4:8]),
			TS:		binary.LittleEndian.Uint64(sample[8:16]),
			LastState:	binary.LittleEndian.Uint16(sample[64:66]),
			AddrFam:	binary.LittleEndian.Uint32(sample[66:70]),
			SrcPort:	binary.BigEndian.Uint16(sample[70:72]),
			DstPort:	binary.BigEndian.Uint16(sample[72:74]),

		}
		copy(event.Comm[:], sample[16:32])
		if event.AddrFam == syscall.AF_INET {
			event.SrcAddr = sample[32:36]
			event.DstAddr = sample[48:52]
		} else {
			event.SrcAddr = sample[32:48]
			event.DstAddr = sample[48:64]
		}

		comm := cleanAfterNull(byteSlice2String(event.Comm[:]))

		outputEvent := proto.Event{}
		outputEvent.Source = GetManagerInstance().GetId(event.NetNs)
		outputEvent.Timestamp = timestamppb.New(time.Unix(0, int64(event.TS)))
		outputEvent.EventType = &proto.Event_Network{
			Network: &proto.Event_NetworkEvent{
				Comm:    comm,
				SrcAddr: event.SrcAddr.String(),
				DstAddr: event.DstAddr.String(),
				SrcPort: int32(event.SrcPort),
				DstPort: int32(event.DstPort),
				AddrFam: int32(event.AddrFam),
				EventType: proto.Event_NetworkEvent_LISTEN, // fixme
			},
		}
		return &outputEvent, nil
	})
	if err != nil {
		return
	}

	o.reader.Start()

	return nil
}

func (o *NetstateFilter) Events() <-chan *proto.Event {
	return o.reader.Events()
}

func (o *NetstateFilter) Close() error {
	if o.reader != nil {
		o.reader.Stop()
	}
	o.tracepoints.Unload()
	return o.objs.Close()
}

