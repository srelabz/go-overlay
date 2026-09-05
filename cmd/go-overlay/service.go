package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const logStreamBufferSize = 64 * 1024

var errUnhealthyService = errors.New("service reported unhealthy")

type ServiceState int

const (
	ServiceStatePending ServiceState = iota
	ServiceStateStarting
	ServiceStateRunning
	ServiceStateStopping
	ServiceStateStopped
	ServiceStateFailed
)

func (s ServiceState) String() string {
	switch s {
	case ServiceStatePending:
		return "PENDING"
	case ServiceStateStarting:
		return "STARTING"
	case ServiceStateRunning:
		return "RUNNING"
	case ServiceStateStopping:
		return "STOPPING"
	case ServiceStateStopped:
		return "STOPPED"
	case ServiceStateFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

type stopIntent int32

const (
	stopIntentNone stopIntent = iota
	stopIntentShutdown
	stopIntentRestartPolicy
	stopIntentRestartOperator
)

type startupError struct {
	err error
}

func (e *startupError) Error() string { return e.err.Error() }
func (e *startupError) Unwrap() error { return e.err }

type ServiceProcess struct {
	LastError    error
	Process      *exec.Cmd
	PTY          *os.File
	LogFile      *os.File
	Cancel       context.CancelFunc
	HealthCancel context.CancelFunc
	Name         string
	StartTime    time.Time
	HealthyAt    time.Time
	LastRestart  time.Time
	Config       Service
	StateMu      sync.RWMutex
	State        ServiceState
	FailureCount int
	intent       atomic.Int32
}

func (sp *ServiceProcess) SetState(state ServiceState) {
	sp.StateMu.Lock()
	oldState := sp.State
	sp.State = state
	sp.StateMu.Unlock()

	if oldState == state {
		return
	}

	_info(fmt.Sprintf("Service '%s' state changed from %s to %s",
		colorize(ColorCyan, sp.Name),
		colorize(getStateColor(oldState), oldState.String()),
		colorize(getStateColor(state), state.String())))
}

func (sp *ServiceProcess) GetState() ServiceState {
	sp.StateMu.RLock()
	defer sp.StateMu.RUnlock()
	return sp.State
}

func (sp *ServiceProcess) SetError(err error) {
	sp.StateMu.Lock()
	sp.LastError = err
	if err != nil {
		sp.State = ServiceStateFailed
	}
	sp.StateMu.Unlock()

	if err != nil {
		_error(fmt.Sprintf("Service '%s' failed with error: %v", colorize(ColorCyan, sp.Name), err))
	}
}

func (sp *ServiceProcess) GetError() error {
	sp.StateMu.RLock()
	defer sp.StateMu.RUnlock()
	return sp.LastError
}

func (sp *ServiceProcess) GetPID() int {
	if sp.Process != nil && sp.Process.Process != nil {
		return sp.Process.Process.Pid
	}
	return 0
}

func (sp *ServiceProcess) RequestStop(intent stopIntent) bool {
	if !sp.intent.CompareAndSwap(int32(stopIntentNone), int32(intent)) {
		return false
	}
	if sp.Cancel != nil {
		sp.Cancel()
	}
	return true
}

func (sp *ServiceProcess) StopIntent() stopIntent {
	return stopIntent(sp.intent.Load())
}

func addActiveService(name string, serviceProc *ServiceProcess) {
	servicesMutex.Lock()
	defer servicesMutex.Unlock()
	serviceProc.SetState(ServiceStateStarting)
	serviceProc.StartTime = time.Now()
	activeServices[name] = serviceProc
	shutdownWg.Add(1)
}

func removeActiveService(name string) {
	servicesMutex.Lock()
	defer servicesMutex.Unlock()
	if serviceProc, exists := activeServices[name]; exists {
		serviceProc.SetState(ServiceStateStopped)
		delete(activeServices, name)
		shutdownWg.Done()
	}
}

func activeServiceCount() int {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()
	return len(activeServices)
}

func loadServices(configFile string) error {
	config, err := loadAndValidateConfig(configFile)
	if err != nil {
		return err
	}
	setGlobalConfig(&config)
	return startAllServices(config)
}

func loadAndValidateConfig(configFile string) (Config, error) {
	_info(fmt.Sprintf("Loading services from %s", colorize(ColorCyan, configFile)))

	file, err := os.Open(configFile) // #nosec G304
	if err != nil {
		return Config{}, fmt.Errorf("error opening config file %s: %w", configFile, err)
	}
	defer file.Close()

	config, err := parseConfig(file)
	if err != nil {
		return Config{}, fmt.Errorf("error parsing config file %s: %w", configFile, err)
	}

	if err := validateConfig(&config); err != nil {
		return Config{}, fmt.Errorf("configuration validation failed: %w", err)
	}

	_success("Configuration validated successfully")
	_info(fmt.Sprintf("Timeouts configured: PostScript=%ds, ServiceShutdown=%ds, GlobalShutdown=%ds, DependencyWait=%ds",
		config.Timeouts.PostScript,
		config.Timeouts.ServiceShutdown,
		config.Timeouts.GlobalShutdown,
		config.Timeouts.DependencyWait))

	return config, nil
}

func startAllServices(config Config) error {
	deps := newDependencyTracker()
	serviceDeps = deps
	maxLength := getLongestServiceNameLength(config.Services)

	var wg sync.WaitGroup
	for i := range config.Services {
		service := &config.Services[i]
		if !isServiceEnabled(service) {
			_info("Service ", service.Name, " is disabled, skipping")
			continue
		}
		wg.Add(1)
		go func(s *Service, timeouts Timeouts) {
			defer wg.Done()
			processService(s, deps, maxLength, timeouts)
		}(service, config.Timeouts)
	}

	wg.Wait()
	printServiceStatuses()
	<-shutdownContext().Done()
	_info("Shutdown signal received, stopping all services...")
	return nil
}

func processService(s *Service, deps *dependencyTracker, maxLength int, timeouts Timeouts) {
	if shutdownContext().Err() != nil {
		_warn(fmt.Sprintf("Shutdown signal received, skipping service: %s", colorize(ColorCyan, s.Name)))
		return
	}

	if !runPreScript(s) {
		deps.MarkFailed(s.Name)
		return
	}

	if !waitForServiceDependencies(s, deps, timeouts) {
		deps.MarkFailed(s.Name)
		return
	}

	serviceDone := make(chan error, 1)
	go func() {
		serviceDone <- startService(s, maxLength, timeouts)
	}()

	postScriptDone := make(chan struct{})
	go runPostScript(s, timeouts.PostScript, postScriptDone)

	err := <-serviceDone

	var startErr *startupError
	switch {
	case errors.As(err, &startErr):
		deps.MarkFailed(s.Name)
		handleServiceError(s, err)
	case s.Oneshot && err != nil:
		deps.MarkFailed(s.Name)
	case s.Oneshot:
		deps.MarkReady(s.Name)
	}

	<-postScriptDone
}

func runPreScript(s *Service) bool {
	if s.PreScript == "" {
		return true
	}

	_info("| === PRE-SCRIPT START --- [SERVICE: ", s.Name, "] === |")

	if err := runScript(s.PreScript); err != nil {
		_error("[PRE-SCRIPT ERROR] Error executing pre-script for service ", s.Name, ": ", err)
		if s.Required {
			_error("[CRITICAL] Required service ", s.Name, " pre-script failed, initiating shutdown")
			triggerServiceFailureShutdown()
		}
		return false
	}

	_info("| === PRE-SCRIPT END --- [SERVICE: ", s.Name, "] === |")
	return true
}

func waitForServiceDependencies(s *Service, deps *dependencyTracker, timeouts Timeouts) bool {
	if len(s.DependsOn) == 0 {
		return true
	}

	_info(fmt.Sprintf("Service '%s' waiting for dependencies: %s",
		colorize(ColorCyan, s.Name),
		colorize(ColorYellow, strings.Join(s.DependsOn, ", "))))

	for _, dep := range s.DependsOn {
		waitTime := 0
		if s.WaitAfter != nil {
			waitTime = s.WaitAfter.GetWaitTime(dep)
		}
		if !waitForDependency(deps, dep, waitTime, timeouts.DependencyWait) {
			_warn(fmt.Sprintf("Dependency wait canceled for service: %s", colorize(ColorCyan, s.Name)))
			return false
		}
	}
	return true
}

func runPostScript(s *Service, postScriptTimeout int, done chan<- struct{}) {
	defer close(done)

	select {
	case <-time.After(time.Duration(postScriptTimeout) * time.Second):
	case <-shutdownContext().Done():
		return
	}

	if s.PosScript == "" {
		return
	}

	_info("| === POST-SCRIPT START --- [SERVICE: ", s.Name, "] === |")

	if err := runScript(s.PosScript); err != nil {
		_error("[POST-SCRIPT ERROR] Error executing post-script for service ", s.Name, ": ", err)
		return
	}

	_info("| === POST-SCRIPT END --- [SERVICE: ", s.Name, "] === |")
}

func handleServiceError(s *Service, err error) {
	_error(fmt.Sprintf("Error starting service '%s': %v", colorize(ColorCyan, s.Name), err))
	if s.Required {
		_error(fmt.Sprintf("[CRITICAL] Required service '%s' failed, initiating shutdown",
			colorize(ColorCyan, s.Name)))
		triggerServiceFailureShutdown()
	}
}

func isBashAvailable() bool {
	_, err := exec.LookPath("bash")
	return err == nil
}

func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode()&0o100 != 0 {
		return nil
	}
	return os.Chmod(path, info.Mode()|0o700) // #nosec G302
}

func runScript(scriptPath string) error {
	if err := ensureExecutable(scriptPath); err != nil {
		_warn(fmt.Sprintf("Could not make script %s executable: %v", scriptPath, err))
	}

	cmd := exec.Command(scriptPath) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err := runTracked(cmd)
	if err == nil {
		return nil
	}

	if !isNotExecutableError(err) {
		return err
	}

	shell := "sh"
	if isBashAvailable() {
		shell = "bash"
	}

	fallback := exec.Command(shell, scriptPath) // #nosec G204
	fallback.Stdout = os.Stdout
	fallback.Stderr = os.Stderr
	fallback.Env = os.Environ()
	return runTracked(fallback)
}

func isNotExecutableError(err error) bool {
	return errors.Is(err, syscall.ENOEXEC) || errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES)
}

