package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

var debugMode = true

type Service struct {
	Name    string   `toml:"name"`
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	LogFile string   `toml:"log_file,omitempty"`
}

type Config struct {
	Services []Service `toml:"services"`
}

func main() {
	_printEnvVariables()

	var rootCmd = &cobra.Command{
		Use:   "entrypoint",
		Short: "Custom Docker entrypoint in Go",
		RunE: func(cmd *cobra.Command, args []string) error {
			return loadServices("services.toml")
		},
	}

	if err := rootCmd.Execute(); err != nil {
		_info("Error:", err)
		os.Exit(1)
	}
}

func loadServices(configFile string) error {
	_info("Loading services from", configFile)

	file, err := os.Open(configFile)
	if err != nil {
		return fmt.Errorf("error opening config file %s: %v", configFile, err)
	}
	defer file.Close()

	var config Config
	if err := toml.NewDecoder(file).Decode(&config); err != nil {
		return fmt.Errorf("error parsing config file %s: %v", configFile, err)
	}

	var wg sync.WaitGroup
	for _, service := range config.Services {
		wg.Add(1)
		go func(s Service) {
			defer wg.Done()
			if err := startServiceWithPTY(s); err != nil {
				_info("Error starting service", s.Name, ":", err)
			}
		}(service)
	}

	wg.Wait()
	return nil
}

func tailLogFile(filePath, serviceName string) {
	file, err := os.Open(filePath)
	if err != nil {
		_info("Error opening log file for service", serviceName, ":", err)
		return
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_info("Error seeking log file for service", serviceName, ":", err)
		return
	}

	scanner := bufio.NewScanner(file)
	for {
		for scanner.Scan() {
			line := scanner.Text()
			_print(fmt.Sprintf("[%s] %s", serviceName, line))
		}
		if err := scanner.Err(); err != nil {
			_info("Error reading log file for service", serviceName, ":", err)
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func startServiceWithPTY(service Service) error {
	if service.LogFile != "" {
		_info("Service", service.Name, "is configured to use log file:", service.LogFile)
		go tailLogFile(service.LogFile, service.Name)
		return nil
	}

	_info("Starting service:", service.Name)

	cmd := exec.Command(service.Command, service.Args...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("error starting PTY for service %s: %v", service.Name, err)
	}
	defer func() { _ = ptmx.Close() }()

	_info("Service", service.Name, "started successfully (PID:", cmd.Process.Pid, ")")

	go prefixLogs(ptmx, service.Name)

	return cmd.Wait()
}

func prefixLogs(reader *os.File, serviceName string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("[%s] %s\n", serviceName, line)
	}
	if err := scanner.Err(); err != nil {
		_info("Error reading logs for service", serviceName, ":", err)
	}
}

func _info(a ...interface{}) {
	_table("INFO", a...)
}

func _print(a ...interface{}) {
	message := fmt.Sprintln(a...)
	fmt.Println(message)
}

func _debug(isDebug bool, a ...interface{}) {
	if isDebug && !debugMode {
		return
	}
	message := fmt.Sprintln(a...)
	fmt.Println(message)
}

func _table(level string, a ...interface{}) {
	prefix := fmt.Sprintf("[%s]", level)
	message := fmt.Sprintln(a...)
	fmt.Println(prefix, message)
}

func _printEnvVariables() {
	_info("Function entry logged.")
	_debug(true, "START - ENVIRONMENT VARS")
	for _, env := range os.Environ() {
		_print(env)
	}
	_debug(true, "CLOSE - ENVIRONMENT VARS")
}
