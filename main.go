package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

var debugMode bool
var version = "v0.1.0"

// Socket path for inter-process communication
const socketPath = "/tmp/go-overlay.sock"

// Service states enum
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

// Command types for IPC
type CommandType string

const (
	CmdListServices   CommandType = "list_services"
	CmdRestartService CommandType = "restart_service"
	CmdGetStatus      CommandType = "get_status"
)

// IPC message structures
type IPCCommand struct {
	Type        CommandType `json:"type"`
	ServiceName string      `json:"service_name,omitempty"`
}

type ServiceInfo struct {
	Name      string        `json:"name"`
	State     ServiceState  `json:"state"`
	PID       int           `json:"pid"`
	Uptime    time.Duration `json:"uptime"`
	LastError string        `json:"last_error,omitempty"`
	Required  bool          `json:"required"`
}

type IPCResponse struct {
	Success  bool          `json:"success"`
	Message  string        `json:"message,omitempty"`
	Services []ServiceInfo `json:"services,omitempty"`
}

// Global variables for graceful shutdown
var (
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	activeServices = make(map[string]*ServiceProcess)
	servicesMutex  sync.RWMutex
	shutdownWg     sync.WaitGroup
	
	// IPC server
	ipcServer net.Listener
	globalConfig *Config
)

// Configuration timeouts
type Timeouts struct {
	PostScript      int `toml:"post_script_timeout,omitempty"`
	ServiceShutdown int `toml:"service_shutdown_timeout,omitempty"`
	GlobalShutdown  int `toml:"global_shutdown_timeout,omitempty"`
	DependencyWait  int `toml:"dependency_wait_timeout,omitempty"`
}

type Service struct {
	Name      string   `toml:"name"`
	Command   string   `toml:"command"`
	Args      []string `toml:"args"`
	LogFile   string   `toml:"log_file,omitempty"`
	PreScript string   `toml:"pre_script,omitempty"`
	PosScript string   `toml:"pos_script,omitempty"`
	DependsOn string   `toml:"depends_on,omitempty"`
	WaitAfter int      `toml:"wait_after,omitempty"`
	Enabled   *bool    `toml:"enabled,omitempty"` // Changed to pointer to detect if set
	User      string   `toml:"user,omitempty"`
	
	// New validation and state fields
	Required bool `toml:"required,omitempty"` // If true, failure stops whole system
}

type Config struct {
	Services []Service `toml:"services"`
	Timeouts Timeouts  `toml:"timeouts,omitempty"`
}

type ServiceProcess struct {
	Name    string
	Process *exec.Cmd
	PTY     *os.File
	Cancel  context.CancelFunc
	State   ServiceState
	StateMu sync.RWMutex
	LastError error
	StartTime time.Time
	Config    Service // Store original config for restart
}

// Service state management methods
func (sp *ServiceProcess) SetState(state ServiceState) {
	sp.StateMu.Lock()
	defer sp.StateMu.Unlock()
	oldState := sp.State
	sp.State = state
	_info("Service", sp.Name, "state changed from", oldState, "to", state)
}

func (sp *ServiceProcess) GetState() ServiceState {
	sp.StateMu.RLock()
	defer sp.StateMu.RUnlock()
	return sp.State
}

func (sp *ServiceProcess) SetError(err error) {
	sp.StateMu.Lock()
	defer sp.StateMu.Unlock()
	sp.LastError = err
	if err != nil {
		sp.State = ServiceStateFailed
		_info("Service", sp.Name, "failed with error:", err)
	}
}

func (sp *ServiceProcess) GetPID() int {
	if sp.Process != nil && sp.Process.Process != nil {
		return sp.Process.Process.Pid
	}
	return 0
}

// Validation errors
type ValidationError struct {
	Field   string
	Service string
	Message string
}

