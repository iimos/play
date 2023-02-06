package main

import (
	"fmt"
	seccomp "github.com/seccomp/libseccomp-golang"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	var regs syscall.PtraceRegs

	fmt.Printf("run %q\n", strings.Join(os.Args[1:], " "))

	// Uncommenting this will cause the open syscall to return with Operation Not Permitted error
	// disallow("open")

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ptrace: true,
	}

	err := cmd.Start()
	if err != nil {
		panic(err)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Printf("Wait returned: %v\n", err)
	}

	pid := cmd.Process.Pid

	var (
		insideSyscall bool
		start         = time.Now()
	)
	for {
		err = syscall.PtraceGetRegs(pid, &regs)
		if err == syscall.ESRCH {
			break // no such process
		}
		if err != nil {
			panic(err)
		}

		//if insideSyscall {
		//	name, err := seccomp.ScmpSyscall(regs.Orig_rax).GetName()
		//	if err != nil {
		//		panic(err)
		//	}
		//	//var arg0, arg1, arg2 interface{}
		//	switch name {
		//	case "write":
		//		arg0 := regs.Rdi
		//		size := int(regs.Rdx)
		//		fmt.Printf(">> (%v, %v)\n", regs.Rsi, size)
		//		arg1 := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(regs.Rsi))), size-1)
		//		arg2 := size
		//		fmt.Printf(">> %s(%v, %v, %v)\n", name, arg0, arg1, arg2)
		//	default:
		//	}
		//}

		if !insideSyscall {
			name, err := seccomp.ScmpSyscall(regs.Orig_rax).GetName()
			if err != nil {
				panic(err)
			}
			ts := float64(start.UnixNano()) / float64(time.Second)
			dur := float64(time.Since(start)) / float64(time.Second)

			var arg0, arg1, arg2 interface{}
			switch name {
			case "write":
				arg0 = regs.Rdi
				arg1, err = readString(pid, regs.Rsi, regs.Rdx)
				if err != nil {
					panic(err)
				}
				arg2 = int(regs.Rdx)
			default:
				arg0 = regs.Rdi
				arg1 = regs.Rsi
				arg2 = regs.Rdx
			}

			fmt.Printf("%f %s(%#v, %#v, %#v) \t <%f>\n", ts, name, arg0, arg1, arg2, dur)
		}

		err = syscall.PtraceSyscall(pid, 0)
		if err != nil {
			panic(err)
		}

		_, err = syscall.Wait4(pid, nil, 0, nil)
		if err != nil {
			panic(err)
		}

		if !insideSyscall {
			start = time.Now()
		}
		insideSyscall = !insideSyscall
	}
}

func readString(pid int, ptr uint64, size uint64) (string, error) {
	if size == 0 {
		return "", nil
	}
	data := make([]byte, size)
	_, err := syscall.PtracePeekData(pid, uintptr(ptr), data)
	if err != nil {
		return "", err
	}
	data = data[:size-1] // strip \x00
	return string(data), nil
}