func waitForDependency(deps *dependencyTracker, depName string, waitAfter, dependencyWait int) bool {
	timeout := time.NewTimer(time.Duration(dependencyWait) * time.Second)
	defer timeout.Stop()

	progress := time.NewTicker(10 * time.Second)
	defer progress.Stop()

	for {
		select {
		case <-deps.Ready(depName):
			return waitAfterDependency(depName, waitAfter)
		case <-deps.Failed(depName):
			_error(fmt.Sprintf("Dependency '%s' failed before becoming ready", colorize(ColorRed, depName)))
			return false
		case <-timeout.C:
			_error(fmt.Sprintf("Dependency wait timeout exceeded for '%s'", colorize(ColorYellow, depName)))
			return false
		case <-shutdownContext().Done():
			return false
		case <-progress.C:
			_info(fmt.Sprintf("Waiting for dependency: %s", colorize(ColorYellow, depName)))
		}
	}
}

func waitAfterDependency(depName string, waitAfter int) bool {
	if waitAfter <= 0 {
		_success(fmt.Sprintf("Dependency '%s' is ready", colorize(ColorGreen, depName)))
		return true
	}

	_info(fmt.Sprintf("Dependency '%s' is up. Waiting %ds before starting dependent service",
		colorize(ColorGreen, depName), waitAfter))

	select {
	case <-time.After(time.Duration(waitAfter) * time.Second):
		return true
	case <-shutdownContext().Done():
		return false
	}
}

