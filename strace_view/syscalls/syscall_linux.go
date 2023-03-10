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
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hugelgupf/go-strace/strace"
	"github.com/iimos/play/strace_view/abi"
	"golang.org/x/sys/unix"
)

func path(t strace.Task, addr strace.Addr) string {
	path, err := strace.ReadString(t, addr, unix.PathMax)
	if err != nil {
		return fmt.Sprintf("%#x (error decoding path: %s)", addr, err)
	}
	return path
}

func utimensTimespec(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	var tim unix.Timespec
	if _, err := t.Read(addr, &tim); err != nil {
		return fmt.Sprintf("%#x (error decoding timespec: %s)", addr, err)
	}

	var ns string
	switch tim.Nsec {
	case unix.UTIME_NOW:
		ns = "UTIME_NOW"
	case unix.UTIME_OMIT:
		ns = "UTIME_OMIT"
	default:
		ns = fmt.Sprintf("%v", tim.Nsec)
	}
	return fmt.Sprintf("%#x {sec=%v nsec=%s}", addr, tim.Sec, ns)
}

func timespec(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	var tim unix.Timespec
	if _, err := t.Read(addr, &tim); err != nil {
		return fmt.Sprintf("%#x (error decoding timespec: %s)", addr, err)
	}
	dur := time.Duration(tim.Sec)*time.Second + time.Duration(tim.Nsec)
	return dur.String()
	// return fmt.Sprintf("%#x {sec=%v nsec=%v}", addr, tim.Sec, tim.Nsec)
}

func timeval(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	var tim unix.Timeval
	if _, err := t.Read(addr, &tim); err != nil {
		return fmt.Sprintf("%#x (error decoding timeval: %s)", addr, err)
	}

	return fmt.Sprintf("%#x {sec=%v usec=%v}", addr, tim.Sec, tim.Usec)
}

func utimbuf(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	var utim syscall.Utimbuf
	if _, err := t.Read(addr, &utim); err != nil {
		return fmt.Sprintf("%#x (error decoding utimbuf: %s)", addr, err)
	}

	return fmt.Sprintf("%#x {actime=%v, modtime=%v}", addr, utim.Actime, utim.Modtime)
}

func fileMode(mode uint32) string {
	return fmt.Sprintf("%#09o", mode&0x1ff)
}

func stat(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	var stat unix.Stat_t
	if _, err := t.Read(addr, &stat); err != nil {
		return fmt.Sprintf("%#x (error decoding stat: %s)", addr, err)
	}
	return fmt.Sprintf("%#x {dev=%d, ino=%d, mode=%s, nlink=%d, uid=%d, gid=%d, rdev=%d, size=%d, blksize=%d, blocks=%d, atime=%s, mtime=%s, ctime=%s}", addr, stat.Dev, stat.Ino, fileMode(stat.Mode), stat.Nlink, stat.Uid, stat.Gid, stat.Rdev, stat.Size, stat.Blksize, stat.Blocks, time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)), time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec)), time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)))
}

func itimerval(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	interval := timeval(t, addr)
	value := timeval(t, addr+strace.Addr(binary.Size(unix.Timeval{})))
	return fmt.Sprintf("%#x {interval=%s, value=%s}", addr, interval, value)
}

func itimerspec(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	interval := timespec(t, addr)
	value := timespec(t, addr+strace.Addr(binary.Size(unix.Timespec{})))
	return fmt.Sprintf("%#x {interval=%s, value=%s}", addr, interval, value)
}

func stringVector(t strace.Task, addr strace.Addr) string {
	vs, err := strace.ReadStringVector(t, addr, strace.ExecMaxElemSize, strace.ExecMaxTotalSize)
	if err != nil {
		return fmt.Sprintf("%#x {error copying vector: %v}", addr, err)
	}
	return fmt.Sprintf("%q", vs)
}

func rusage(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	var ru unix.Rusage
	if _, err := t.Read(addr, &ru); err != nil {
		return fmt.Sprintf("%#x (error decoding rusage: %s)", addr, err)
	}
	return fmt.Sprintf("%#x %+v", addr, ru)
}

func cpuSet(t strace.Task, addr strace.Addr) string {
	if addr == 0 {
		return "null"
	}

	var set unix.CPUSet
	if _, err := t.Read(addr, &set); err != nil {
		return fmt.Sprintf("%#x (error decoding CPUSet: %s)", addr, err)
	}

	cpus := make([]int, 0, 4)
	maxCPUID := 64 * len(set)
	for i := 0; i < maxCPUID; i++ {
		if set.IsSet(i) {
			cpus = append(cpus, i)
		}
	}
	return fmt.Sprintf("%v", cpus)
}

type flagSpec struct {
	flag int
	str  string
}

var mapProtFlags = [...]flagSpec{
	{syscall.PROT_EXEC, "EXEC"},
	{syscall.PROT_GROWSDOWN, "GROWSDOWN"},
	{syscall.PROT_GROWSUP, "GROWSUP"},
	{syscall.PROT_NONE, "NONE"},
	{syscall.PROT_READ, "READ"},
	{syscall.PROT_WRITE, "WRITE"},
}