func (e ValidationError) Error() string {
	if e.Service != "" {
		return fmt.Sprintf("validation error in service '%s', field '%s': %s", e.Service, e.Field, e.Message)
	}
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// Auto-install function to create symlink in PATH
func autoInstallInPath() {
	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		_info("Warning: Could not determine executable path:", err)
		return
	}

	// Check if we're already in a standard PATH location
	pathDirs := []string{"/usr/local/bin", "/usr/bin", "/bin"}
	execDir := filepath.Dir(execPath)
	
	for _, pathDir := range pathDirs {
		if execDir == pathDir {
			_info("Already installed in PATH:", execDir)
			return
		}
	}

	// Target installation path
	targetPath := "/usr/local/bin/go-overlay"
	
	// Check if symlink already exists and points to our executable
	if linkTarget, err := os.Readlink(targetPath); err == nil {
		if linkTarget == execPath {
			return // Already correctly installed
		}
		// Remove existing symlink if it points somewhere else
		os.Remove(targetPath)
	}

	// Create symlink
	if err := os.Symlink(execPath, targetPath); err != nil {
		_info("Warning: Could not create symlink in PATH:", err)
		_info("You can manually run: sudo ln -sf", execPath, targetPath)
		return
	}

	_info("Auto-installed in PATH as 'go-overlay'")
	_info("You can now use: go-overlay list, go-overlay restart <service>, etc.")
}

func main() {
	fmt.Printf("Go Overlay - Version: %s\n", version)

	var rootCmd = &cobra.Command{
		Use:   "go-overlay",
		Short: "Go-based service supervisor like s6-overlay",
		RunE: func(cmd *cobra.Command, args []string) error {
			if debugMode {
				_printEnvVariables()
			}

			// Auto-install in PATH for easier CLI usage
			autoInstallInPath()

			// Initialize shutdown context
			shutdownCtx, shutdownCancel = context.WithCancel(context.Background())

			// Setup signal handler
			setupSignalHandler()

			// Start IPC server
			if err := startIPCServer(); err != nil {
				_info("Warning: Could not start IPC server:", err)
			}

			return loadServices("/services.toml")
		},
	}

	// List services command
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all services and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listServices()
		},
	}

	// Restart service command
	var restartCmd = &cobra.Command{
		Use:   "restart [service-name]",
		Short: "Restart a specific service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return restartService(args[0])
		},
	}

	// Status command
	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show overall system status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showStatus()
		},
	}

	// Install command - manual installation
	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Install go-overlay in system PATH",
		RunE: func(cmd *cobra.Command, args []string) error {
			autoInstallInPath()
			return nil
		},
	}

	// Add flags
	rootCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug mode")

	// Add subcommands
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(installCmd)

	if err := rootCmd.Execute(); err != nil {
		_info("Error:", err)
		os.Exit(1)
	}
}

func setupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigChan
		_info("Received signal:", sig)
		_info("Initiating graceful shutdown...")
		gracefulShutdown()
		os.Exit(0)
	}()
}

func gracefulShutdown() {
	_info("Starting graceful shutdown process...")
	
	// Print current service statuses only if we have active services
	if len(activeServices) > 0 {
		printServiceStatuses()
	}
	
	// Cancel the shutdown context to signal all services to stop
	// Only if it was initialized (daemon mode)
	if shutdownCancel != nil {
		shutdownCancel()
	}

	// Close IPC server
	if ipcServer != nil {
		ipcServer.Close()
	}

	// Remove socket file
	os.Remove(socketPath)

	// If no active services, we can exit early
	if len(activeServices) == 0 {
		_info("No active services to shutdown")
		return
	}

	// Get global shutdown timeout (default 30s if not configured)
	globalTimeout := 30 * time.Second
	servicesMutex.RLock()
	if len(activeServices) > 0 {
		// Use default timeout since we can't easily access config here
		globalTimeout = 30 * time.Second
	}
	servicesMutex.RUnlock()

	shutdownTimer := time.NewTimer(globalTimeout)
	defer shutdownTimer.Stop()

	// Channel to signal when all services have stopped
	done := make(chan struct{})
	
	go func() {
		shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		_info("All services stopped gracefully")
	case <-shutdownTimer.C:
		_info("Shutdown timeout reached after", globalTimeout, ", forcing termination...")
		forceKillAllServices()
		// Give a bit more time for force kill to complete
		select {
		case <-done:
			_info("All services stopped after force kill")
		case <-time.After(5 * time.Second):
			_info("Some services may still be running after force kill timeout")
		}
	}

	_info("Graceful shutdown completed")
}

