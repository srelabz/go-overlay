package main

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"syscall"
)

var (
	childMu   sync.Mutex
	childPIDs = make(map[int]struct{})
	selfPgid  = resolveSelfPgid()
)

func resolveSelfPgid() int {
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		return 0
	}
	return pgid
}

func trackChild(pid int) {
	if pid <= 0 {
		return
	}
	childMu.Lock()
	childPIDs[pid] = struct{}{}
	childMu.Unlock()
}

func untrackChild(pid int) {
	if pid <= 0 {
		return
	}
	childMu.Lock()
	delete(childPIDs, pid)
	childMu.Unlock()
}

func isTrackedChild(pid int) bool {
	childMu.Lock()
	defer childMu.Unlock()
	_, ok := childPIDs[pid]
	return ok
}

func runTracked(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	trackChild(pid)
	defer untrackChild(pid)
	return cmd.Wait()
}

func signalProcess(proc *os.Process, sig syscall.Signal) error {
	if proc == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(proc.Pid)
	if err == nil && pgid > 0 && pgid != selfPgid {
		if killErr := syscall.Kill(-pgid, sig); killErr == nil {
			return nil
		}
	}
	return proc.Signal(sig)
}

func killProcess(proc *os.Process) error {
	if proc == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(proc.Pid)
	if err == nil && pgid > 0 && pgid != selfPgid {
		if killErr := syscall.Kill(-pgid, syscall.SIGKILL); killErr == nil {
			return nil
		}
	}
	return proc.Kill()
}

func lookupUserCredential(name string) (*syscall.Credential, string, error) {
	usr, err := user.Lookup(name)
	if err != nil {
		return nil, "", err
	}

	uid, err := strconv.ParseUint(usr.Uid, 10, 32)
	if err != nil {
		return nil, "", err
	}

	gid, err := strconv.ParseUint(usr.Gid, 10, 32)
	if err != nil {
		return nil, "", err
	}

	cred := &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}

	if groupIDs, groupErr := usr.GroupIds(); groupErr == nil {
		groups := make([]uint32, 0, len(groupIDs))
		for _, raw := range groupIDs {
			parsed, convErr := strconv.ParseUint(raw, 10, 32)
			if convErr != nil {
				continue
			}
			groups = append(groups, uint32(parsed))
		}
		cred.Groups = groups
	}

	return cred, usr.HomeDir, nil
}
