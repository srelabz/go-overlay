package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

const healthBodyDrainLimit = 4096

var healthHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	},
}

func applyHealthCheckDefaults(hc *HealthCheckConfig) {
	if hc.Interval == 0 {
		hc.Interval = 30
	}
	if hc.Retries == 0 {
		hc.Retries = 3
	}
	if hc.Timeout == 0 {
		hc.Timeout = 5
	}
	if hc.StartDelay == 0 {
		hc.StartDelay = 10
	}
}

func startHealthMonitor(serviceProc *ServiceProcess) {
	if serviceProc.Config.HealthCheck == nil {
		return
	}

	config := *serviceProc.Config.HealthCheck
	applyHealthCheckDefaults(&config)

	healthCtx, healthCancel := context.WithCancel(shutdownContext())
	serviceProc.HealthCancel = healthCancel

	go func() {
		defer healthCancel()

		_info(fmt.Sprintf("Health monitor for '%s' will start in %ds",
			serviceProc.Name, config.StartDelay))

		select {
		case <-time.After(time.Duration(config.StartDelay) * time.Second):
		case <-healthCtx.Done():
			return
		}

		_info(fmt.Sprintf("Health monitor started for '%s' (interval: %ds, retries: %d)",
			serviceProc.Name, config.Interval, config.Retries))

		ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
		defer ticker.Stop()

		failureCount := 0

		for {
			select {
			case <-healthCtx.Done():
				return
			case <-ticker.C:
				if performHealthCheck(healthCtx, config) {
					if failureCount > 0 {
						_success(fmt.Sprintf("Service '%s' is healthy again after %d failures",
							serviceProc.Name, failureCount))
					}
					failureCount = 0
					serviceProc.StateMu.Lock()
					serviceProc.HealthyAt = time.Now()
					serviceProc.FailureCount = 0
					serviceProc.StateMu.Unlock()
					continue
				}

				failureCount++
				serviceProc.StateMu.Lock()
				serviceProc.FailureCount = failureCount
				serviceProc.StateMu.Unlock()

				_warn(fmt.Sprintf("Health check failed for '%s' (%d/%d)",
					serviceProc.Name, failureCount, config.Retries))

				if failureCount >= config.Retries {
					handleUnhealthyService(serviceProc)
					return
				}
			}
		}
	}()
}

func performHealthCheck(ctx context.Context, config HealthCheckConfig) bool {
	if config.Endpoint != "" {
		return checkHTTPHealth(ctx, config.Endpoint, config.Timeout)
	}
	if config.Command != "" {
		return checkCommandHealth(config.Command, config.Timeout)
	}
	return true
}

func checkHTTPHealth(ctx context.Context, endpoint string, timeout int) bool {
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return false
	}

	resp, err := healthHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if _, drainErr := io.Copy(io.Discard, io.LimitReader(resp.Body, healthBodyDrainLimit)); drainErr != nil {
		_debug(fmt.Sprintf("Could not drain health check body from %s: %v", endpoint, drainErr))
	}

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func checkCommandHealth(command string, timeout int) bool {
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(timeout)*time.Second)
	defer cancel()

	shell := "sh"
	if isBashAvailable() {
		shell = "bash"
	}

	cmd := exec.CommandContext(ctx, shell, "-c", command) // #nosec G204
	return runTracked(cmd) == nil
}

func handleUnhealthyService(serviceProc *ServiceProcess) {
	_error(fmt.Sprintf("Service '%s' is unhealthy after %d consecutive failures",
		serviceProc.Name, serviceProc.FailureCount))

	policy := serviceProc.Config.Restart
	if policy == RestartAlways || policy == RestartOnFailure {
		_info(fmt.Sprintf("Triggering restart for unhealthy service '%s'", serviceProc.Name))
		serviceProc.RequestStop(stopIntentRestartPolicy)
		return
	}

	if serviceProc.Config.Required {
		_error(fmt.Sprintf("[CRITICAL] Required service '%s' is unhealthy, initiating shutdown",
			serviceProc.Name))
		triggerServiceFailureShutdown()
	}
}