func forceKillAllServices() {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	for name, serviceProc := range activeServices {
		if serviceProc.Process != nil && serviceProc.Process.Process != nil {
			_info("Force killing service:", name)
			if err := serviceProc.Process.Process.Kill(); err != nil {
				_info("Error force killing service", name, ":", err)
			}
		}
	}
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
		if serviceProc.PTY != nil {
			serviceProc.PTY.Close()
		}
		delete(activeServices, name)
		shutdownWg.Done()
	}
}

func loadServices(configFile string) error {
	_info("Loading services from ", configFile)

	file, err := os.Open(configFile)
	if err != nil {
		return fmt.Errorf("error opening config file %s: %v", configFile, err)
	}
	defer file.Close()

	var config Config
	if err := toml.NewDecoder(file).Decode(&config); err != nil {
		return fmt.Errorf("error parsing config file %s: %v", configFile, err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	// Store global config for restart functionality
	globalConfig = &config

	_info("Configuration validated successfully")
    _info(fmt.Sprintf(
        "Timeouts configured: PostScript=%ds, ServiceShutdown=%ds, GlobalShutdown=%ds",
        config.Timeouts.PostScript,
        config.Timeouts.ServiceShutdown,
        config.Timeouts.GlobalShutdown,
    ))

	startedServices := make(map[string]bool)
	var mu sync.Mutex

	maxLength := getLongestServiceNameLength(config.Services)

	var wg sync.WaitGroup
	for _, service := range config.Services {
		// Skip disabled services
		if service.Enabled != nil && !*service.Enabled {
			_info("Service ", service.Name, " is disabled, skipping")
			continue
		}

		wg.Add(1)
		go func(s Service, timeouts Timeouts) {
			defer wg.Done()

			// Check for shutdown signal before starting
			select {
			case <-shutdownCtx.Done():
				_info("Shutdown signal received, skipping service:", s.Name)
				return
			default:
			}

			if s.PreScript != "" {
				_info("| === PRE-SCRIPT START --- [SERVICE: ", s.Name, "] === |")

				if err := os.Chmod(s.PreScript, 0755); err != nil {
					_info("[PRE-SCRIPT ERROR] Error setting execute permission for script ", s.PreScript, ": ", err)
					return
				}

				if err := runScript(s.PreScript); err != nil {
					_info("[PRE-SCRIPT ERROR] Error executing pre-script for service ", s.Name, ": ", err)
					if s.Required {
						_info("[CRITICAL] Required service ", s.Name, " pre-script failed, initiating shutdown")
						gracefulShutdown()
					}
					return
				}

				_info("| === PRE-SCRIPT END --- [SERVICE: ", s.Name, "] === |")
			}

			if s.DependsOn != "" {
				_info("Service ", s.Name, " waiting for dependency: ", s.DependsOn)
				if !waitForDependency(s.DependsOn, s.WaitAfter, &mu, startedServices, timeouts.DependencyWait) {
					_info("Dependency wait cancelled for service:", s.Name)
					return
				}
			}

			// Create channel to signal service completion
			serviceDone := make(chan error, 1)
			serviceExited := make(chan struct{})

			// Start service in goroutine
			go func() {
				err := startServiceWithPTY(s, maxLength, timeouts)
				serviceDone <- err
				close(serviceExited)
			}()

			// Wait for service to start
			mu.Lock()
			startedServices[s.Name] = true
			mu.Unlock()

			// Wait for service to exit and handle post-script
			postScriptDone := make(chan struct{})
			go func() {
				// Wait for service to be ready or shutdown signal
				postScriptTimeout := time.Duration(timeouts.PostScript) * time.Second
				select {
				case <-time.After(postScriptTimeout):
					// Service should be ready now
				case <-shutdownCtx.Done():
					close(postScriptDone)
					return
				}

				if s.PosScript != "" {
					_info("| === POST-SCRIPT START --- [SERVICE: ", s.Name, "] === |")

					if err := os.Chmod(s.PosScript, 0755); err != nil {
						_info("[POST-SCRIPT ERROR] Error setting execute permission for script ", s.PosScript, ": ", err)
						close(postScriptDone)
						return
					}

					if err := runScript(s.PosScript); err != nil {
						_info("[POST-SCRIPT ERROR] Error executing post-script for service ", s.Name, ": ", err)
						close(postScriptDone)
						return
					}

					_info("| === POST-SCRIPT END --- [SERVICE: ", s.Name, "] === |")
				}
				close(postScriptDone)
			}()

			// Handle service errors
			if err := <-serviceDone; err != nil {
				_info("Error starting service ", s.Name, ": ", err)
				if s.Required {
					_info("[CRITICAL] Required service ", s.Name, " failed, initiating shutdown")
					gracefulShutdown()
				}
			}

			// Wait for post-script to complete
			<-postScriptDone

		}(service, config.Timeouts)
	}

	wg.Wait()

	// Print service statuses
	printServiceStatuses()

	// Keep the main process alive until shutdown signal
	select {
	case <-shutdownCtx.Done():
		_info("Shutdown signal received, stopping all services...")
	}

	return nil
}

func isBashAvailable() bool {
	_, err := exec.LookPath("bash")
	return err == nil
}

func runScript(scriptPath string) error {
	shell := "sh"
	if isBashAvailable() {
		shell = "bash"
	}

	cmd := exec.Command(shell, "-c", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}

func waitForDependency(depName string, waitAfter int, mu *sync.Mutex, startedServices map[string]bool, dependencyWait int) bool {
	maxWait := time.Duration(dependencyWait) * time.Second
	start := time.Now()

	for {
		// Check for shutdown signal
		select {
		case <-shutdownCtx.Done():
			return false
		default:
		}

		// Check for timeout
		if time.Since(start) > maxWait {
			_info("Dependency wait timeout exceeded for", depName)
			return false
		}

		mu.Lock()
		depStarted := startedServices[depName]
		mu.Unlock()

		if depStarted {
			_info("Dependency ", depName, " is up. Waiting ", waitAfter, " seconds before starting dependent service.")
			
			// Wait with cancellation support
			select {
			case <-time.After(time.Duration(waitAfter) * time.Second):
				return true
			case <-shutdownCtx.Done():
				return false
			}
		}

		_info("Waiting for dependency: ", depName)
		
		// Sleep with cancellation support
		select {
		case <-time.After(2 * time.Second):
			continue
		case <-shutdownCtx.Done():
			return false
		}
	}
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func startServiceWithPTY(service Service, maxLength int, timeouts Timeouts) error {
	if service.LogFile != "" {
		_info("Service ", service.Name, " is configured to use log file: ", service.LogFile)
		go tailLogFile(service.LogFile, service.Name)
		return nil
	}

	_info("Starting service: ", service.Name)

	var cmd *exec.Cmd
	
	// Use exec.Command directly instead of shell when possible
	if len(service.Args) > 0 {
		// If we have args, pass them directly to the command
		cmd = exec.Command(service.Command, service.Args...)
	} else {
		// No args, just the command
		cmd = exec.Command(service.Command)
	}

	// Handle user switching if specified
	if service.User != "" {
		// For user switching, we need to use shell
		fullCommand := service.Command
		if len(service.Args) > 0 {
			fullCommand = fmt.Sprintf("%s %s", service.Command, joinArgs(service.Args))
		}
		
		shell := "sh"
		if isBashAvailable() {
			shell = "bash"
		}
		
		cmd = exec.Command("su", "-s", shell, "-c", fullCommand, service.User)
	}

	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("error starting PTY for service %s: %v", service.Name, err)
	}

	_info("Service ", service.Name, " started successfully (PID: ", cmd.Process.Pid, ")")

	// Create service context for graceful shutdown
	serviceCtx, serviceCancel := context.WithCancel(shutdownCtx)
	
	// Register the service as active
	serviceProcess := &ServiceProcess{
		Name:    service.Name,
		Process: cmd,
		PTY:     ptmx,
		Cancel:  serviceCancel,
		State:   ServiceStatePending,
		Config:  service,
	}
	addActiveService(service.Name, serviceProcess)

	// Mark service as running once it's started
	serviceProcess.SetState(ServiceStateRunning)

	// Start log processing in background
	go prefixLogs(ptmx, service.Name, maxLength)

	// Handle graceful shutdown
	go func() {
		<-serviceCtx.Done()
		serviceProcess.SetState(ServiceStateStopping)
		_info("Gracefully stopping service:", service.Name)
		
		// Send SIGTERM to the process
		if cmd.Process != nil {
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				_info("Error sending SIGTERM to service", service.Name, ":", err)
				serviceProcess.SetError(err)
			}
			
			// Wait for graceful shutdown with configurable timeout
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()
			
			shutdownTimeout := time.Duration(timeouts.ServiceShutdown) * time.Second
			select {
			case <-time.After(shutdownTimeout):
				// Force kill if not stopped gracefully
				_info("Force killing service", service.Name, "after", shutdownTimeout)
				if err := cmd.Process.Kill(); err != nil {
					_info("Error force killing service", service.Name, ":", err)
					serviceProcess.SetError(err)
				}
				<-done // Wait for the process to actually exit
			case err := <-done:
				if err != nil {
					_info("Service", service.Name, "exited with error:", err)
					serviceProcess.SetError(err)
				} else {
					_info("Service", service.Name, "stopped gracefully")
				}
			}
		}
		
		// Clean up
		if ptmx != nil {
			ptmx.Close()
		}
		removeActiveService(service.Name)
	}()

	// Wait for the command to complete or context cancellation
	select {
	case <-serviceCtx.Done():
		return nil
	default:
		err := cmd.Wait()
		// Service exited on its own, clean up
		serviceCancel()
		if err != nil {
			serviceProcess.SetError(err)
		}
		removeActiveService(service.Name)
		return err
	}
}

func prefixLogs(reader *os.File, serviceName string, maxLength int) {
	formattedName := formatServiceName(serviceName, maxLength)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			fmt.Printf("[%s] %s\n", formattedName, line)
		}
	}
	if err := scanner.Err(); err != nil {
		_info("Error reading logs for service ", serviceName, ": ", err)
	}
}

