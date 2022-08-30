containerdbg
============

TBD

Compiling Requirments
=====================

libbpf-dev
ko

Technical details
=================

BPF CO-RE (Compile Once – Run Everywhere)
-----------------------------------------

Libbpf supports building BPF CO-RE-enabled applications, which, in contrast to
[BCC](https://github.com/iovisor/bcc/), do not require Clang/LLVM runtime
being deployed to target servers and doesn't rely on kernel-devel headers
being available.

It does rely on kernel to be built with [BTF type
information](https://www.kernel.org/doc/html/latest/bpf/btf.html), though.
Some major Linux distributions come with kernel BTF already built in:
  - Fedora 31+
  - RHEL 8.2+
  - OpenSUSE Tumbleweed (in the next release, as of 2020-06-04)
  - Arch Linux (from kernel 5.7.1.arch1-1)
  - Manjaro (from kernel 5.4 if compiled after 2021-06-18)
  - Ubuntu 20.10
  - Debian 11 (amd64/arm64)

If your kernel doesn't come with BTF built-in, you'll need to build custom
kernel. You'll need:
  - `pahole` 1.16+ tool (part of `dwarves` package), which performs DWARF to
    BTF conversion;
  - kernel built with `CONFIG_DEBUG_INFO_BTF=y` option;
  - you can check if your kernel has BTF built-in by looking for
    `/sys/kernel/btf/vmlinux` file:

vmlinux.h generation
-------------------

vmlinux.h contains all kernel types, both exported and internal-only. BPF
CO-RE-based applications are expected to include this file in their BPF
program C source code to avoid dependency on kernel headers package.

For more reproducible builds, vmlinux.h header file is pre-generated and
checked in along the other sources. This is done to avoid dependency on
specific user/build server's kernel configuration, because vmlinux.h
generation depends on having a kernel with BTF type information built-in
(which is enabled by `CONFIG_DEBUG_INFO_BTF=y` Kconfig option See below).

vmlinux.h is generated from upstream Linux version at particular minor
version tag. E.g., `vmlinux_505.h` is generated from v5.5 tag. Exact set of
types available in compiled kernel depends on configuration used to compile
it. To generate present vmlinux.h header, default configuration was used, with
only extra `CONFIG_DEBUG_INFO_BTF=y` option enabled.

Given different kernel version can have incompatible type definitions, it
might be important to use vmlinux.h of a specific kernel version as a "base"
version of header. To that extent, all vmlinux.h headers are versioned by
appending <MAJOR><MINOR> suffix to a file name. There is always a symbolic
link vmlinux.h, that points to whichever version is deemed to be default
(usually, latest).
