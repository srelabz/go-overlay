package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	envOnlyServices  = "GO_OVERLAY_ONLY_SERVICES"
	envEnablePrefix  = "GO_OVERLAY_ENABLE_"
	envDisablePrefix = "GO_OVERLAY_DISABLE_"
)

func loadEnvFile(filePath string) (map[string]string, error) {
	env := make(map[string]string)

	file, err := os.Open(filePath) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, `"'`)
			env[key] = value
		}
	}

	return env, scanner.Err()
}

func overrideEnv(env []string, overrides map[string]string) []string {
	result := make([]string, 0, len(env)+len(overrides))
	for _, entry := range env {
		key, _, found := strings.Cut(entry, "=")
		if found {
			if _, replaced := overrides[key]; replaced {
				continue
			}
		}
		result = append(result, entry)
	}

	for key, value := range overrides {
		if value == "" {
			continue
		}
		result = append(result, key+"="+value)
	}

	return result
}

func buildServiceEnv(service *Service) []string {
	env := os.Environ()
	envMap := make(map[string]string)

	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if service.EnvFile != "" {
		if fileEnv, err := loadEnvFile(service.EnvFile); err == nil {
			for k, v := range fileEnv {
				envMap[k] = v
			}
			_info(fmt.Sprintf("Loaded %d env vars from %s for service '%s'",
				len(fileEnv), service.EnvFile, service.Name))
		} else {
			_warn(fmt.Sprintf("Could not load env_file '%s' for service '%s': %v",
				service.EnvFile, service.Name, err))
		}
	}

	for k, v := range service.Env {
		envMap[k] = v
	}

	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}

	return result
}

func applyServiceEnvOverrides(config *Config) {
	onlyServices := parseServiceListEnv(os.Getenv(envOnlyServices))
	onlyMode := len(onlyServices) > 0

	for i := range config.Services {
		service := &config.Services[i]
		enabled := isServiceEnabled(service)

		if onlyMode {
			_, enabled = onlyServices[strings.ToLower(service.Name)]
		}

		enableKey := envEnablePrefix + serviceEnvToken(service.Name)
		if raw, ok := os.LookupEnv(enableKey); ok {
			if parsed, valid := parseBoolEnv(raw); valid {
				enabled = parsed
			} else {
				_warn(fmt.Sprintf("Ignoring invalid boolean for %s=%q", enableKey, raw))
			}
		}

		disableKey := envDisablePrefix + serviceEnvToken(service.Name)
		if raw, ok := os.LookupEnv(disableKey); ok {
			if parsed, valid := parseBoolEnv(raw); valid {
				if parsed {
					enabled = false
				}
			} else {
				_warn(fmt.Sprintf("Ignoring invalid boolean for %s=%q", disableKey, raw))
			}
		}

		service.Enabled = new(bool)
		*service.Enabled = enabled
	}
}

func parseServiceListEnv(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\n', '\t':
			return true
		default:
			return false
		}
	})

	for _, token := range tokens {
		name := strings.TrimSpace(token)
		if name == "" {
			continue
		}
		result[strings.ToLower(name)] = struct{}{}
	}

	return result
}

func serviceEnvToken(serviceName string) string {
	var b strings.Builder
	b.Grow(len(serviceName))
	lastUnderscore := false

	for _, r := range strings.ToUpper(serviceName) {
		isAlphaNum := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}

		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}

	return strings.Trim(b.String(), "_")
}

func parseBoolEnv(value string) (parsed, valid bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "y":
		return true, true
	case "0", "false", "no", "off", "n":
		return false, true
	default:
		return false, false
	}
}

var sensitiveEnvMarkers = []string{
	"PASSWORD", "PASSWD", "SECRET", "TOKEN", "APIKEY", "API_KEY",
	"PRIVATE", "CREDENTIAL", "SESSION", "AUTH", "SIGNATURE", "CERT",
}

func isSensitiveEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, marker := range sensitiveEnvMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return strings.HasSuffix(upper, "_KEY") || upper == "KEY"
}

func maskEnvEntry(entry string) string {
	key, value, found := strings.Cut(entry, "=")
	if !found {
		return entry
	}
	if !isSensitiveEnvKey(key) {
		return entry
	}
	if value == "" {
		return key + "="
	}
	return fmt.Sprintf("%s=<redacted:%d chars>", key, len(value))
}

func _printEnvVariables() {
	_debug("| ---------------- START - ENVIRONMENT VARS ---------------- |")

	for _, entry := range os.Environ() {
		_printLine(maskEnvEntry(entry))
	}

	_debug("| ---------------- CLOSE - ENVIRONMENT VARS ---------------- |")
}
