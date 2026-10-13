package bridge

import (
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/config"
)

// DefaultDashboardURL is the dashboard origin of DefaultHost's control plane.
const DefaultDashboardURL = "https://dashboard.bex.co"

// EnsureDashboardURL gives a config with no dashboard origin the Bex one
// before a command that opens dashboard pages runs (w8/090): upstream saves
// the origin only at device login, so a config that never logged in (a missing
// or version-only file, an environment credential) had none.
//
// It acts only for the production default API, and never replaces an origin
// already saved, so custom and local control planes and explicit overrides
// keep theirs. Only the non-secret origin is written; an environment bearer is
// never persisted. It is best-effort: an unreadable or unwritable config is
// left for upstream to report as it always has.
func EnsureDashboardURL(use bool) {
	if !use || cfg.GetHost() != DefaultHost {
		return
	}
	current, err := config.Load()
	if err != nil || current.DashboardURL != "" || (current.Host != "" && current.Host != DefaultHost) {
		return
	}
	current.DashboardURL = DefaultDashboardURL
	_ = current.Persist()
}
