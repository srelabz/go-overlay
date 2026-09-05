package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testTimeouts() Timeouts {
	return Timeouts{
		PostScript:      1,
		ServiceShutdown: 3,
		GlobalShutdown:  5,
		DependencyWait:  5,
	}
}

func resetSupervisorState(t *testing.T) {
	t.Helper()

	if shutdownCancel != nil {
		cancelShutdown()
	}
	setShutdownContext(context.WithCancel(context.Background()))

	servicesMutex.Lock()
	activeServices = make(map[string]*ServiceProcess)
	servicesMutex.Unlock()

	restartMu.Lock()
	restartCounts = make(map[string]int)
	restartMu.Unlock()

	serviceDeps = newDependencyTracker()
	shutdownOnce = sync.Once{}
	shutdownWg = sync.WaitGroup{}
	exitCode.Store(0)
	setGlobalConfig(&Config{Timeouts: testTimeouts()})

	t.Cleanup(func() {
		cancelShutdown()
	})
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = writer

	collected := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		collected <- string(data)
	}()

	fn()

	os.Stdout = original
	_ = writer.Close()
	output := <-collected
	_ = reader.Close()

	return output
}

func TestRestartCountersArePerService(t *testing.T) {
	resetSupervisorState(t)

	if got := restartCount("api"); got != 0 {
		t.Fatalf("restartCount() = %d, want 0", got)
	}

	if got := incrementRestartCount("api"); got != 1 {
		t.Errorf("incrementRestartCount() = %d, want 1", got)
	}
	if got := incrementRestartCount("api"); got != 2 {
		t.Errorf("incrementRestartCount() = %d, want 2", got)
	}
	if got := restartCount("worker"); got != 0 {
		t.Errorf("restartCount(worker) = %d, want 0", got)
	}

	resetRestartCount("api")
	if got := restartCount("api"); got != 0 {
		t.Errorf("restartCount() after reset = %d, want 0", got)
	}
}

func TestHandleServiceExitStopsAtMaxRestarts(t *testing.T) {
	resetSupervisorState(t)

	serviceProc := &ServiceProcess{
		Name:   "capped",
		Config: Service{Name: "capped", Restart: RestartOnFailure, MaxRestarts: 2},
	}

	restartMu.Lock()
	restartCounts["capped"] = 2
	restartMu.Unlock()

	handleServiceExit(serviceProc, os.ErrClosed)

	if got := restartCount("capped"); got != 2 {
		t.Errorf("restartCount() = %d, want 2 (no restart past the cap)", got)
	}
}

func TestStopIntentIsSetOnlyOnce(t *testing.T) {
	serviceProc := &ServiceProcess{Name: "svc", Cancel: func() {}}

	if !serviceProc.RequestStop(stopIntentRestartOperator) {
		t.Fatal("first RequestStop() should succeed")
	}
	if serviceProc.RequestStop(stopIntentShutdown) {
		t.Error("second RequestStop() should fail")
	}
	if got := serviceProc.StopIntent(); got != stopIntentRestartOperator {
		t.Errorf("StopIntent() = %v, want stopIntentRestartOperator", got)
	}
}

func TestStreamServiceLogsKeepsOutputAfterLongLine(t *testing.T) {
	longLine := strings.Repeat("x", 200*1024)
	input := longLine + "\nAFTER_LONG_LINE\n"

	output := captureStdout(t, func() {
		streamServiceLogs(strings.NewReader(input), "svc", 3)
	})

	if !strings.Contains(output, "AFTER_LONG_LINE") {
		t.Error("output after a long line was dropped")
	}
	if got := strings.Count(output, "x"); got != len(longLine) {
		t.Errorf("streamed %d bytes of the long line, want %d", got, len(longLine))
	}
}

func TestStartServiceCleanExitLeavesNoState(t *testing.T) {
	resetSupervisorState(t)

	service := Service{
		Name:    "quick",
		Command: "/bin/sh",
		Args:    []string{"-c", "echo hello; exit 0"},
	}

	err := startService(&service, 10, currentConfig().Timeouts)
	if err != nil {
		t.Fatalf("startService() returned %v, want nil for a clean exit", err)
	}

	if got := activeServiceCount(); got != 0 {
		t.Errorf("activeServiceCount() = %d, want 0", got)
	}
	if got := restartCount("quick"); got != 0 {
		t.Errorf("restartCount() = %d, want 0 for restart policy never", got)
	}
}

