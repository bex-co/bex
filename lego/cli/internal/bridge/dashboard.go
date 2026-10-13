package bridge

import (
	"net/url"
	"strings"

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
	if !use || !isDefaultHost(cfg.GetHost()) {
		return
	}
	current, err := config.Load()
	if err != nil || current.DashboardURL != "" || (current.Host != "" && !isDefaultHost(current.Host)) {
		return
	}
	current.DashboardURL = DefaultDashboardURL
	_ = current.Persist()
}

// isDefaultHost reports whether host names DefaultHost's API base in any form
// the client accepts for it: optional trailing slash, hostname case, and an
// omitted or explicit default HTTPS port (w8/090).
func isDefaultHost(host string) bool {
	u, err := url.Parse(strings.TrimSpace(host))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if port := u.Port(); port != "" && port != "443" {
		return false
	}
	return strings.EqualFold(u.Hostname(), defaultHostURL.Hostname()) &&
		strings.TrimSuffix(u.Path, "/") == strings.TrimSuffix(defaultHostURL.Path, "/")
}

var defaultHostURL, _ = url.Parse(DefaultHost)
