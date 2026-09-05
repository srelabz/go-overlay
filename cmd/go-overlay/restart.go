package main

import (
	"fmt"
	"sync"
	"time"
)

var (
	restartMu     sync.Mutex
	restartCounts = make(map[string]int)
	serviceDeps   *dependencyTracker
)

func restartCount(name string) int {
	restartMu.Lock()
	defer restartMu.Unlock()
	return restartCounts[name]
}

func incrementRestartCount(name string) int {
	restartMu.Lock()
	defer restartMu.Unlock()
	restartCounts[name]++
	return restartCounts[name]
}

func resetRestartCount(name string) {
	restartMu.Lock()
	defer restartMu.Unlock()
	delete(restartCounts, name)
}

func handleServiceExit(serviceProc *ServiceProcess, exitErr error) {
	policy := serviceProc.Config.Restart

	shouldRestart := false
	switch policy {
	case RestartAlways:
		shouldRestart = true
	case RestartOnFailure:
		shouldRestart = exitErr != nil
	case RestartNever, "":
		shouldRestart = false
	}

	maxRestarts := serviceProc.Config.MaxRestarts
	if shouldRestart && maxRestarts > 0 && restartCount(serviceProc.Name) >= maxRestarts {
		_warn(fmt.Sprintf("Service '%s' reached max restarts (%d), not restarting",
			serviceProc.Name, maxRestarts))

		if serviceProc.Config.Required {
			_error(fmt.Sprintf("[CRITICAL] Required service '%s' exhausted restart attempts, initiating shutdown",
				serviceProc.Name))
			triggerServiceFailureShutdown()
		}
		return
	}

	if shouldRestart {
		attempt := incrementRestartCount(serviceProc.Name)
		delay := serviceProc.Config.RestartDelay
		if delay <= 0 {
			delay = 1
		}

		_info(fmt.Sprintf("Restarting service '%s' in %ds (attempt %d/%s)",
			serviceProc.Name, delay, attempt,
			formatMaxRestarts(maxRestarts)))

		scheduleRestart(serviceProc, delay)
		return
	}

	if serviceProc.Config.Required && exitErr != nil {
		_error(fmt.Sprintf("[CRITICAL] Required service '%s' exited with error, initiating shutdown",
			serviceProc.Name))
		triggerServiceFailureShutdown()
	}
}

func formatMaxRestarts(maxRestarts int) string {
	if maxRestarts == 0 {
		return "∞"
	}
	return fmt.Sprintf("%d", maxRestarts)
}

func scheduleRestart(serviceProc *ServiceProcess, delaySeconds int) {
	if delaySeconds < 0 {
		delaySeconds = 0
	}

	ctx := shutdownContext()
	time.AfterFunc(time.Duration(delaySeconds)*time.Second, func() {
		if ctx.Err() != nil {
			_info(fmt.Sprintf("Skipping restart of '%s' - shutdown in progress", serviceProc.Name))
			return
		}
		restartServiceInternal(serviceProc)
	})
}

func restartServiceInternal(serviceProc *ServiceProcess) {
	config := currentConfig()
	if config == nil {
		_error(fmt.Sprintf("Cannot restart service '%s': no global config", serviceProc.Name))
		return
	}

	if shutdownContext().Err() != nil {
		_info(fmt.Sprintf("Skipping restart of '%s' - shutdown in progress", serviceProc.Name))
		return
	}

	serviceProc.LastRestart = time.Now()
	maxLength := getLongestServiceNameLength(config.Services)

	_info(fmt.Sprintf("Starting restart of service '%s'", serviceProc.Name))

	go func() {
		if err := startService(&serviceProc.Config, maxLength, config.Timeouts); err != nil {
			_error(fmt.Sprintf("Error restarting service '%s': %v", serviceProc.Name, err))
		}
	}()
}
