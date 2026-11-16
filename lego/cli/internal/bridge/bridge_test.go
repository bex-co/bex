package bridge

import (
	"errors"
	"maps"
	"path/filepath"
	"testing"
)

func TestApplyUsesBexDefaults(t *testing.T) {
	env := map[string]string{}
	_, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil })
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderHost], DefaultHost; got != want {
		t.Errorf("host = %q, want %q", got, want)
	}
	if got, want := env[renderConfigDir], filepath.Join("/home/alice", ".bex"); got != want {
		t.Errorf("config dir = %q, want %q", got, want)
	}
	if got, exists := env[renderConfigPath]; exists {
		t.Errorf("%s = %q, want unset (DIR supplies cli.yaml)", renderConfigPath, got)
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
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for upstream, want := range map[string]string{
		renderConfigPath: "/tmp/bex.yaml",
		renderConfigDir:  "/home/alice/.bex",
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

func TestApplyMapsBexConfigDirectoryToUpstreamDir(t *testing.T) {
	env := map[string]string{bexConfigDir: "/tmp/bex-config"}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) {
		t.Fatal("home lookup must not happen for BEX_CLI_CONFIG_DIR")
		return "", nil
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderConfigDir], "/tmp/bex-config"; got != want {
		t.Errorf("config dir = %q, want %q", got, want)
	}
	if got, exists := env[renderConfigPath]; exists {
		t.Errorf("%s = %q, want unset so state lives under the Bex directory", renderConfigPath, got)
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
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) {
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

func TestApplyLeavesRenderConfigDirUnsetSoUpstreamHonorsIt(t *testing.T) {
	env := map[string]string{
		renderConfigDir: "/tmp/rdir",
		bexConfigDir:    "/tmp/bex-config",
	}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) {
		t.Fatal("home lookup must not happen when RENDER_CLI_CONFIG_DIR is set")
		return "", nil
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, exists := env[renderConfigPath]; exists {
		t.Errorf("%s = %q, want unset so upstream reads %s", renderConfigPath, got, renderConfigDir)
	}
	if got := env[renderConfigDir]; got != "/tmp/rdir" {
		t.Errorf("%s = %q, want /tmp/rdir", renderConfigDir, got)
	}
}

func TestApplyRenderConfigDirAloneDoesNotPinDefaultBexPath(t *testing.T) {
	env := map[string]string{renderConfigDir: "/tmp/rdir"}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) {
		t.Fatal("home lookup must not happen when RENDER_CLI_CONFIG_DIR is set")
		return "", nil
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, exists := env[renderConfigPath]; exists {
		t.Errorf("%s = %q, want unset (would otherwise write $HOME/.bex/cli.yaml)", renderConfigPath, got)
	}
}

func TestApplyBlankRenderConfigDirIsUnset(t *testing.T) {
	env := map[string]string{renderConfigDir: "", bexConfigDir: "/tmp/bex-config"}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) {
		t.Fatal("home lookup must not happen for BEX_CLI_CONFIG_DIR")
		return "", nil
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderConfigDir], "/tmp/bex-config"; got != want {
		t.Errorf("config dir = %q, want %q", got, want)
	}
	if got, exists := env[renderConfigPath]; exists {
		t.Errorf("%s = %q, want unset", renderConfigPath, got)
	}
}

func TestApplyTreatsBlankRenderVariablesAsUnset(t *testing.T) {
	env := map[string]string{
		renderConfigPath: "",
		renderHost:       "",
		bexHost:          "http://127.0.0.1:8090/v1/",
	}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderConfigDir], "/home/alice/.bex"; got != want {
		t.Errorf("config dir = %q, want %q", got, want)
	}
	if got := env[renderConfigPath]; got != "" {
		t.Errorf("%s = %q, want blank/unset", renderConfigPath, got)
	}
	if got, want := env[renderHost], env[bexHost]; got != want {
		t.Errorf("host = %q, want %q", got, want)
	}
}

func TestApplySendsTelemetryToBexByDefault(t *testing.T) {
	// Telemetry flows to bex-api (the bridge repoints the API base at Bex, so
	// the imported sender already targets us) unless the user opts out.
	env := map[string]string{bexHost: "http://127.0.0.1:8090/v1/"}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, exists := env[renderDisableAnalytics]; exists {
		t.Errorf("%s = %q, want unset (default sends to bex-api)", renderDisableAnalytics, got)
	}
}

func TestApplyMapsBexAnalyticsOptOut(t *testing.T) {
	env := map[string]string{bexDisableAnalytics: "1"}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := env[renderDisableAnalytics], "1"; got != want {
		t.Errorf("%s = %q, want %q (Bex opt-out mapped through)", renderDisableAnalytics, got, want)
	}
}

func TestApplyKeepsExplicitAnalyticsSetting(t *testing.T) {
	// An explicit upstream setting wins over the Bex opt-out: the user spoke
	// to the imported CLI directly, so the launcher must not override it.
	for _, explicit := range []string{"0", "1"} {
		env := map[string]string{renderDisableAnalytics: explicit, bexDisableAnalytics: "1"}
		if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if got := env[renderDisableAnalytics]; got != explicit {
			t.Errorf("%s = %q, want %q (explicit user setting preserved)", renderDisableAnalytics, got, explicit)
		}
	}
}

func TestApplyTreatsBlankAnalyticsOptOutAsUnset(t *testing.T) {
	// A profile that exports BEX_CLI_DISABLE_ANALYTICS= (blank) has not opted
	// out — blank counts as unset throughout, so telemetry still flows.
	env := map[string]string{bexDisableAnalytics: "", doNotTrack: ""}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, exists := env[renderDisableAnalytics]; exists {
		t.Errorf("%s = %q, want unset (blank opt-out is unset)", renderDisableAnalytics, got)
	}
}

func TestApplyLeavesAnalyticsAloneUnderDoNotTrack(t *testing.T) {
	// DO_NOT_TRACK already denies consent upstream, so there is nothing to add.
	env := map[string]string{doNotTrack: "1"}
	if _, err := apply(lookupFrom(env), setInto(env), func() (string, error) { return "/home/alice", nil }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, exists := env[renderDisableAnalytics]; exists {
		t.Errorf("%s = %q, want unset (DO_NOT_TRACK already opts out)", renderDisableAnalytics, got)
	}
}

// w32/009: the launcher's run-time copy names the Bex variable only for an
// upstream variable the bridge filled, so apply reports exactly those.
func TestApplyReportsWhatItFilledFromBex(t *testing.T) {
	home := func() (string, error) { return "/home/alice", nil }
	for _, tc := range []struct {
		name string
		env  map[string]string
		want map[string]string
	}{
		{"nothing set", map[string]string{}, map[string]string{}},
		{
			"bex inputs only",
			map[string]string{bexAccess: "token", bexOutput: "json", bexWorkspace: "tea-demo", bexHost: "http://127.0.0.1:8090/v1/"},
			map[string]string{renderAPIKey: bexAccess, renderOutput: bexOutput, renderWorkspace: bexWorkspace, renderHost: bexHost},
		},
		{
			"the user's own upstream variable wins and is not reported",
			map[string]string{bexAccess: "token", renderAPIKey: "rnd_user", bexOutput: "json", renderOutput: "yaml"},
			map[string]string{},
		},
		{
			"a blank upstream variable counts as unset, a blank bex input as absent",
			map[string]string{bexAccess: "token", renderAPIKey: "", bexOutput: ""},
			map[string]string{renderAPIKey: bexAccess},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filled, err := apply(lookupFrom(tc.env), setInto(tc.env), home)
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			if !maps.Equal(filled, tc.want) {
				t.Fatalf("filled = %v, want %v", filled, tc.want)
			}
		})
	}
}

// w7/m426: an API key is one variable, as RENDER_API_KEY is for Render's CLI.
// Highest first: the user's own RENDER_API_KEY, BEX_API_KEY, BEX_ACCESS_TOKEN.
// A blank variable counts as unset at every level.
func TestApplyCredentialPrecedence(t *testing.T) {
	home := func() (string, error) { return "/home/alice", nil }
	for _, tc := range []struct {
		name       string
		env        map[string]string
		wantBearer string // RENDER_API_KEY as upstream reads it
		wantFilled string // the Bex input FilledFromBex names, "" for none
	}{
		{"no credential", map[string]string{}, "", ""},
		{"only BEX_API_KEY", map[string]string{bexAPIKey: "bex_id_secret"}, "bex_id_secret", bexAPIKey},
		{"only BEX_ACCESS_TOKEN", map[string]string{bexAccess: "ory_at_token"}, "ory_at_token", bexAccess},
		{
			"both: the key wins",
			map[string]string{bexAPIKey: "bex_id_secret", bexAccess: "ory_at_token"},
			"bex_id_secret", bexAPIKey,
		},
		{
			"RENDER_API_KEY beside BEX_API_KEY is left untouched",
			map[string]string{renderAPIKey: "rnd_user", bexAPIKey: "bex_id_secret"},
			"rnd_user", "",
		},
		{
			"RENDER_API_KEY beside BEX_ACCESS_TOKEN is left untouched",
			map[string]string{renderAPIKey: "rnd_user", bexAccess: "ory_at_token"},
			"rnd_user", "",
		},
		{
			"RENDER_API_KEY beside both is left untouched",
			map[string]string{renderAPIKey: "rnd_user", bexAPIKey: "bex_id_secret", bexAccess: "ory_at_token"},
			"rnd_user", "",
		},
		{
			"a blank BEX_API_KEY falls through to BEX_ACCESS_TOKEN",
			map[string]string{bexAPIKey: "", bexAccess: "ory_at_token"},
			"ory_at_token", bexAccess,
		},
		{
			"a blank BEX_ACCESS_TOKEN beside BEX_API_KEY",
			map[string]string{bexAPIKey: "bex_id_secret", bexAccess: ""},
			"bex_id_secret", bexAPIKey,
		},
		{
			"a blank RENDER_API_KEY is filled from BEX_API_KEY",
			map[string]string{renderAPIKey: "", bexAPIKey: "bex_id_secret", bexAccess: "ory_at_token"},
			"bex_id_secret", bexAPIKey,
		},
		{
			"a blank RENDER_API_KEY and a blank BEX_API_KEY are filled from BEX_ACCESS_TOKEN",
			map[string]string{renderAPIKey: "", bexAPIKey: "", bexAccess: "ory_at_token"},
			"ory_at_token", bexAccess,
		},
		{"every credential blank", map[string]string{renderAPIKey: "", bexAPIKey: "", bexAccess: ""}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filled, err := apply(lookupFrom(tc.env), setInto(tc.env), home)
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			if got := tc.env[renderAPIKey]; got != tc.wantBearer {
				t.Errorf("%s = %q, want %q", renderAPIKey, got, tc.wantBearer)
			}
			if got := filled[renderAPIKey]; got != tc.wantFilled {
				t.Errorf("filled[%s] = %q, want %q", renderAPIKey, got, tc.wantFilled)
			}
		})
	}
}