func buildServiceCommand(service *Service) (*exec.Cmd, []string, error) {
	cmd := exec.Command(service.Command, service.Args...) // #nosec G204
	env := buildServiceEnv(service)

	if service.User == "" {
		return cmd, env, nil
	}

	credential, homeDir, err := lookupUserCredential(service.User)
	if err != nil {
		return nil, nil, fmt.Errorf("could not resolve user %q for service %s: %w", service.User, service.Name, err)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = credential

	env = overrideEnv(env, map[string]string{
		"USER":    service.User,
		"LOGNAME": service.User,
		"HOME":    homeDir,
	})

	return cmd, env, nil
}

func startService(service *Service, maxLength int, timeouts Timeouts) error {
	select {
	case <-shutdownContext().Done():
		return nil
	default:
	}

	_info(fmt.Sprintf("Starting service: %s", colorize(ColorCyan, service.Name)))

	cmd, env, err := buildServiceCommand(service)
	if err != nil {
		return &startupError{err: err}
	}
	cmd.Env = env

	if len(service.Env) > 0 || service.EnvFile != "" {
		_info(fmt.Sprintf("Service '%s' has custom environment variables configured",
			colorize(ColorCyan, service.Name)))
	}

	ptmx, logFile, err := attachServiceOutput(cmd, service)
	if err != nil {
		return &startupError{err: err}
	}

	pid := cmd.Process.Pid
	trackChild(pid)

	_success(fmt.Sprintf("Service '%s' started successfully (PID: %d)",
		colorize(ColorCyan, service.Name), pid))

	serviceCtx, serviceCancel := context.WithCancel(shutdownContext())

	serviceProcess := &ServiceProcess{
		Name:    service.Name,
		Process: cmd,
		PTY:     ptmx,
		LogFile: logFile,
		Cancel:  serviceCancel,
		State:   ServiceStatePending,
		Config:  *service,
	}

	addActiveService(service.Name, serviceProcess)
	serviceProcess.SetState(ServiceStateRunning)
	startHealthMonitor(serviceProcess)

	if ptmx != nil {
		go streamServiceLogs(ptmx, service.Name, maxLength)
	}

	if !service.Oneshot && serviceDeps != nil {
		serviceDeps.MarkReady(service.Name)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	var exitErr error
	select {
	case exitErr = <-waitDone:
	case <-serviceCtx.Done():
		serviceProcess.RequestStop(stopIntentShutdown)
		exitErr = stopRunningService(serviceProcess, waitDone, timeouts)
	}

	untrackChild(pid)
	if serviceProcess.HealthCancel != nil {
		serviceProcess.HealthCancel()
	}
	serviceCancel()
	closeServiceOutput(ptmx, logFile)

	return finishService(serviceProcess, exitErr)
}

func finishService(serviceProcess *ServiceProcess, exitErr error) error {
	intent := serviceProcess.StopIntent()
	if intent == stopIntentNone && exitErr != nil {
		serviceProcess.SetError(exitErr)
	}
	removeActiveService(serviceProcess.Name)

	switch intent {
	case stopIntentShutdown:
		return nil
	case stopIntentRestartOperator:
		resetRestartCount(serviceProcess.Name)
		scheduleRestart(serviceProcess, 1)
		return nil
	case stopIntentRestartPolicy:
		handleServiceExit(serviceProcess, errUnhealthyService)
		return nil
	default:
		if exitErr == nil {
			_success(fmt.Sprintf("Service '%s' exited cleanly", colorize(ColorCyan, serviceProcess.Name)))
		}
		handleServiceExit(serviceProcess, exitErr)
		return exitErr
	}
}

func attachServiceOutput(cmd *exec.Cmd, service *Service) (ptmx, logFile *os.File, err error) {
	if service.LogFile == "" {
		ptmx, err = pty.Start(cmd)
		if err != nil {
			return nil, nil, fmt.Errorf("error starting PTY for service %s: %w", service.Name, err)
		}
		return ptmx, nil, nil
	}

	logFile, err = os.OpenFile(service.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304
	if err != nil {
		return nil, nil, fmt.Errorf("error opening log file %s for service %s: %w", service.LogFile, service.Name, err)
	}

	_info(fmt.Sprintf("Service '%s' writes output to log file: %s",
		colorize(ColorCyan, service.Name), colorize(ColorYellow, service.LogFile)))

	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true

	if startErr := cmd.Start(); startErr != nil {
		_ = logFile.Close()
		return nil, nil, fmt.Errorf("error starting service %s: %w", service.Name, startErr)
	}

	return nil, logFile, nil
}

func closeServiceOutput(ptmx, logFile *os.File) {
	if ptmx != nil {
		_ = ptmx.Close()
	}
	if logFile != nil {
		_ = logFile.Close()
	}
}

func stopRunningService(sp *ServiceProcess, waitDone chan error, timeouts Timeouts) error {
	sp.SetState(ServiceStateStopping)
	_info(fmt.Sprintf("Gracefully stopping service: %s", colorize(ColorCyan, sp.Name)))

	if sp.Process == nil || sp.Process.Process == nil {
		return <-waitDone
	}

	if err := signalProcess(sp.Process.Process, syscall.SIGTERM); err != nil {
		_debug(fmt.Sprintf("Could not send SIGTERM to service '%s': %v", sp.Name, err))
	}

	shutdownTimeout := time.Duration(timeouts.ServiceShutdown) * time.Second
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	select {
	case err := <-waitDone:
		_success(fmt.Sprintf("Service '%s' stopped gracefully", colorize(ColorCyan, sp.Name)))
		return err
	case <-time.After(shutdownTimeout):
		_warn(fmt.Sprintf("Force killing service '%s' after %s timeout",
			colorize(ColorCyan, sp.Name), shutdownTimeout))
		if err := killProcess(sp.Process.Process); err != nil {
			_error(fmt.Sprintf("Error force killing service '%s': %v", colorize(ColorCyan, sp.Name), err))
		}
		return <-waitDone
	}
}

func streamServiceLogs(reader io.Reader, serviceName string, maxLength int) {
	formattedName := formatServiceName(serviceName, maxLength)
	buffered := bufio.NewReaderSize(reader, logStreamBufferSize)

	for {
		chunk, err := buffered.ReadSlice('\n')
		if len(chunk) > 0 {
			printServiceLine(formattedName, chunk)
		}

		if err == nil {
			continue
		}

		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}

		if isStreamClosed(err) {
			return
		}

		_debug(fmt.Sprintf("Stopped reading logs for service %s: %v", serviceName, err))
		return
	}
}

func printServiceLine(formattedName string, chunk []byte) {
	line := strings.TrimRight(string(chunk), "\r\n")
	if line == "" {
		return
	}
	_printLine(fmt.Sprintf("[%s] %s", formattedName, line))
}

func isStreamClosed(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, os.ErrClosed) ||
		errors.Is(err, syscall.EIO)
}

func getLongestServiceNameLength(services []Service) int {
	maxLength := 0
	for i := range services {
		if len(services[i].Name) > maxLength {
			maxLength = len(services[i].Name)
		}
	}
	return maxLength
}

func formatServiceName(serviceName string, maxLength int) string {
	return fmt.Sprintf("%-*s", maxLength, serviceName)
}

func printServiceStatuses() {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	_printLine(colorize(ColorBoldCyan, "\n=== Service Status Summary ==="))
	for name, serviceProc := range activeServices {
		uptime := time.Since(serviceProc.StartTime).Round(time.Second)
		state := serviceProc.GetState()

		status := fmt.Sprintf("  %s │ State: %s │ Uptime: %s",
			colorize(ColorCyan, fmt.Sprintf("%-15s", name)),
			colorize(getStateColor(state), state.String()),
			colorize(ColorWhite, uptime.String()))

		if lastErr := serviceProc.GetError(); lastErr != nil {
			status += fmt.Sprintf(" │ %s: %s", colorize(ColorRed, "Error"), lastErr)
		}

		_printLine(status)
	}
	_printLine(colorize(ColorBoldCyan, "=== End Status Summary ===\n"))
}