var mapFlags = [...]flagSpec{
	{syscall.MAP_32BIT, "32BIT"},
	{syscall.MAP_ANON, "ANON"},
	{syscall.MAP_ANONYMOUS, "ANONYMOUS"},
	{syscall.MAP_DENYWRITE, "DENYWRITE"},
	{syscall.MAP_EXECUTABLE, "EXECUTABLE"},
	{syscall.MAP_FILE, "FILE"},
	{syscall.MAP_FIXED, "FIXED"},
	{syscall.MAP_GROWSDOWN, "GROWSDOWN"},
	{syscall.MAP_HUGETLB, "HUGETLB"},
	{syscall.MAP_LOCKED, "LOCKED"},
	{syscall.MAP_NONBLOCK, "NONBLOCK"},
	{syscall.MAP_NORESERVE, "NORESERVE"},
	{syscall.MAP_POPULATE, "POPULATE"},
	{syscall.MAP_PRIVATE, "PRIVATE"},
	{syscall.MAP_SHARED, "SHARED"},
	{syscall.MAP_STACK, "STACK"},
	{syscall.MAP_TYPE, "TYPE"},
}

var madvFlags = [...]flagSpec{
	{syscall.MADV_DOFORK, "DOFORK"},
	{syscall.MADV_DONTFORK, "DONTFORK"},
	{syscall.MADV_DONTNEED, "DONTNEED"},
	{syscall.MADV_HUGEPAGE, "HUGEPAGE"},
	{syscall.MADV_HWPOISON, "HWPOISON"},
	{syscall.MADV_MERGEABLE, "MERGEABLE"},
	{syscall.MADV_NOHUGEPAGE, "NOHUGEPAGE"},
	{syscall.MADV_NORMAL, "NORMAL"},
	{syscall.MADV_RANDOM, "RANDOM"},
	{syscall.MADV_REMOVE, "REMOVE"},
	{syscall.MADV_SEQUENTIAL, "SEQUENTIAL"},
	{syscall.MADV_UNMERGEABLE, "UNMERGEABLE"},
	{syscall.MADV_WILLNEED, "WILLNEED"},
}

var archPrctlCodes = map[int]string{
	0x1001: "ARCH_SET_GS",
	0x1002: "ARCH_SET_FS",
	0x1003: "ARCH_GET_FS",
	0x1004: "ARCH_GET_GS",
	0x1011: "ARCH_GET_CPUID",
	0x1012: "ARCH_SET_CPUID",
	0x1021: "ARCH_GET_XCOMP_SUPP",
	0x1022: "ARCH_GET_XCOMP_PERM",
	0x1023: "ARCH_REQ_XCOMP_PERM",
	0x1024: "ARCH_GET_XCOMP_GUEST_PERM",
	0x1025: "ARCH_REQ_XCOMP_GUEST_PERM",
	0x2001: "ARCH_MAP_VDSO_X32",
	0x2002: "ARCH_MAP_VDSO_32",
	0x2003: "ARCH_MAP_VDSO_64",
}

func flagsToStrings(specs []flagSpec, flags int) []string {
	if flags == 0 {
		for _, v := range specs {
			if v.flag == 0 {
				return []string{v.str}
			}
		}
		return []string{"0"}
	}

	count := bits.OnesCount(uint(flags))
	ret := make([]string, 0, count)
	for _, v := range specs {
		if v.flag&flags != 0 {
			ret = append(ret, v.str)
			flags &= ^v.flag // erase flag to avoid dublicates
		}
	}
	return ret
}

func flags(specs []flagSpec, t strace.Task, bits int) string {
	flagsStr := flagsToStrings(specs, bits)
	return strings.Join(flagsStr, "|")
}