// The precedence is apply's own: it must not depend on a write to the
// environment being visible to the next lookup.
func TestApplyCredentialPrecedenceDoesNotReadItsOwnWrites(t *testing.T) {
	env := map[string]string{bexAPIKey: "bex_id_secret", bexAccess: "ory_at_token"}
	written := map[string]string{}
	filled, err := apply(lookupFrom(env), setInto(written), func() (string, error) { return "/home/alice", nil })
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, want := written[renderAPIKey], "bex_id_secret"; got != want {
		t.Errorf("%s = %q, want %q (BEX_ACCESS_TOKEN must not overwrite BEX_API_KEY)", renderAPIKey, got, want)
	}
	if got := filled[renderAPIKey]; got != bexAPIKey {
		t.Errorf("filled[%s] = %q, want %q", renderAPIKey, got, bexAPIKey)
	}
}

func TestApplyReportsHomeLookupFailure(t *testing.T) {
	_, err := apply(lookupFrom(map[string]string{}), setInto(map[string]string{}), func() (string, error) {
		return "", errors.New("home unavailable")
	})
	if err == nil || err.Error() != "resolve home directory: home unavailable" {
		t.Fatalf("error = %v", err)
	}
}

func TestApplyReportsEnvironmentFailure(t *testing.T) {
	_, err := apply(lookupFrom(map[string]string{}), func(string, string) error { return errors.New("read-only") }, func() (string, error) {
		return "/home/alice", nil
	})
	if err == nil || err.Error() != "set RENDER_CLI_CONFIG_DIR: read-only" {
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