func getLongestServiceNameLength(services []Service) int {
	maxLength := 0
	for _, service := range services {
		if len(service.Name) > maxLength {
			maxLength = len(service.Name)
		}
	}
	return maxLength
}

func formatServiceName(serviceName string, maxLength int) string {
	return fmt.Sprintf("%-*s", maxLength, serviceName)
}

func tailLogFile(filePath, serviceName string) {
	file, err := os.Open(filePath)
	if err != nil {
		_info("Error opening log file for service ", serviceName, ": ", err)
		return
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_info("Error seeking log file for service ", serviceName, ": ", err)
		return
	}

	scanner := bufio.NewScanner(file)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-shutdownCtx.Done():
			_info("Stopping log tailing for service:", serviceName)
			return
		case <-ticker.C:
			for scanner.Scan() {
				line := scanner.Text()
				_print(fmt.Sprintf("[%s] %s", serviceName, line))
			}
			if err := scanner.Err(); err != nil {
				_info("Error reading log file for service ", serviceName, ": ", err)
				return
			}
		}
	}
}

func _info(a ...interface{}) {
	_table("INFO", a...)
}

func _print(a ...interface{}) {
	message := fmt.Sprint(a...)
	fmt.Println(message)
}

func _debug(isDebug bool, a ...interface{}) {
	if isDebug && !debugMode {
		return
	}
	message := fmt.Sprint(a...)
	fmt.Println(message)
}