// ArgumentsStrings fills arguments for a system call. If an argument
// cannot be interpreted, then a hex value will be used. Note that
// a full output slice will always be provided, that is len(return) == len(args).
func ArgumentsStrings(si SyscallInfo, t strace.Task, args strace.SyscallArguments, rval strace.SyscallArgument, maximumBlobSize uint) []string {
	output := make([]string, len(si.ArgTypes))
	for i, format := range si.ArgTypes {
		if i >= len(args) {
			break
		}
		switch format {
		// Available on syscall enter:
		// case WriteBuffer:
		// 	output[i] = dump(t, args[i].Pointer(), args[i+1].SizeT(), maximumBlobSize)
		// case WriteIOVec:
		// 	output[i] = iovecs(t, args[i].Pointer(), int(args[i+1].Int()), true /* content */, uint64(maximumBlobSize))
		// case IOVec:
		// 	output[i] = iovecs(t, args[i].Pointer(), int(args[i+1].Int()), false /* content */, uint64(maximumBlobSize))
		// case SendMsgHdr:
		// 	output[i] = msghdr(t, args[i].Pointer(), true /* content */, uint64(maximumBlobSize))
		// case RecvMsgHdr:
		// 	output[i] = msghdr(t, args[i].Pointer(), false /* content */, uint64(maximumBlobSize))
		case Path:
			output[i] = path(t, args[i].Pointer())
		case ExecveStringVector:
			output[i] = stringVector(t, args[i].Pointer())
		case SockAddr:
			output[i] = sockAddr(t, args[i].Pointer(), uint32(args[i+1].Uint64()))
		// case SockLen:
		// 	output[i] = sockLenPointer(t, args[i].Pointer())
		case SockFamily:
			output[i] = abi.SocketFamily.Parse(uint64(args[i].Int()))
		case SockType:
			output[i] = abi.SockType(args[i].Int())
		case SockProtocol:
			output[i] = abi.SockProtocol(args[i-2].Int(), args[i].Int())
		case SockFlags:
			output[i] = abi.SockFlags(args[i].Int())
		case Timespec:
			output[i] = timespec(t, args[i].Pointer())
		case UTimeTimespec:
			output[i] = utimensTimespec(t, args[i].Pointer())
		case ItimerVal:
			output[i] = itimerval(t, args[i].Pointer())
		case ItimerSpec:
			output[i] = itimerspec(t, args[i].Pointer())
		// case Timeval:
		// 	output[i] = timeval(t, args[i].Pointer())
		case Utimbuf:
			output[i] = utimbuf(t, args[i].Pointer())
		case CloneFlags:
			output[i] = abi.CloneFlagSet.Parse(uint64(args[i].Uint()))
		case OpenFlags:
			output[i] = abi.Open(uint64(args[i].Uint()))
		case Mode:
			output[i] = os.FileMode(args[i].Uint()).String()
		case FutexOp:
			output[i] = abi.Futex(uint64(args[i].Uint()))
		case PtraceRequest:
			output[i] = abi.PtraceRequestSet.Parse(args[i].Uint64())
		case ItimerType:
			output[i] = abi.ItimerTypes.Parse(uint64(args[i].Int()))
		case MMapProt:
			output[i] = flags(mapProtFlags[:], t, int(args[i].Int()))
		case MMapFlags:
			output[i] = flags(mapFlags[:], t, int(args[i].Int()))
		case MADVFlags:
			output[i] = flags(madvFlags[:], t, int(args[i].Int()))
		case Signal:
			output[i] = SignalString(unix.Signal(args[i].Int()))
		case ArchPrctl:
			v := archPrctlCodes[int(args[i].Int())]
			if v == "" {
				v = strconv.FormatUint(args[i].Uint64(), 16)
			}
			output[i] = v
		case Oct:
			if args[i].Uint64() == 0 {
				output[i] = "0"
			} else {
				output[i] = "0o" + strconv.FormatUint(args[i].Uint64(), 8)
			}
		case Dec, PID:
			output[i] = strconv.FormatInt(int64(args[i].Int()), 10)
		case Hex:
			if args[i].Uint64() == 0 {
				output[i] = "0"
			} else {
				output[i] = "0x" + strconv.FormatUint(args[i].Uint64(), 16)
			}

		// Available on syscall exit:
		case ReadBuffer:
			output[i] = dump(t, args[i].Pointer(), uint(rval.Uint64()), maximumBlobSize)
		case ReadIOVec:
			printLength := rval.Uint()
			if printLength > uint32(maximumBlobSize) {
				printLength = uint32(maximumBlobSize)
			}
			output[i] = iovecs(t, args[i].Pointer(), int(args[i+1].Int()), true /* content */, uint64(printLength))
		case WriteIOVec, IOVec, WriteBuffer:
			// We already have a big blast from write.
			output[i] = "..."
		case SendMsgHdr:
			output[i] = msghdr(t, args[i].Pointer(), false /* content */, uint64(maximumBlobSize))
		case RecvMsgHdr:
			output[i] = msghdr(t, args[i].Pointer(), true /* content */, uint64(maximumBlobSize))
		case PostPath:
			output[i] = path(t, args[i].Pointer())
		case PipeFDs:
			output[i] = fdpair(t, args[i].Pointer())
		case Uname:
			output[i] = uname(t, args[i].Pointer())
		case Stat:
			output[i] = stat(t, args[i].Pointer())
		case PostSockAddr:
			output[i] = postSockAddr(t, args[i].Pointer(), args[i+1].Pointer())
		case SockLen:
			output[i] = sockLenPointer(t, args[i].Pointer())
		case PostTimespec:
			output[i] = timespec(t, args[i].Pointer())
		case PostItimerVal:
			output[i] = itimerval(t, args[i].Pointer())
		case PostItimerSpec:
			output[i] = itimerspec(t, args[i].Pointer())
		case Timeval:
			output[i] = timeval(t, args[i].Pointer())
		case Rusage:
			output[i] = rusage(t, args[i].Pointer())
		case CPUSet:
			output[i] = cpuSet(t, args[i].Pointer())
		default:
			output[i] = "0x" + strconv.FormatUint(args[i].Uint64(), 16)
		}
	}
	return output
}

func SignalString(s unix.Signal) string {
	if 0 <= s && int(s) < len(signals) {
		return signals[s]
	}
	return fmt.Sprintf("signal %d", int(s))
}
