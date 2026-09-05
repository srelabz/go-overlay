//go:build linux && (amd64 || arm64)

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"
)

const (
	waitIDAll    = 0
	waitIDExited = 0x00000004
	waitIDNoWait = 0x01000000
	waitIDNoHang = 0x00000001

	reaperMaxBatch = 128
)

type siginfoChild struct {
	Signo  int32
	Errno  int32
	Code   int32
	_      int32
	Pid    int32
	UID    uint32
	Status int32
	_      [100]byte
}

func peekExitedChild() (int, bool) {
	var info siginfoChild
	// #nosec G103 -- waitid requires a raw syscall with a siginfo_t buffer
	_, _, errno := syscall.Syscall6(
		syscall.SYS_WAITID,
		uintptr(waitIDAll),
		0,
		uintptr(unsafe.Pointer(&info)),
		uintptr(waitIDExited|waitIDNoWait|waitIDNoHang),
		0,
		0,
	)
	if errno != 0 || info.Pid <= 0 {
		return 0, false
	}
	return int(info.Pid), true
}

func reapOrphans() {
	for i := 0; i < reaperMaxBatch; i++ {
		pid, ok := peekExitedChild()
		if !ok {
			return
		}
		if isTrackedChild(pid) {
			return
		}
		var status syscall.WaitStatus
		reaped, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		if err != nil || reaped <= 0 {
			return
		}
		_debug(fmt.Sprintf("Reaped orphan process %d (exit status %d)", reaped, status.ExitStatus()))
	}
}

func startZombieReaper() {
	if os.Getpid() != 1 {
		return
	}

	_info("Running as PID 1: orphan process reaper enabled")

	sigChild := make(chan os.Signal, 1)
	signal.Notify(sigChild, syscall.SIGCHLD)

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sigChild:
			case <-ticker.C:
			}
			reapOrphans()
		}
	}()
}
