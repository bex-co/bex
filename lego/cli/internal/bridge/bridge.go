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

	// Upstream telemetry opt-outs. render-oss/cli v2.26.0 made usage analytics
	// opt-out (on by default), sending events — including a stable install id
	// and the active workspace — to Render's telemetry endpoint. That endpoint
	// belongs to Render, not Bex; a bex user never consented to it and gains
	// nothing from it, so the launcher disables it by default. Either upstream
	// opt-out (its own RENDER_CLI_DISABLE_ANALYTICS, or the cross-tool
	// DO_NOT_TRACK convention) already denies consent, so an explicit user
	// setting is left untouched.
	renderDisableAnalytics = "RENDER_CLI_DISABLE_ANALYTICS"
	doNotTrack             = "DO_NOT_TRACK"
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
	if !isSet(lookup, renderConfigPath) {
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
		if isSet(lookup, mapping.upstream) {
			continue
		}
		if value, exists := lookup(mapping.bex); exists && value != "" {
			if err := set(mapping.upstream, value); err != nil {
				return fmt.Errorf("set %s: %w", mapping.upstream, err)
			}
		}
	}

	if !isSet(lookup, renderHost) {
		if err := set(renderHost, DefaultHost); err != nil {
			return fmt.Errorf("set %s: %w", renderHost, err)
		}
	}

	if !isSet(lookup, renderDisableAnalytics) && !isSet(lookup, doNotTrack) {
		if err := set(renderDisableAnalytics, "1"); err != nil {
			return fmt.Errorf("set %s: %w", renderDisableAnalytics, err)
		}
	}
	return nil
}

// isSet reports whether key holds a non-empty value. apply treats a blank
// variable as unset throughout, so a present-but-empty env var never counts.
func isSet(lookup lookupEnv, key string) bool {
	value, exists := lookup(key)
	return exists && value != ""
}