func _table(level string, a ...interface{}) {
	prefix := fmt.Sprintf("[%s]", level)
	message := fmt.Sprint(a...)
	fmt.Println(prefix, message)
}

func _printEnvVariables() {
	_info("Function entry logged.")
	_debug(true, "| ---------------- START - ENVIRONMENT VARS ---------------- |")

	envVars := os.Environ()
	for i, env := range envVars {
		if i == len(envVars)-1 {
			fmt.Printf("%s", env)
		} else {
			fmt.Printf("%s\n", env)
		}
	}

	_debug(true, "| ---------------- CLOSE - ENVIRONMENT VARS ---------------- |")
}

// Validation functions
func validateConfig(config *Config) error {
	var errors ValidationErrors

	// Set default timeouts if not specified
	if config.Timeouts.PostScript == 0 {
		config.Timeouts.PostScript = 7
	}
	if config.Timeouts.ServiceShutdown == 0 {
		config.Timeouts.ServiceShutdown = 10
	}
	if config.Timeouts.GlobalShutdown == 0 {
		config.Timeouts.GlobalShutdown = 30
	}
	if config.Timeouts.DependencyWait == 0 {
		config.Timeouts.DependencyWait = 300 // 5 minutes
	}

	// Validate services
	serviceNames := make(map[string]bool)
	for i, service := range config.Services {
		// Validate service
		if errs := validateService(service); len(errs) > 0 {
			errors = append(errors, errs...)
		}

		// Check for duplicate service names
		if serviceNames[service.Name] {
			errors = append(errors, ValidationError{
				Field:   "name",
				Service: service.Name,
				Message: "duplicate service name",
			})
		}
		serviceNames[service.Name] = true

		// Set default enabled if not specified
		if service.Enabled == nil {
			config.Services[i].Enabled = new(bool)
			*config.Services[i].Enabled = true
		}
	}

	// Validate dependencies
	if err := validateDependencies(config.Services); err != nil {
		errors = append(errors, ValidationError{
			Field:   "dependencies",
			Message: err.Error(),
		})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func validateService(service Service) ValidationErrors {
	var errors ValidationErrors

	// Validate required fields
	if service.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Service: service.Name,
			Message: "service name is required",
		})
	}

	if service.Command == "" {
		errors = append(errors, ValidationError{
			Field:   "command",
			Service: service.Name,
			Message: "command is required",
		})
	}

	// Validate service name format (alphanumeric, dash, underscore)
	if service.Name != "" {
		validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !validName.MatchString(service.Name) {
			errors = append(errors, ValidationError{
				Field:   "name",
				Service: service.Name,
				Message: "service name must contain only alphanumeric characters, dashes, and underscores",
			})
		}
	}

	// Validate command exists (if it's not a shell command)
	if service.Command != "" && !strings.Contains(service.Command, " ") {
		if _, err := exec.LookPath(service.Command); err != nil {
			// Check if it's an absolute path
			if !filepath.IsAbs(service.Command) {
				errors = append(errors, ValidationError{
					Field:   "command",
					Service: service.Name,
					Message: fmt.Sprintf("command '%s' not found in PATH", service.Command),
				})
			} else {
				// Check if absolute path exists
				if _, err := os.Stat(service.Command); os.IsNotExist(err) {
					errors = append(errors, ValidationError{
						Field:   "command",
						Service: service.Name,
						Message: fmt.Sprintf("command file '%s' does not exist", service.Command),
					})
				}
			}
		}
	}

	// Validate scripts exist if specified
	if service.PreScript != "" {
		if _, err := os.Stat(service.PreScript); os.IsNotExist(err) {
			errors = append(errors, ValidationError{
				Field:   "pre_script",
				Service: service.Name,
				Message: fmt.Sprintf("pre-script file '%s' does not exist", service.PreScript),
			})
		}
	}

	if service.PosScript != "" {
		if _, err := os.Stat(service.PosScript); os.IsNotExist(err) {
			errors = append(errors, ValidationError{
				Field:   "pos_script",
				Service: service.Name,
				Message: fmt.Sprintf("post-script file '%s' does not exist", service.PosScript),
			})
		}
	}

	// Validate log file directory exists if specified
	if service.LogFile != "" {
		logDir := filepath.Dir(service.LogFile)
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			errors = append(errors, ValidationError{
				Field:   "log_file",
				Service: service.Name,
				Message: fmt.Sprintf("log file directory '%s' does not exist", logDir),
			})
		}
	}

	// Validate wait_after is reasonable
	if service.WaitAfter < 0 || service.WaitAfter > 300 {
		errors = append(errors, ValidationError{
			Field:   "wait_after",
			Service: service.Name,
			Message: "wait_after must be between 0 and 300 seconds",
		})
	}

	// Validate user exists if specified
	if service.User != "" {
		if _, err := exec.Command("id", service.User).Output(); err != nil {
			errors = append(errors, ValidationError{
				Field:   "user",
				Service: service.Name,
				Message: fmt.Sprintf("user '%s' does not exist", service.User),
			})
		}
	}

	return errors
}

