// Copyright 2018 Google LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syscalls

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/hugelgupf/go-strace/strace"
	"github.com/iimos/play/stracy/ubinary"
	"golang.org/x/sys/unix"
)

// NetAddress is a byte slice cast as a string that represents the address of a
// network node. Or, in the case of unix endpoints, it may represent a path.
type NetAddress string

type FullNetAddress struct {
	// Addr is the network address.
	Addr NetAddress

	// Port is the transport port.
	//
	// This may not be used by all endpoint types.
	Port uint16
}

// GetAddress reads an sockaddr struct from the given address and converts it
// to the FullNetAddress format. It supports AF_UNIX, AF_INET and AF_INET6
// addresses.
func GetAddress(t strace.Task, addr []byte) (FullNetAddress, error) {
	r := bytes.NewBuffer(addr[:2])
	var fam uint16
	if err := binary.Read(r, ubinary.NativeEndian, &fam); err != nil {
		return FullNetAddress{}, unix.EFAULT
	}

	// Get the rest of the fields based on the address family.
	switch fam {
	case unix.AF_UNIX:
		path := addr[2:]
		if len(path) > unix.PathMax {
			return FullNetAddress{}, unix.EINVAL
		}
		// Drop the terminating NUL (if one exists) and everything after
		// it for filesystem (non-abstract) addresses.
		if len(path) > 0 && path[0] != 0 {
			if n := bytes.IndexByte(path[1:], 0); n >= 0 {
				path = path[:n+1]
			}
		}
		return FullNetAddress{
			Addr: NetAddress(path),
		}, nil

	case unix.AF_INET:
		var a unix.RawSockaddrInet4
		r = bytes.NewBuffer(addr)
		if err := binary.Read(r, binary.BigEndian, &a); err != nil {
			return FullNetAddress{}, unix.EFAULT
		}
		out := FullNetAddress{
			Addr: NetAddress(a.Addr[:]),
			Port: uint16(a.Port),
		}
		if out.Addr == "\x00\x00\x00\x00" {
			out.Addr = ""
		}
		return out, nil

	case unix.AF_INET6:
		var a unix.RawSockaddrInet6
		r = bytes.NewBuffer(addr)
		if err := binary.Read(r, binary.BigEndian, &a); err != nil {
			return FullNetAddress{}, unix.EFAULT
		}

		out := FullNetAddress{
			Addr: NetAddress(a.Addr[:]),
			Port: uint16(a.Port),
		}

		//if isLinkLocal(out.Addr) {
		//			out.NIC = NICID(a.Scope_id)
		//}

		if out.Addr == NetAddress(strings.Repeat("\x00", 16)) {
			out.Addr = ""
		}
		return out, nil

	default:
		return FullNetAddress{}, unix.ENOTSUP
	}
}
