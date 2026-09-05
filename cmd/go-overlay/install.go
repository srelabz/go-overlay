package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	envAutoInstall = "GO_OVERLAY_AUTO_INSTALL"
	installTarget  = "/usr/local/bin/go-overlay"
)

func autoInstallInPath() {
	raw, ok := os.LookupEnv(envAutoInstall)
	if !ok {
		return
	}

	enabled, valid := parseBoolEnv(raw)
	if !valid {
		_warn(fmt.Sprintf("Ignoring invalid boolean for %s=%q", envAutoInstall, raw))
		return
	}
	if !enabled {
		return
	}

	if err := installInPath(); err != nil {
		_warn(fmt.Sprintf("Could not install in PATH: %v", err))
	}
}

func installInPath() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	execDir := filepath.Dir(execPath)
	for _, pathDir := range []string{"/usr/local/bin", "/usr/bin", "/bin"} {
		if execDir == pathDir {
			_info("Already installed in PATH:", execDir)
			return nil
		}
	}

	if linkTarget, err := os.Readlink(installTarget); err == nil {
		if linkTarget == execPath {
			return nil
		}
		if err := os.Remove(installTarget); err != nil {
			return fmt.Errorf("could not replace existing symlink: %w", err)
		}
	} else if _, statErr := os.Lstat(installTarget); statErr == nil {
		return fmt.Errorf("%s already exists and is not a symlink", installTarget)
	}

	if err := os.Symlink(execPath, installTarget); err != nil {
		return fmt.Errorf("could not create symlink %s: %w (try: ln -sf %s %s)",
			installTarget, err, execPath, installTarget)
	}

	_success("Installed in PATH as 'go-overlay'")
	_info("You can now use: go-overlay list, go-overlay restart <service>, etc.")
	return nil
}
