package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

var debugMode bool
var version = "dev"

type Service struct {
	Name      string   `toml:"name"`
	Command   string   `toml:"command"`
	Args      []string `toml:"args"`
	LogFile   string   `toml:"log_file,omitempty"`
	PreScript string   `toml:"pre_script,omitempty"`
	DependsOn string   `toml:"depends_on,omitempty"`
	WaitAfter int      `toml:"wait_after,omitempty"`
	Enabled   bool     `toml:"enabled,omitempty"`
	User      string   `toml:"user,omitempty"`
}

type Config struct {
	Services []Service `toml:"services"`
}

func main() {
	fmt.Printf("TM Orchestrator - Version: %s\n", version)

	var rootCmd = &cobra.Command{
		Use:   "entrypoint",
		Short: "Custom Docker entrypoint in Go",
		RunE: func(cmd *cobra.Command, args []string) error {
			if debugMode {
				_printEnvVariables()
			}
			return loadServices("/services.toml")
		},
	}

	// --debug
	rootCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug mode")

	if err := rootCmd.Execute(); err != nil {
		_info("Error:", err)
		os.Exit(1)
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

	startedServices := make(map[string]bool)
	var mu sync.Mutex

	maxLength := getLongestServiceNameLength(config.Services)

	var wg sync.WaitGroup
	for _, service := range config.Services {
		if !service.Enabled {
			service.Enabled = true
		}

		if !service.Enabled {
			_info("Service ", service.Name, " is disabled, skipping")
			continue
		}

		wg.Add(1)
		go func(s Service) {
			defer wg.Done()

			if s.PreScript != "" {
				_info("Executing pre-script for service: ", s.Name)

				if err := os.Chmod(s.PreScript, 0755); err != nil {
					_info("Error setting execute permission for script ", s.PreScript, ": ", err)
					return
				}

				if err := runPreScript(s.PreScript); err != nil {
					_info("Error executing pre-script for service ", s.Name, ": ", err)
					return
				}
			}

			if s.DependsOn != "" {
				_info("Service ", s.Name, " waiting for dependency: ", s.DependsOn)
				waitForDependency(s.DependsOn, s.WaitAfter, &mu, startedServices)
			}

			if err := startServiceWithPTY(s, maxLength); err != nil {
				_info("Error starting service ", s.Name, ": ", err)
			}

			mu.Lock()
			startedServices[s.Name] = true
			mu.Unlock()
		}(service)
	}

	wg.Wait()
	return nil
}

func isBashAvailable() bool {
	_, err := exec.LookPath("bash")
	return err == nil
}

func runPreScript(scriptPath string) error {
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

func waitForDependency(depName string, waitAfter int, mu *sync.Mutex, startedServices map[string]bool) {
	for {
		mu.Lock()
		depStarted := startedServices[depName]
		mu.Unlock()

		if depStarted {
			_info("Dependency ", depName, " is up. Waiting ", waitAfter, " seconds before starting dependent service.")
			time.Sleep(time.Duration(waitAfter) * time.Second)
			return
		}

		_info("Waiting for dependency: ", depName)
		time.Sleep(2 * time.Second)
	}
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func startServiceWithPTY(service Service, maxLength int) error {
	if service.LogFile != "" {
		_info("Service ", service.Name, " is configured to use log file: ", service.LogFile)
		go tailLogFile(service.LogFile, service.Name)
		return nil
	}

	_info("Starting service: ", service.Name)

	fullCommand := fmt.Sprintf("%s %s", service.Command, joinArgs(service.Args))
	shell := "sh"
	if isBashAvailable() {
		shell = "bash"
	}

	var cmd *exec.Cmd
	if service.User != "" {
		// Usando su para exe c/ user do services.toml
		fullCommand = fmt.Sprintf("su -c '%s' %s", fullCommand, service.User)
		cmd = exec.Command(shell, "-c", fullCommand)
	} else {
		cmd = exec.Command(shell, "-c", fullCommand)
	}

	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("error starting PTY for service %s: %v", service.Name, err)
	}
	defer func() { _ = ptmx.Close() }()

	_info("Service ", service.Name, " started successfully (PID: ", cmd.Process.Pid, ")")

	go prefixLogs(ptmx, service.Name, maxLength)

	return cmd.Wait()
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
	for {
		for scanner.Scan() {
			line := scanner.Text()
			_print(fmt.Sprintf("[%s] %s", serviceName, line))
		}
		if err := scanner.Err(); err != nil {
			_info("Error reading log file for service ", serviceName, ": ", err)
			return
		}
		time.Sleep(1 * time.Second)
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
