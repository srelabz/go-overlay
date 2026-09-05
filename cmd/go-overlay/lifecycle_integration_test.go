//go:build integration

package main

import (
	"syscall"
	"testing"
	"time"
)

func waitForServicePID(t *testing.T, name string, timeout time.Duration) int {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		servicesMutex.RLock()
		serviceProc, ok := activeServices[name]
		servicesMutex.RUnlock()
		if ok {
			if pid := serviceProc.GetPID(); pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("service %q never became active", name)
	return 0
}

func TestIntegrationMaxRestartsIsEnforced(t *testing.T) {
	resetSupervisorState(t)

	service := Service{
		Name:         "crash-loop",
		Command:      "/bin/sh",
		Args:         []string{"-c", "exit 1"},
		Restart:      RestartOnFailure,
		MaxRestarts:  2,
		RestartDelay: 1,
	}
	config := currentConfig()
	config.Services = []Service{service}
	setGlobalConfig(config)

	if err := startService(&service, 12, currentConfig().Timeouts); err == nil {
		t.Fatal("startService() should return the failure of a crashing service")
	}

	time.Sleep(7 * time.Second)

	if got := restartCount("crash-loop"); got != 2 {
		t.Errorf("restartCount() = %d, want exactly 2 (max_restarts)", got)
	}
	if got := activeServiceCount(); got != 0 {
		t.Errorf("activeServiceCount() = %d, want 0 after the restart cap", got)
	}
}

func TestIntegrationRestartPolicyNeverDoesNotRestart(t *testing.T) {
	resetSupervisorState(t)

	service := Service{
		Name:    "one-shot-crash",
		Command: "/bin/sh",
		Args:    []string{"-c", "exit 3"},
		Restart: RestartNever,
	}
	config := currentConfig()
	config.Services = []Service{service}
	setGlobalConfig(config)

	if err := startService(&service, 12, currentConfig().Timeouts); err == nil {
		t.Fatal("startService() should return the failure")
	}

	time.Sleep(2 * time.Second)

	if got := restartCount("one-shot-crash"); got != 0 {
		t.Errorf("restartCount() = %d, want 0", got)
	}
}

func TestIntegrationStopKillsProcessGroup(t *testing.T) {
	resetSupervisorState(t)

	service := Service{
		Name:    "group",
		Command: "/bin/sh",
		Args:    []string{"-c", "sleep 60 & wait"},
	}
	config := currentConfig()
	config.Services = []Service{service}
	setGlobalConfig(config)

	done := make(chan error, 1)
	go func() {
		done <- startService(&service, 12, currentConfig().Timeouts)
	}()

	pid := waitForServicePID(t, "group", 5*time.Second)
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatalf("could not read process group: %v", err)
	}

	cancelShutdown()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("service did not stop after shutdown")
	}

	time.Sleep(500 * time.Millisecond)

	if err := syscall.Kill(-pgid, 0); err == nil {
		t.Error("process group is still alive: grandchildren survived the stop")
	}
}

func TestIntegrationUnhealthyServiceRestarts(t *testing.T) {
	resetSupervisorState(t)

	service := Service{
		Name:         "unhealthy",
		Command:      "/bin/sh",
		Args:         []string{"-c", "sleep 30"},
		Restart:      RestartOnFailure,
		MaxRestarts:  1,
		RestartDelay: 1,
		HealthCheck: &HealthCheckConfig{
			Command:    "exit 1",
			Interval:   1,
			Retries:    1,
			Timeout:    1,
			StartDelay: 1,
		},
	}
	config := currentConfig()
	config.Services = []Service{service}
	setGlobalConfig(config)

	done := make(chan error, 1)
	go func() {
		done <- startService(&service, 12, currentConfig().Timeouts)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("unhealthy service was never stopped by the health monitor")
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if restartCount("unhealthy") >= 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Errorf("restartCount() = %d, want at least 1 after health failures", restartCount("unhealthy"))
}