func validateDependencies(services []Service) error {
	serviceMap := make(map[string]Service)
	for _, service := range services {
		serviceMap[service.Name] = service
	}

	// Check if all dependencies exist
	for _, service := range services {
		if service.DependsOn != "" {
			if _, exists := serviceMap[service.DependsOn]; !exists {
				return fmt.Errorf("service '%s' depends on non-existent service '%s'", service.Name, service.DependsOn)
			}
		}
	}

	// Check for circular dependencies
	for _, service := range services {
		if hasCycles(service.Name, serviceMap, make(map[string]bool), make(map[string]bool)) {
			return fmt.Errorf("circular dependency detected involving service '%s'", service.Name)
		}
	}

	return nil
}

func hasCycles(serviceName string, serviceMap map[string]Service, visited, recursionStack map[string]bool) bool {
	visited[serviceName] = true
	recursionStack[serviceName] = true

	service, exists := serviceMap[serviceName]
	if !exists {
		return false
	}

	if service.DependsOn != "" {
		if !visited[service.DependsOn] {
			if hasCycles(service.DependsOn, serviceMap, visited, recursionStack) {
				return true
			}
		} else if recursionStack[service.DependsOn] {
			return true
		}
	}

	recursionStack[serviceName] = false
	return false
}

// Enhanced service status reporting
func printServiceStatuses() {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()
	
	_info("=== Service Status Summary ===")
	for name, serviceProc := range activeServices {
		uptime := time.Since(serviceProc.StartTime).Round(time.Second)
		status := fmt.Sprintf("Service: %s | State: %s | Uptime: %s", 
			name, serviceProc.GetState(), uptime)
		
		if serviceProc.LastError != nil {
			status += fmt.Sprintf(" | Last Error: %s", serviceProc.LastError)
		}
		
		_info(status)
	}
	_info("=== End Status Summary ===")
}

