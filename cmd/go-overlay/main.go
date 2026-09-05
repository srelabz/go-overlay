package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultConfigPath      = "/services.toml"
	exitCodeServiceFailure = 1
)

var (
	debugMode bool
	version   = "v0.1.3"
)

var (
	shutdownMu     sync.RWMutex
	shutdownCtx    = context.Background()
	shutdownCancel context.CancelFunc
	activeServices = make(map[string]*ServiceProcess)
	servicesMutex  sync.RWMutex
	shutdownWg     sync.WaitGroup
	ipcServer      net.Listener
	configMu       sync.RWMutex
	globalConfig   *Config
	shutdownOnce   sync.Once
	exitCode       atomic.Int32
)

func setShutdownContext(ctx context.Context, cancel context.CancelFunc) {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	shutdownCtx = ctx
	shutdownCancel = cancel
}

func shutdownContext() context.Context {
	shutdownMu.RLock()
	defer shutdownMu.RUnlock()
	return shutdownCtx
}

func cancelShutdown() {
	shutdownMu.RLock()
	cancel := shutdownCancel
	shutdownMu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func setGlobalConfig(config *Config) {
	configMu.Lock()
	defer configMu.Unlock()
	globalConfig = config
}

func currentConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Printf("Go Overlay - Version: %s\n", version)

	configPath := defaultConfigPath

	rootCmd := &cobra.Command{
		Use:           "go-overlay",
		Short:         "Go-based service supervisor like s6-overlay",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if debugMode {
				_printEnvVariables()
			}
			setShutdownContext(context.WithCancel(context.Background()))
			startZombieReaper()
			autoInstallInPath()
			setupSignalHandler()
			if err := startIPCServer(); err != nil {
				_warn(fmt.Sprintf("Could not start IPC server: %v", err))
			}
			return loadServices(configPath)
		},
	}

	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List all services and their status",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return listServices()
		},
	}

	restartCmd := &cobra.Command{
		Use:           "restart [service-name]",
		Short:         "Restart a specific service",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			return restartService(args[0])
		},
	}

	statusCmd := &cobra.Command{
		Use:           "status",
		Short:         "Show overall system status",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return showStatus()
		},
	}

	installCmd := &cobra.Command{
		Use:           "install",
		Short:         "Install go-overlay in system PATH",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return installInPath()
		},
	}

	rootCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug mode")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigPath, "Path to services.toml")
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(installCmd)

	if err := rootCmd.Execute(); err != nil {
		_error("Error:", err)
		return exitCodeServiceFailure
	}

	return int(exitCode.Load())
}

func setupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigChan
		_info("Received signal:", sig)
		_info("Initiating graceful shutdown...")
		gracefulShutdown()
		os.Exit(int(exitCode.Load()))
	}()
}

func triggerServiceFailureShutdown() {
	exitCode.CompareAndSwap(0, exitCodeServiceFailure)
	gracefulShutdown()
}

func gracefulShutdown() {
	shutdownOnce.Do(runShutdown)
}

func runShutdown() {
	_info("Starting graceful shutdown process...")

	if activeServiceCount() > 0 {
		printServiceStatuses()
	}

	cancelShutdown()

	if ipcServer != nil {
		if err := ipcServer.Close(); err != nil && !isStreamClosed(err) {
			_warn(fmt.Sprintf("Could not close IPC server: %v", err))
		}
	}
	removeSocketFileIfPresent()

	if activeServiceCount() == 0 {
		_info("No active services to shutdown")
		return
	}

	globalTimeout := globalShutdownTimeout()

	shutdownTimer := time.NewTimer(globalTimeout)
	defer shutdownTimer.Stop()

	done := make(chan struct{})
	go func() {
		shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		_success("All services stopped gracefully")
	case <-shutdownTimer.C:
		_warn(fmt.Sprintf("Shutdown timeout reached after %s, forcing termination...", globalTimeout))
		forceKillAllServices()
		select {
		case <-done:
			_info("All services stopped after force kill")
		case <-time.After(5 * time.Second):
			_warn("Some services may still be running after force kill timeout")
		}
	}

	_info("Graceful shutdown completed")
}

func globalShutdownTimeout() time.Duration {
	timeout := 30 * time.Second
	if config := currentConfig(); config != nil && config.Timeouts.GlobalShutdown > 0 {
		timeout = time.Duration(config.Timeouts.GlobalShutdown) * time.Second
	}
	return timeout
}

func forceKillAllServices() {
	servicesMutex.RLock()
	processes := make(map[string]*ServiceProcess, len(activeServices))
	for name, serviceProc := range activeServices {
		processes[name] = serviceProc
	}
	servicesMutex.RUnlock()

	for name, serviceProc := range processes {
		if serviceProc.Process == nil || serviceProc.Process.Process == nil {
			continue
		}
		_warn("Force killing service: " + name)
		if err := killProcess(serviceProc.Process.Process); err != nil {
			_error(fmt.Sprintf("Error force killing service %s: %v", name, err))
		}
	}
}

func removeFile(path string) error {
	return os.Remove(path)
}
