/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// replaceBinary installs newBin at execPath atomically: it stages the bytes in
// a temp file in the same directory (so the final step is a same-filesystem
// rename) and renames it over the target. rename(2) atomically replaces the
// destination, so any earlier failure leaves the original binary untouched —
// the rollback is implicit. The staged temp file is removed unless it becomes
// the installed binary.
func replaceBinary(execPath string, newBin []byte) error {
	dir := filepath.Dir(execPath)
	mode := os.FileMode(0o755)
	if fi, err := os.Stat(execPath); err == nil {
		mode = fi.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".bex-upgrade-*")
	if err != nil {
		return fmt.Errorf("stage upgrade in %s (need write access to that directory): %w", dir, err)
	}
	tmpName := tmp.Name()
	installed := false
	defer func() {
		if !installed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(newBin); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write staged binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write staged binary: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("set permissions on staged binary: %w", err)
	}
	if err := os.Rename(tmpName, execPath); err != nil {
		return fmt.Errorf("install upgrade over %s (the original binary is unchanged): %w", execPath, err)
	}
	installed = true
	return nil
}

// packageManagerMarkers are path fragments that mean a package manager owns
// the binary. Homebrew is the only managed channel bex ships (cli-release.yml
// publishes the tap formula), plus the Linuxbrew prefix.
var packageManagerMarkers = []string{
	"/Cellar/",                    // Homebrew (both /usr/local and /opt/homebrew)
	"/opt/homebrew/",              // Apple-silicon Homebrew prefix
	"/home/linuxbrew/.linuxbrew/", // Linuxbrew
}

const (
	selfUpgradeCommand     = "bex upgrade"
	homebrewUpgradeCommand = "brew upgrade bex"
)

// packageManagerHint reports whether execPath is owned by a package manager
// and, if so, the upgrade instruction to print instead of self-replacing.
func packageManagerHint(execPath string) (string, bool) {
	if !isPackageManaged(execPath) {
		return "", false
	}
	return "bex was installed with Homebrew — upgrade it with:\n\n    " + homebrewUpgradeCommand + "\n", true
}

func isPackageManaged(execPath string) bool {
	for _, marker := range packageManagerMarkers {
		if strings.Contains(execPath, marker) {
			return true
		}
	}
	return false
}

// InstructionCommand is the command that upgrades the running binary for the
// channel that installed it: Homebrew owns its binary, every other install
// self-replaces through `bex upgrade`. Update notices print it so the user can
// upgrade in one step instead of reading release notes to find out how.
func InstructionCommand() string {
	execPath, err := runningBinaryPath()
	if err != nil {
		return selfUpgradeCommand
	}
	return instructionCommandFor(execPath)
}

func instructionCommandFor(execPath string) string {
	if isPackageManaged(execPath) {
		return homebrewUpgradeCommand
	}
	return selfUpgradeCommand
}