// IPC functions
func startIPCServer() error {
	// Remove existing socket
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %v", err)
	}

	ipcServer = listener
	_info("IPC server started at", socketPath)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-shutdownCtx.Done():
					return // Shutting down
				default:
					_info("Error accepting IPC connection:", err)
					continue
				}
			}

			go handleIPCConnection(conn)
		}
	}()

	return nil
}

func handleIPCConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var cmd IPCCommand
	if err := decoder.Decode(&cmd); err != nil {
		_info("Error decoding IPC command:", err)
		return
	}

	var response IPCResponse

	switch cmd.Type {
	case CmdListServices:
		response = handleListServices()
	case CmdRestartService:
		response = handleRestartService(cmd.ServiceName)
	case CmdGetStatus:
		response = handleGetStatus()
	default:
		response = IPCResponse{
			Success: false,
			Message: "Unknown command type",
		}
	}

	if err := encoder.Encode(response); err != nil {
		_info("Error encoding IPC response:", err)
	}
}

func handleListServices() IPCResponse {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	var services []ServiceInfo
	for name, serviceProc := range activeServices {
		var lastError string
		if serviceProc.LastError != nil {
			lastError = serviceProc.LastError.Error()
		}

		services = append(services, ServiceInfo{
			Name:      name,
			State:     serviceProc.GetState(),
			PID:       serviceProc.GetPID(),
			Uptime:    time.Since(serviceProc.StartTime),
			LastError: lastError,
			Required:  serviceProc.Config.Required,
		})
	}

	return IPCResponse{
		Success:  true,
		Services: services,
	}
}

