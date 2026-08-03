// Package bridge maps Bex-owned process configuration onto the documented
// configuration seams of the upstream Render CLI. It deliberately does not
// alter the upstream command tree or HTTP client.
package bridge

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultHost = "https://api.bex.co/v1/"

	bexConfigDir  = "BEX_CLI_CONFIG_DIR"
	bexConfigPath = "BEX_CLI_CONFIG_PATH"
	bexHost       = "BEX_HOST"
	bexWorkspace  = "BEX_WORKSPACE"
	bexOutput     = "BEX_OUTPUT"
	bexAccess     = "BEX_ACCESS_TOKEN"

	renderConfigPath = "RENDER_CLI_CONFIG_PATH"
	renderHost       = "RENDER_HOST"
	renderWorkspace  = "RENDER_WORKSPACE"
	renderOutput     = "RENDER_OUTPUT"
	renderAPIKey     = "RENDER_API_KEY"
)

// Apply installs Bex defaults only when their upstream equivalents are absent.
// The upstream CLI exposes a config *path*, not a config-directory variable,
// so directory inputs are resolved to cli.yaml before delegation. That
// preserves an explicit RENDER_* configuration for developers who
// intentionally use the imported upstream CLI against another target.
func Apply() error {
	return apply(os.LookupEnv, os.Setenv, os.UserHomeDir)
}

type lookupEnv func(string) (string, bool)
type setEnv func(string, string) error
type userHomeDir func() (string, error)

func apply(lookup lookupEnv, set setEnv, home userHomeDir) error {
	if value, pathSet := lookup(renderConfigPath); !pathSet || value == "" {
		path := ""
		if value, exists := lookup(bexConfigPath); exists && value != "" {
			path = value
		} else if dir, exists := lookup(bexConfigDir); exists && dir != "" {
			path = filepath.Join(dir, "cli.yaml")
		} else {
			dir, err := home()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}
			path = filepath.Join(dir, ".bex", "cli.yaml")
		}
		if err := set(renderConfigPath, path); err != nil {
			return fmt.Errorf("set %s: %w", renderConfigPath, err)
		}
	}

	for _, mapping := range []struct{ upstream, bex string }{
		{renderHost, bexHost},
		{renderWorkspace, bexWorkspace},
		{renderOutput, bexOutput},
		{renderAPIKey, bexAccess},
	} {
		if value, exists := lookup(mapping.upstream); exists && value != "" {
			continue
		}
		if value, exists := lookup(mapping.bex); exists && value != "" {
			if err := set(mapping.upstream, value); err != nil {
				return fmt.Errorf("set %s: %w", mapping.upstream, err)
			}
		}
	}

	if value, exists := lookup(renderHost); !exists || value == "" {
		if err := set(renderHost, DefaultHost); err != nil {
			return fmt.Errorf("set %s: %w", renderHost, err)
		}
	}
	return nil
}
