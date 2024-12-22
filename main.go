package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

type Service struct {
	Name    string   `toml:"name"`
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadServices(configFile string) error {
	_debug(true, "Loading services from %s\n", configFile)

	file, err := os.Open(configFile)
	if err != nil {
		return fmt.Errorf("error opening config file %s: %v", configFile, err)
	}
	defer file.Close()

	var config Config
	if err := toml.NewDecoder(file).Decode(&config); err != nil {
		return fmt.Errorf("error parsing config file %s: %v", configFile, err)
	}

	servicesJSON, _ := json.MarshalIndent(config.Services, "", "  ")
	_debug(true, "Loaded services: %s\n", servicesJSON)

	for _, service := range config.Services {
		_debug(true, "Starting service: %s\n", service.Name)
		cmd := exec.Command(service.Command, service.Args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			_debug(true, "Error starting service %s: %v\n", service.Name, err)
		} else {
			_debug(true, "Service %s started successfully (PID: %d)\n", service.Name, cmd.Process.Pid)
		}
	}

	return nil
}

func logFunctionEntry() {
	fmt.Println("Function entry logged.")
}

func _debug(enabled bool, format string, args ...interface{}) {
	if enabled {
		fmt.Printf(format, args...)
	}
}

func _printEnvVariables() {
	logFunctionEntry()
	_debug(true, "START - ENVIRONMENT VARS\n")
	for _, env := range os.Environ() {
		fmt.Println(env)
	}
	_debug(true, "CLOSE - ENVIRONMENT VARS\n")
}