func handleRestartService(serviceName string) IPCResponse {
	servicesMutex.Lock()
	defer servicesMutex.Unlock()

	serviceProc, exists := activeServices[serviceName]
	if !exists {
		return IPCResponse{
			Success: false,
			Message: fmt.Sprintf("Service '%s' not found", serviceName),
		}
	}

	_info("Restarting service:", serviceName)

	// Stop the current service
	serviceProc.SetState(ServiceStateStopping)
	if serviceProc.Cancel != nil {
		serviceProc.Cancel()
	}

	// Wait a moment for graceful stop
	time.Sleep(2 * time.Second)

	// Force kill if still running
	if serviceProc.Process != nil && serviceProc.Process.Process != nil {
		if err := serviceProc.Process.Process.Kill(); err != nil {
			_info("Error killing service during restart:", err)
		}
	}

	// Clean up
	if serviceProc.PTY != nil {
		serviceProc.PTY.Close()
	}
	delete(activeServices, serviceName)

	// Restart the service
	go func() {
		time.Sleep(1 * time.Second) // Brief pause before restart
		
		if globalConfig != nil {
			maxLength := getLongestServiceNameLength(globalConfig.Services)
			if err := startServiceWithPTY(serviceProc.Config, maxLength, globalConfig.Timeouts); err != nil {
				_info("Error restarting service", serviceName, ":", err)
			}
		}
	}()

	return IPCResponse{
		Success: true,
		Message: fmt.Sprintf("Service '%s' restart initiated", serviceName),
	}
}

func handleGetStatus() IPCResponse {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	totalServices := len(activeServices)
	runningServices := 0
	failedServices := 0

	for _, serviceProc := range activeServices {
		state := serviceProc.GetState()
		if state == ServiceStateRunning {
			runningServices++
		} else if state == ServiceStateFailed {
			failedServices++
		}
	}

	message := fmt.Sprintf("Total: %d, Running: %d, Failed: %d", 
		totalServices, runningServices, failedServices)

	return IPCResponse{
		Success: true,
		Message: message,
	}
}

// Client functions for CLI commands
func sendIPCCommand(cmd IPCCommand) (*IPCResponse, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Go Overlay daemon: %v", err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(cmd); err != nil {
		return nil, fmt.Errorf("error sending command: %v", err)
	}

	var response IPCResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("error receiving response: %v", err)
	}

	return &response, nil
}

func listServices() error {
	response, err := sendIPCCommand(IPCCommand{Type: CmdListServices})
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf(response.Message)
	}

	fmt.Printf("%-15s %-10s %-8s %-12s %-8s %s\n", 
		"NAME", "STATE", "PID", "UPTIME", "REQUIRED", "LAST_ERROR")
	fmt.Println(strings.Repeat("-", 80))

	for _, service := range response.Services {
		uptime := service.Uptime.Round(time.Second)
		required := "No"
		if service.Required {
			required = "Yes"
		}

		lastError := service.LastError
		if len(lastError) > 30 {
			lastError = lastError[:27] + "..."
		}

		fmt.Printf("%-15s %-10s %-8d %-12s %-8s %s\n",
			service.Name, service.State, service.PID, uptime, required, lastError)
	}

	return nil
}

func restartService(serviceName string) error {
	response, err := sendIPCCommand(IPCCommand{
		Type:        CmdRestartService,
		ServiceName: serviceName,
	})
	if err != nil {
		return err
	}

	if response.Success {
		fmt.Println(response.Message)
	} else {
		return fmt.Errorf(response.Message)
	}

	return nil
}

func showStatus() error {
	response, err := sendIPCCommand(IPCCommand{Type: CmdGetStatus})
	if err != nil {
		return err
	}

	if response.Success {
		fmt.Println("System Status:", response.Message)
	} else {
		return fmt.Errorf(response.Message)
	}

	return nil
}
