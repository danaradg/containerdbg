// Copyright 2022 Google LLC All Rights Reserved.
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

package events

import (
	"github.com/go-logr/logr"
	"velostrata-internal.googlesource.com/containerdbg.git/pkg/ebpf"
	"velostrata-internal.googlesource.com/containerdbg.git/pkg/events/api"
	"velostrata-internal.googlesource.com/containerdbg.git/proto"
)

var defaultEventSources = []api.EventsSource{
	&ebpf.OpenFilesFilter{},
	&ebpf.RenameLinkFilter{},
	&ebpf.NetstateFilter{},
}

type EventsSourceManager struct {
	sources          []api.EventsSource
	aggregateChannel chan *proto.Event
}

func NewEventSourceManager() *EventsSourceManager {
	return &EventsSourceManager{
		sources:          defaultEventSources,
		aggregateChannel: make(chan *proto.Event, 10),
	}
}

func (mgr *EventsSourceManager) Load(log logr.Logger) error {
	if err := ebpf.GetManagerInstance().Init(); err != nil {
		return err
	}
	loadedSources := []api.EventsSource{}
	for _, source := range mgr.sources {
		if err := source.Load(log); err != nil {
			unloadSources(loadedSources)
			return err
		}
		loadedSources = append(loadedSources, source)
	}

	for _, source := range mgr.sources {
		go func(c <-chan *proto.Event) {
			for msg := range c {
				mgr.aggregateChannel <- msg
			}
		}(source.Events())
	}

	return nil
}

func (mgr *EventsSourceManager) Events() <-chan *proto.Event {
	return mgr.aggregateChannel
}

func (mgr *EventsSourceManager) Unload() {
	unloadSources(mgr.sources)
	close(mgr.aggregateChannel)
}

func (mgr *EventsSourceManager) RegisterContainer(nsId uint32, id *proto.SourceId) {
	ebpf.GetManagerInstance().RegisterContainer(nsId, id)
}

func unloadSources(sources []api.EventsSource) {
	for _, source := range sources {
		source.Close()
	}
}