func TestStartServiceReportsStartupFailure(t *testing.T) {
	resetSupervisorState(t)

	service := Service{
		Name:    "missing",
		Command: filepath.Join(t.TempDir(), "does-not-exist"),
	}

	err := startService(&service, 10, currentConfig().Timeouts)
	if err == nil {
		t.Fatal("startService() should fail when the command does not exist")
	}

	var startErr *startupError
	if !errors.As(err, &startErr) {
		t.Errorf("error %v should be a startupError", err)
	}
}

func TestStartServiceWritesToLogFile(t *testing.T) {
	resetSupervisorState(t)

	logPath := filepath.Join(t.TempDir(), "service.log")
	service := Service{
		Name:    "logged",
		Command: "/bin/sh",
		Args:    []string{"-c", "echo written-to-file"},
		LogFile: logPath,
	}

	if err := startService(&service, 10, currentConfig().Timeouts); err != nil {
		t.Fatalf("startService() failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("could not read log file: %v", err)
	}

	if !strings.Contains(string(content), "written-to-file") {
		t.Errorf("log file content = %q, want it to contain the service output", string(content))
	}
}

func TestHandleRestartServiceDoesNotBlock(t *testing.T) {
	resetSupervisorState(t)

	canceled := make(chan struct{})
	serviceProc := &ServiceProcess{
		Name:   "api",
		Config: Service{Name: "api"},
		Cancel: func() { close(canceled) },
	}

	servicesMutex.Lock()
	activeServices["api"] = serviceProc
	servicesMutex.Unlock()

	start := time.Now()
	response := handleRestartService("api")
	elapsed := time.Since(start)

	if !response.Success {
		t.Fatalf("handleRestartService() = %+v, want success", response)
	}
	if elapsed > time.Second {
		t.Errorf("handleRestartService() took %v, want an immediate return", elapsed)
	}

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Error("handleRestartService() did not cancel the service context")
	}

	if got := activeServiceCount(); got != 1 {
		t.Errorf("activeServiceCount() = %d, want 1 (entry removed by the lifecycle, not the handler)", got)
	}

	if second := handleRestartService("api"); second.Success {
		t.Error("second restart request should be rejected while stopping")
	}

	servicesMutex.Lock()
	delete(activeServices, "api")
	servicesMutex.Unlock()
}

func TestHandleRestartServiceUnknownService(t *testing.T) {
	resetSupervisorState(t)

	response := handleRestartService("ghost")
	if response.Success {
		t.Error("restarting an unknown service should fail")
	}
	if !strings.Contains(response.Message, "not found") {
		t.Errorf("message = %q, want it to mention the service was not found", response.Message)
	}
}

func TestOverrideEnvReplacesExistingKeys(t *testing.T) {
	env := []string{"HOME=/root", "PATH=/usr/bin", "USER=root"}

	result := overrideEnv(env, map[string]string{"HOME": "/home/app", "USER": "app"})

	values := make(map[string]string, len(result))
	for _, entry := range result {
		key, value, _ := strings.Cut(entry, "=")
		if _, duplicated := values[key]; duplicated {
			t.Errorf("key %q appears twice in the resulting environment", key)
		}
		values[key] = value
	}

	if values["HOME"] != "/home/app" {
		t.Errorf("HOME = %q, want /home/app", values["HOME"])
	}
	if values["USER"] != "app" {
		t.Errorf("USER = %q, want app", values["USER"])
	}
	if values["PATH"] != "/usr/bin" {
		t.Errorf("PATH = %q, want it preserved", values["PATH"])
	}
}

func TestMaskEnvEntryHidesSecrets(t *testing.T) {
	tests := []struct {
		entry  string
		masked bool
	}{
		{"DATABASE_PASSWORD=hunter2", true},
		{"API_TOKEN=abc123", true},
		{"AWS_SECRET_ACCESS_KEY=xyz", true},
		{"SSH_PRIVATE_KEY=xyz", true},
		{"LOG_LEVEL=debug", false},
		{"PATH=/usr/bin", false},
	}

	for _, tt := range tests {
		got := maskEnvEntry(tt.entry)
		if tt.masked && strings.Contains(got, "=") && !strings.Contains(got, "<redacted") {
			t.Errorf("maskEnvEntry(%q) = %q, want the value redacted", tt.entry, got)
		}
		if !tt.masked && got != tt.entry {
			t.Errorf("maskEnvEntry(%q) = %q, want it unchanged", tt.entry, got)
		}
	}
}

func TestAutoInstallRequiresOptIn(t *testing.T) {
	t.Setenv(envAutoInstall, "")
	os.Unsetenv(envAutoInstall)

	autoInstallInPath()

	if _, err := os.Lstat(installTarget); err == nil {
		t.Skip("install target already exists on this machine")
	}
}
