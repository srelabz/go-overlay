package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test ServiceState String method
func TestServiceStateString(t *testing.T) {
	tests := []struct {
		state    ServiceState
		expected string
	}{
		{ServiceStatePending, "PENDING"},
		{ServiceStateStarting, "STARTING"},
		{ServiceStateRunning, "RUNNING"},
		{ServiceStateStopping, "STOPPING"},
		{ServiceStateStopped, "STOPPED"},
		{ServiceStateFailed, "FAILED"},
		{ServiceState(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("ServiceState.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test getStateColor function
func TestGetStateColor(t *testing.T) {
	tests := []struct {
		state    ServiceState
		expected string
	}{
		{ServiceStatePending, ColorYellow},
		{ServiceStateStarting, ColorCyan},
		{ServiceStateRunning, ColorGreen},
		{ServiceStateStopping, ColorMagenta},
		{ServiceStateStopped, ColorGray},
		{ServiceStateFailed, ColorRed},
		{ServiceState(999), ColorWhite},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := getStateColor(tt.state); got != tt.expected {
				t.Errorf("getStateColor() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test colorize function
func TestColorize(t *testing.T) {
	tests := []struct {
		name     string
		color    string
		text     string
		expected string
	}{
		{"Red text", ColorRed, "error", ColorRed + "error" + ColorReset},
		{"Green text", ColorGreen, "success", ColorGreen + "success" + ColorReset},
		{"Empty text", ColorBlue, "", ColorBlue + "" + ColorReset},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := colorize(tt.color, tt.text); got != tt.expected {
				t.Errorf("colorize() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test DependsOnField UnmarshalTOML
func TestDependsOnFieldUnmarshalTOML(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expected  []string
		shouldErr bool
	}{
		{
			name:      "Single string",
			input:     "service1",
			expected:  []string{"service1"},
			shouldErr: false,
		},
		{
			name:      "Array of strings",
			input:     []interface{}{"service1", "service2"},
			expected:  []string{"service1", "service2"},
			shouldErr: false,
		},
		{
			name:      "Invalid type",
			input:     123,
			expected:  nil,
			shouldErr: true,
		},
		{
			name:      "Array with non-string",
			input:     []interface{}{"service1", 123},
			expected:  nil,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d DependsOnField
			err := d.UnmarshalTOML(tt.input)

			if tt.shouldErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(d) != len(tt.expected) {
				t.Errorf("Length mismatch: got %d, want %d", len(d), len(tt.expected))
				return
			}

			for i, v := range d {
				if v != tt.expected[i] {
					t.Errorf("Value mismatch at index %d: got %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

// Test WaitAfterField UnmarshalTOML
func TestWaitAfterFieldUnmarshalTOML(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expected  WaitAfterField
		shouldErr bool
	}{
		{
			name:  "Integer value",
			input: int64(5),
			expected: WaitAfterField{
				Global:   5,
				IsPerDep: false,
			},
			shouldErr: false,
		},
		{
			name: "Map value",
			input: map[string]interface{}{
				"service1": int64(10),
				"service2": int64(20),
			},
			expected: WaitAfterField{
				PerDep: map[string]int{
					"service1": 10,
					"service2": 20,
				},
				IsPerDep: true,
			},
			shouldErr: false,
		},
		{
			name:      "Invalid type",
			input:     "invalid",
			expected:  WaitAfterField{},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w WaitAfterField
			err := w.UnmarshalTOML(tt.input)

			if tt.shouldErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if w.IsPerDep != tt.expected.IsPerDep {
				t.Errorf("IsPerDep mismatch: got %v, want %v", w.IsPerDep, tt.expected.IsPerDep)
			}

			if !w.IsPerDep && w.Global != tt.expected.Global {
				t.Errorf("Global mismatch: got %v, want %v", w.Global, tt.expected.Global)
			}
		})
	}
}

// Test WaitAfterField GetWaitTime
func TestWaitAfterFieldGetWaitTime(t *testing.T) {
	tests := []struct {
		name     string
		field    WaitAfterField
		depName  string
		expected int
	}{
		{
			name: "Global wait time",
			field: WaitAfterField{
				Global:   10,
				IsPerDep: false,
			},
			depName:  "any-service",
			expected: 10,
		},
		{
			name: "Per-dep wait time exists",
			field: WaitAfterField{
				PerDep: map[string]int{
					"service1": 15,
					"service2": 20,
				},
				IsPerDep: true,
			},
			depName:  "service1",
			expected: 15,
		},
		{
			name: "Per-dep wait time not found",
			field: WaitAfterField{
				PerDep: map[string]int{
					"service1": 15,
				},
				IsPerDep: true,
			},
			depName:  "service2",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.field.GetWaitTime(tt.depName); got != tt.expected {
				t.Errorf("GetWaitTime() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test validateService
func TestValidateService(t *testing.T) {
	tests := []struct {
		name      string
		service   Service
		shouldErr bool
		errCount  int
	}{
		{
			name: "Valid service",
			service: Service{
				Name:    "test-service",
				Command: "/bin/echo",
			},
			shouldErr: false,
			errCount:  0,
		},
		{
			name: "Missing name",
			service: Service{
				Command: "/bin/echo",
			},
			shouldErr: true,
			errCount:  1,
		},
		{
			name: "Missing command",
			service: Service{
				Name: "test-service",
			},
			shouldErr: true,
			errCount:  1,
		},
		{
			name: "Invalid name characters",
			service: Service{
				Name:    "test service!",
				Command: "/bin/echo",
			},
			shouldErr: true,
			errCount:  1,
		},
		{
			name: "Command not found",
			service: Service{
				Name:    "test-service",
				Command: "nonexistent-command-xyz",
			},
			shouldErr: true,
			errCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateService(tt.service)

			if tt.shouldErr && len(errs) == 0 {
				t.Error("Expected errors but got none")
			}

			if !tt.shouldErr && len(errs) > 0 {
				t.Errorf("Expected no errors but got: %v", errs)
			}

			if tt.errCount > 0 && len(errs) != tt.errCount {
				t.Errorf("Expected %d errors but got %d", tt.errCount, len(errs))
			}
		})
	}
}

// Test validateDependencies
func TestValidateDependencies(t *testing.T) {
	tests := []struct {
		name      string
		services  []Service
		shouldErr bool
	}{
		{
			name: "Valid dependencies",
			services: []Service{
				{Name: "service1", Command: "/bin/echo"},
				{Name: "service2", Command: "/bin/echo", DependsOn: []string{"service1"}},
			},
			shouldErr: false,
		},
		{
			name: "Non-existent dependency",
			services: []Service{
				{Name: "service1", Command: "/bin/echo", DependsOn: []string{"nonexistent"}},
			},
			shouldErr: true,
		},
		{
			name: "Circular dependency",
			services: []Service{
				{Name: "service1", Command: "/bin/echo", DependsOn: []string{"service2"}},
				{Name: "service2", Command: "/bin/echo", DependsOn: []string{"service1"}},
			},
			shouldErr: true,
		},
		{
			name: "Valid wait_after map",
			services: []Service{
				{Name: "service1", Command: "/bin/echo"},
				{
					Name:      "service2",
					Command:   "/bin/echo",
					DependsOn: []string{"service1"},
					WaitAfter: WaitAfterField{
						PerDep:   map[string]int{"service1": 5},
						IsPerDep: true,
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "Invalid wait_after map reference",
			services: []Service{
				{Name: "service1", Command: "/bin/echo"},
				{
					Name:      "service2",
					Command:   "/bin/echo",
					DependsOn: []string{"service1"},
					WaitAfter: WaitAfterField{
						PerDep:   map[string]int{"nonexistent": 5},
						IsPerDep: true,
					},
				},
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDependencies(tt.services)

			if tt.shouldErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// Test hasCycles
func TestHasCycles(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		serviceMap   map[string]Service
		hasCycle     bool
	}{
		{
			name:        "No cycle",
			serviceName: "service1",
			serviceMap: map[string]Service{
				"service1": {Name: "service1", DependsOn: []string{"service2"}},
				"service2": {Name: "service2"},
			},
			hasCycle: false,
		},
		{
			name:        "Direct cycle",
			serviceName: "service1",
			serviceMap: map[string]Service{
				"service1": {Name: "service1", DependsOn: []string{"service2"}},
				"service2": {Name: "service2", DependsOn: []string{"service1"}},
			},
			hasCycle: true,
		},
		{
			name:        "Indirect cycle",
			serviceName: "service1",
			serviceMap: map[string]Service{
				"service1": {Name: "service1", DependsOn: []string{"service2"}},
				"service2": {Name: "service2", DependsOn: []string{"service3"}},
				"service3": {Name: "service3", DependsOn: []string{"service1"}},
			},
			hasCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visited := make(map[string]bool)
			recursionStack := make(map[string]bool)
			got := hasCycles(tt.serviceName, tt.serviceMap, visited, recursionStack)

			if got != tt.hasCycle {
				t.Errorf("hasCycles() = %v, want %v", got, tt.hasCycle)
			}
		})
	}
}

// Test getLongestServiceNameLength
func TestGetLongestServiceNameLength(t *testing.T) {
	tests := []struct {
		name     string
		services []Service
		expected int
	}{
		{
			name:     "Empty services",
			services: []Service{},
			expected: 0,
		},
		{
			name: "Single service",
			services: []Service{
				{Name: "service1"},
			},
			expected: 8,
		},
		{
			name: "Multiple services",
			services: []Service{
				{Name: "short"},
				{Name: "very-long-service-name"},
				{Name: "medium"},
			},
			expected: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLongestServiceNameLength(tt.services); got != tt.expected {
				t.Errorf("getLongestServiceNameLength() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test formatServiceName
func TestFormatServiceName(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		maxLength   int
		expected    string
	}{
		{
			name:        "Exact length",
			serviceName: "service",
			maxLength:   7,
			expected:    "service",
		},
		{
			name:        "Padding needed",
			serviceName: "short",
			maxLength:   10,
			expected:    "short     ",
		},
		{
			name:        "No padding",
			serviceName: "exact",
			maxLength:   5,
			expected:    "exact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatServiceName(tt.serviceName, tt.maxLength); got != tt.expected {
				t.Errorf("formatServiceName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Test joinArgs
func TestJoinArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Empty args",
			args:     []string{},
			expected: "",
		},
		{
			name:     "Single arg",
			args:     []string{"arg1"},
			expected: "arg1",
		},
		{
			name:     "Multiple args",
			args:     []string{"arg1", "arg2", "arg3"},
			expected: "arg1 arg2 arg3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinArgs(tt.args); got != tt.expected {
				t.Errorf("joinArgs() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test ValidationError
func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		expected string
	}{
		{
			name: "With service name",
			err: ValidationError{
				Field:   "command",
				Service: "test-service",
				Message: "command is required",
			},
			expected: "validation error in service 'test-service', field 'command': command is required",
		},
		{
			name: "Without service name",
			err: ValidationError{
				Field:   "timeouts",
				Message: "invalid timeout value",
			},
			expected: "validation error in field 'timeouts': invalid timeout value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("ValidationError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test ServiceProcess state management
func TestServiceProcessSetGetState(t *testing.T) {
	sp := &ServiceProcess{
		Name: "test-service",
	}

	// Test initial state
	sp.SetState(ServiceStatePending)
	if got := sp.GetState(); got != ServiceStatePending {
		t.Errorf("GetState() = %v, want %v", got, ServiceStatePending)
	}

	// Test state transition
	sp.SetState(ServiceStateRunning)
	if got := sp.GetState(); got != ServiceStateRunning {
		t.Errorf("GetState() = %v, want %v", got, ServiceStateRunning)
	}
}

// Test isBashAvailable
func TestIsBashAvailable(t *testing.T) {
	// This test is environment-dependent
	// Just ensure it doesn't panic
	_ = isBashAvailable()
}

// Integration test helper: create a temporary TOML config file
func createTempConfig(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "services.toml")

	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}

	return configPath
}

// Test validateConfig with default timeouts
func TestValidateConfigDefaults(t *testing.T) {
	config := &Config{
		Services: []Service{
			{
				Name:    "test",
				Command: "/bin/echo",
			},
		},
	}

	err := validateConfig(config)
	if err != nil {
		t.Errorf("validateConfig() error = %v, want nil", err)
	}

	// Check default timeouts
	if config.Timeouts.PostScript != 7 {
		t.Errorf("Default PostScript timeout = %v, want 7", config.Timeouts.PostScript)
	}
	if config.Timeouts.ServiceShutdown != 10 {
		t.Errorf("Default ServiceShutdown timeout = %v, want 10", config.Timeouts.ServiceShutdown)
	}
	if config.Timeouts.GlobalShutdown != 30 {
		t.Errorf("Default GlobalShutdown timeout = %v, want 30", config.Timeouts.GlobalShutdown)
	}
	if config.Timeouts.DependencyWait != 300 {
		t.Errorf("Default DependencyWait timeout = %v, want 300", config.Timeouts.DependencyWait)
	}
}

// Benchmark tests
func BenchmarkGetStateColor(b *testing.B) {
	for i := 0; i < b.N; i++ {
		getStateColor(ServiceStateRunning)
	}
}

func BenchmarkColorize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		colorize(ColorGreen, "test message")
	}
}

func BenchmarkValidateService(b *testing.B) {
	service := Service{
		Name:    "test-service",
		Command: "/bin/echo",
	}
	for i := 0; i < b.N; i++ {
		validateService(service)
	}
}

// Test ServiceProcess GetPID
func TestServiceProcessGetPID(t *testing.T) {
	sp := &ServiceProcess{
		Name:    "test-service",
		Process: nil,
	}

	// Test with nil process
	if pid := sp.GetPID(); pid != 0 {
		t.Errorf("GetPID() with nil process = %v, want 0", pid)
	}
}

// Test ServiceProcess SetError
func TestServiceProcessSetError(t *testing.T) {
	sp := &ServiceProcess{
		Name: "test-service",
	}

	// Test setting error
	testErr := os.ErrNotExist
	sp.SetError(testErr)

	if sp.LastError != testErr {
		t.Errorf("LastError = %v, want %v", sp.LastError, testErr)
	}

	if sp.State != ServiceStateFailed {
		t.Errorf("State = %v, want %v", sp.State, ServiceStateFailed)
	}
}

// Test that version is set
func TestVersionSet(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}
}

// Test socket path constant
func TestSocketPath(t *testing.T) {
	expected := "/tmp/go-overlay.sock"
	if socketPath != expected {
		t.Errorf("socketPath = %v, want %v", socketPath, expected)
	}
}

// Mock test for IPC structures
func TestIPCStructures(t *testing.T) {
	// Test IPCCommand
	cmd := IPCCommand{
		Type:        CmdListServices,
		ServiceName: "test-service",
	}
	if cmd.Type != CmdListServices {
		t.Errorf("IPCCommand.Type = %v, want %v", cmd.Type, CmdListServices)
	}

	// Test ServiceInfo
	info := ServiceInfo{
		Name:      "test",
		State:     ServiceStateRunning,
		PID:       123,
		Uptime:    time.Second * 10,
		LastError: "",
		Required:  true,
	}
	if info.Name != "test" {
		t.Errorf("ServiceInfo.Name = %v, want %v", info.Name, "test")
	}

	// Test IPCResponse
	resp := IPCResponse{
		Success: true,
		Message: "OK",
		Services: []ServiceInfo{info},
	}
	if !resp.Success {
		t.Error("IPCResponse.Success should be true")
	}
}
