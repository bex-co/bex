package bridge

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/config"
)

// VersionHeader carries the launcher's own release identity to bex-api.
//
// The upstream CLI names itself in User-Agent (`render-cli/<cfg.Version>`),
// which stays truthful about the pinned upstream release the compatibility
// ledger tracks — so it cannot also carry bex's release. This header is the
// second axis: `POST /v1/cli-telemetry-events` attributes an event to the bex
// build that produced it (w5/m94), and every other bex-api route simply
// ignores it. It is a bex extension, never sent to Render.
const VersionHeader = "X-Bex-CLI-Version"

// InstallVersionHeader stamps VersionHeader on requests the process sends to
// the configured control plane.
//
// It wraps http.DefaultTransport rather than any specific client because the
// upstream CLI builds its API client as &http.Client{} with a nil Transport
// (render-oss/cli pkg/client/client.go), so every upstream request — the
// command's own calls and the detached analytics sender's POST, which
// re-executes this same binary — resolves through the default. Wrapping the
// default is therefore the one seam that covers both without forking upstream
// or reaching into its client construction.
//
// Calling it more than once installs a single wrapper; an empty version
// installs nothing.
func InstallVersionHeader(version string) {
	if version == "" {
		return
	}
	if _, already := http.DefaultTransport.(*versionTransport); already {
		return
	}
	http.DefaultTransport = &versionTransport{
		base:    http.DefaultTransport,
		version: version,
		hosts:   controlPlaneHosts,
	}
}

type versionTransport struct {
	base    http.RoundTripper
	version string
	hosts   func() map[string]struct{}
}

// RoundTrip adds the header only for the control plane. A request to any other
// host — the GitHub release check in internal/update, a model provider reached
// by internal/code — is forwarded exactly as the caller built it, so the
// launcher never announces itself to a third party.
func (t *versionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, ok := t.hosts()[req.URL.Host]; !ok {
		return t.base.RoundTrip(req)
	}
	// RoundTrip must not modify the request it is given, so the header goes on
	// a clone (http.RoundTripper contract).
	clone := req.Clone(req.Context())
	clone.Header.Set(VersionHeader, t.version)
	return t.base.RoundTrip(clone)
}

// controlPlaneHosts is the set of hosts that belong to the configured control
// plane: the API base the bridge pointed upstream at, plus whatever host a
// prior login stored in the CLI config (the two can differ if BEX_HOST changed
// after login). Resolution reads a file, so it is memoized and deferred to the
// first request rather than paid at startup; a missing or unreadable config
// contributes no host rather than failing the request.
var controlPlaneHosts = sync.OnceValue(func() map[string]struct{} {
	hosts := make(map[string]struct{}, 2)
	addHost(hosts, cfg.GetHost())
	if loaded, err := config.Load(); err == nil && loaded != nil {
		addHost(hosts, loaded.Host)
	}
	return hosts
})

func addHost(set map[string]struct{}, raw string) {
	if raw == "" {
		return
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return
	}
	set[parsed.Host] = struct{}{}
}
