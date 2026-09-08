package bridge

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestApplyUsesBexDefaults(t *testing.T) {
	env := map[string]string{}
	err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil })
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderHost], DefaultHost; got != want {
		t.Errorf("host = %q, want %q", got, want)
	}
	if got, want := env[renderConfigPath], filepath.Join("/home/alice", ".bex", "cli.yaml"); got != want {
		t.Errorf("config path = %q, want %q", got, want)
	}
}

func TestApplyMapsBexOverrides(t *testing.T) {
	env := map[string]string{
		bexConfigPath: "/tmp/bex.yaml",
		bexHost:       "http://127.0.0.1:8090/v1/",
		bexWorkspace:  "tea-demo",
		bexOutput:     "json",
		bexAccess:     "short-lived-access-token",
	}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for upstream, want := range map[string]string{
		renderConfigPath: "/tmp/bex.yaml",
		renderHost:       "http://127.0.0.1:8090/v1/",
		renderWorkspace:  "tea-demo",
		renderOutput:     "json",
		renderAPIKey:     "short-lived-access-token",
	} {
		if got := env[upstream]; got != want {
			t.Errorf("%s = %q, want %q", upstream, got, want)
		}
	}
}

func TestApplyMapsBexConfigDirectoryToUpstreamPath(t *testing.T) {
	env := map[string]string{bexConfigDir: "/tmp/bex-config"}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) {
		t.Fatal("home lookup must not happen for BEX_CLI_CONFIG_DIR")
		return "", nil
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderConfigPath], "/tmp/bex-config/cli.yaml"; got != want {
		t.Errorf("config path = %q, want %q", got, want)
	}
}

func TestApplyNeverOverridesExplicitRenderConfiguration(t *testing.T) {
	env := map[string]string{
		renderConfigPath: "/home/alice/.render/cli.yaml",
		renderHost:       "https://api.render.com/v1/",
		renderWorkspace:  "tea-render",
		renderOutput:     "yaml",
		renderAPIKey:     "render-access-token",
		bexConfigDir:     "/home/alice/.bex",
		bexHost:          "https://api.bex.co/v1/",
		bexWorkspace:     "tea-bex",
		bexOutput:        "json",
		bexAccess:        "bex-access-token",
	}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) {
		t.Fatal("home lookup must not happen when Render config is explicit")
		return "", nil
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for key, want := range map[string]string{
		renderConfigPath: "/home/alice/.render/cli.yaml",
		renderHost:       "https://api.render.com/v1/",
		renderWorkspace:  "tea-render",
		renderOutput:     "yaml",
		renderAPIKey:     "render-access-token",
	} {
		if got := env[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestApplyTreatsBlankRenderVariablesAsUnset(t *testing.T) {
	env := map[string]string{
		renderConfigPath: "",
		renderHost:       "",
		bexHost:          "http://127.0.0.1:8090/v1/",
	}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderConfigPath], "/home/alice/.bex/cli.yaml"; got != want {
		t.Errorf("config path = %q, want %q", got, want)
	}
	if got, want := env[renderHost], env[bexHost]; got != want {
		t.Errorf("host = %q, want %q", got, want)
	}
}

func TestApplyDisablesUpstreamAnalyticsByDefault(t *testing.T) {
	env := map[string]string{bexHost: "http://127.0.0.1:8090/v1/"}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderDisableAnalytics], "1"; got != want {
		t.Errorf("%s = %q, want %q — bex must not ship default-on Render telemetry", renderDisableAnalytics, got, want)
	}
}

func TestApplyKeepsExplicitAnalyticsOptIn(t *testing.T) {
	// A user who explicitly re-enables upstream analytics (falsey opt-out) has
	// expressed intent; the launcher must not silently override it back to 1.
	env := map[string]string{renderDisableAnalytics: "0"}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderDisableAnalytics], "0"; got != want {
		t.Errorf("%s = %q, want %q (explicit user setting preserved)", renderDisableAnalytics, got, want)
	}
}

func TestApplyTreatsBlankAnalyticsOptOutAsUnset(t *testing.T) {
	// A profile that exports RENDER_CLI_DISABLE_ANALYTICS= (blank) must not
	// leave telemetry on — blank counts as unset, so the launcher fills in "1".
	env := map[string]string{renderDisableAnalytics: "", doNotTrack: ""}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderDisableAnalytics], "1"; got != want {
		t.Errorf("%s = %q, want %q (blank opt-out is unset)", renderDisableAnalytics, got, want)
	}
}

func TestApplyLeavesAnalyticsAloneUnderDoNotTrack(t *testing.T) {
	// DO_NOT_TRACK already denies consent upstream, so there is nothing to add.
	env := map[string]string{doNotTrack: "1"}
	if err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, exists := env[renderDisableAnalytics]; exists {
		t.Errorf("%s = %q, want unset (DO_NOT_TRACK already opts out)", renderDisableAnalytics, got)
	}
}

func TestApplyReportsHomeLookupFailure(t *testing.T) {
	err := apply(lookupFrom(map[string]string{}), setInto(map[string]string{}), func() (string, error) {
		return "", errors.New("home unavailable")
	})
	if err == nil || err.Error() != "resolve home directory: home unavailable" {
		t.Fatalf("error = %v", err)
	}
}

func TestApplyReportsEnvironmentFailure(t *testing.T) {
	err := apply(lookupFrom(map[string]string{}), func(string, string) error { return errors.New("read-only") }, func() (string, error) {
		return "/home/alice", nil
	})
	if err == nil || err.Error() != "set RENDER_CLI_CONFIG_PATH: read-only" {
		t.Fatalf("error = %v", err)
	}
}

func lookupFrom(env map[string]string) lookupEnv {
	return func(key string) (string, bool) {
		value, exists := env[key]
		return value, exists
	}
}

func setInto(env map[string]string) setEnv {
	return func(key, value string) error {
		env[key] = value
		return nil
	}
}
