package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

const writeOK = 0x2

const (
	envSocketPath   = "GO_OVERLAY_SOCKET"
	runSocketPath   = "/run/go-overlay.sock"
	tmpSocketPath   = "/tmp/go-overlay.sock"
	ipcConnDeadline = 10 * time.Second
	ipcMaxRequest   = 64 * 1024
)

var socketPath = resolveSocketPath()

func resolveSocketPath() string {
	if custom := strings.TrimSpace(os.Getenv(envSocketPath)); custom != "" {
		return custom
	}
	if isWritableDir("/run") {
		return runSocketPath
	}
	return tmpSocketPath
}

func isWritableDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	return syscall.Access(path, writeOK) == nil
}

type CommandType string

const (
	CmdListServices   CommandType = "list_services"
	CmdRestartService CommandType = "restart_service"
	CmdGetStatus      CommandType = "get_status"
)

type IPCCommand struct {
	Type        CommandType `json:"type"`
	ServiceName string      `json:"service_name,omitempty"`
}

type ServiceInfo struct {
	Name      string        `json:"name"`
	LastError string        `json:"last_error,omitempty"`
	Uptime    time.Duration `json:"uptime"`
	State     ServiceState  `json:"state"`
	PID       int           `json:"pid"`
	Required  bool          `json:"required"`
}

type IPCResponse struct {
	Message  string        `json:"message,omitempty"`
	Services []ServiceInfo `json:"services,omitempty"`
	Success  bool          `json:"success"`
}

func startIPCServer() error {
	removeSocketFileIfPresent()

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %w", err)
	}

	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to restrict socket permissions: %w", err)
	}

	ipcServer = listener
	_success(fmt.Sprintf("IPC server started at %s", colorize(ColorCyan, socketPath)))

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-shutdownContext().Done():
					return
				default:
					if isStreamClosed(err) {
						return
					}
					_warn(fmt.Sprintf("Error accepting IPC connection: %v", err))
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

	if err := conn.SetDeadline(time.Now().Add(ipcConnDeadline)); err != nil {
		_debug(fmt.Sprintf("Could not set IPC deadline: %v", err))
	}

	decoder := json.NewDecoder(io.LimitReader(conn, ipcMaxRequest))
	encoder := json.NewEncoder(conn)

	var cmd IPCCommand
	if err := decoder.Decode(&cmd); err != nil {
		_debug(fmt.Sprintf("Error decoding IPC command: %v", err))
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
		response = IPCResponse{Success: false, Message: "Unknown command type"}
	}

	if err := encoder.Encode(response); err != nil {
		_debug(fmt.Sprintf("Error encoding IPC response: %v", err))
	}
}

func handleListServices() IPCResponse {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	services := make([]ServiceInfo, 0, len(activeServices))
	for name, serviceProc := range activeServices {
		var lastError string
		if err := serviceProc.GetError(); err != nil {
			lastError = err.Error()
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

	return IPCResponse{Success: true, Services: services}
}

func handleRestartService(serviceName string) IPCResponse {
	servicesMutex.RLock()
	serviceProc, exists := activeServices[serviceName]
	servicesMutex.RUnlock()

	if !exists {
		return IPCResponse{
			Success: false,
			Message: fmt.Sprintf("Service '%s' not found", serviceName),
		}
	}

	if !serviceProc.RequestStop(stopIntentRestartOperator) {
		return IPCResponse{
			Success: false,
			Message: fmt.Sprintf("Service '%s' is already stopping or restarting", serviceName),
		}
	}

	_info("Restarting service:", serviceName)

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
		switch serviceProc.GetState() {
		case ServiceStateRunning:
			runningServices++
		case ServiceStateFailed:
			failedServices++
		}
	}

	return IPCResponse{
		Success: true,
		Message: fmt.Sprintf("Total: %d, Running: %d, Failed: %d",
			totalServices, runningServices, failedServices),
	}
}

func sendIPCCommand(cmd IPCCommand) (*IPCResponse, error) {
	conn, err := net.DialTimeout("unix", socketPath, ipcConnDeadline)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Go Overlay daemon: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(ipcConnDeadline)); err != nil {
		return nil, fmt.Errorf("could not set connection deadline: %w", err)
	}

	if err := json.NewEncoder(conn).Encode(cmd); err != nil {
		return nil, fmt.Errorf("error sending command: %w", err)
	}

	var response IPCResponse
	if err := json.NewDecoder(io.LimitReader(conn, ipcMaxRequest)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error receiving response: %w", err)
	}

	return &response, nil
}

func listServices() error {
	response, err := sendIPCCommand(IPCCommand{Type: CmdListServices})
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("%s", response.Message)
	}

	fmt.Printf("%s %-15s %s %-10s %s %-8s %s %-12s %s %-8s %s %s%s\n",
		ColorBoldWhite, "NAME",
		ColorBoldWhite, "STATE",
		ColorBoldWhite, "PID",
		ColorBoldWhite, "UPTIME",
		ColorBoldWhite, "REQUIRED",
		ColorBoldWhite, "LAST_ERROR", ColorReset)
	fmt.Println(colorize(ColorGray, strings.Repeat("─", 85)))

	for _, service := range response.Services {
		uptime := service.Uptime.Round(time.Second)
		required := colorize(ColorGray, "No")
		if service.Required {
			required = colorize(ColorYellow, "Yes")
		}

		lastError := service.LastError
		if len(lastError) > 30 {
			lastError = lastError[:27] + "..."
		}

		if lastError != "" {
			lastError = colorize(ColorRed, lastError)
		} else {
			lastError = colorize(ColorGray, "-")
		}

		fmt.Printf("%s%-15s%s %s%-10s%s %s%-8d%s %s%-12s%s %s%-8s%s %s\n",
			ColorCyan, service.Name, ColorReset,
			getStateColor(service.State), service.State, ColorReset,
			ColorWhite, service.PID, ColorReset,
			ColorWhite, uptime, ColorReset,
			ColorWhite, required, ColorReset,
			lastError)
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

	if !response.Success {
		return fmt.Errorf("%s", response.Message)
	}

	fmt.Println(colorize(ColorGreen, "✓ "+response.Message))
	return nil
}

func showStatus() error {
	response, err := sendIPCCommand(IPCCommand{Type: CmdGetStatus})
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("%s", response.Message)
	}

	fmt.Printf("%s: %s\n",
		colorize(ColorBoldCyan, "System Status"),
		colorize(ColorGreen, response.Message))
	return nil
}

func removeSocketFile() error {
	return removeFile(socketPath)
}

func removeSocketFileIfPresent() {
	if err := removeSocketFile(); err != nil && !errors.Is(err, os.ErrNotExist) {
		_warn(fmt.Sprintf("Could not remove socket %s: %v", socketPath, err))
	}
}
