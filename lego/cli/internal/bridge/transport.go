package bridge

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
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

// WorkspaceHeader preserves the active workspace on upstream service-name paths,
// which do not otherwise carry ownerId. The server ignores it for typed ids.
const WorkspaceHeader = "X-Bex-Workspace"

// InstallControlPlaneHeaders adds launcher identity and service workspace scope
// to requests sent to the configured control plane.
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
func InstallControlPlaneHeaders(version string) {
	if version == "" {
		return
	}
	if _, already := http.DefaultTransport.(*controlPlaneTransport); already {
		return
	}
	http.DefaultTransport = &controlPlaneTransport{
		base:      http.DefaultTransport,
		version:   version,
		hosts:     controlPlaneHosts,
		workspace: func() string { selected, _ := config.WorkspaceID(); return selected },
		blocked:   loadBlockedOwner,
	}
}

type controlPlaneTransport struct {
	base      http.RoundTripper
	version   string
	hosts     func() map[string]struct{}
	workspace func() string
	// blocked is the unresolved workspace name barred from ownerId,
	// /owners/{id} paths, and X-Bex-Workspace (see blockedOwner); nil bars nothing.
	blocked func() blockedOwnerInfo
}

// RoundTrip adds the header only for the control plane. A request to any other
// host — the GitHub release check in internal/update, a model provider reached
// by internal/code — is forwarded exactly as the caller built it, so the
// launcher never announces itself to a third party. The one exception is the
// imported CLI's own release check, answered locally (isUpstreamReleaseCheck).
func (t *controlPlaneTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if isUpstreamReleaseCheck(req.URL) {
		return upstreamReleaseCheckRefusal(req), nil
	}
	if _, ok := t.hosts()[req.URL.Host]; !ok {
		return t.base.RoundTrip(req)
	}
	// An unresolvable workspace name left in place for a direct-ID read must
	// fail fast rather than ride along as an ownerId on a fallback search.
	var barred blockedOwnerInfo
	if t.blocked != nil {
		barred = t.blocked()
		if barred.name != "" && carriesBlockedOwner(req, barred.name) {
			return blockedOwnerRefusal(req, barred), nil
		}
	}
	// RoundTrip must not modify the request it is given, so the header goes on
	// a clone (http.RoundTripper contract).
	clone := req.Clone(req.Context())
	clone.Header.Set(VersionHeader, t.version)
	// Read the selection fresh: an interactive command may change workspaces
	// after an earlier request. Other API paths do not consume this header.
	if t.workspace != nil && strings.HasPrefix(req.URL.Path, "/v1/services/") {
		if selected := t.workspace(); selected != "" && selected != barred.name {
			clone.Header.Set(WorkspaceHeader, selected)
		}
	}
	return t.base.RoundTrip(clone)
}

// carriesBlockedOwner reports whether req would send the barred workspace
// name as an ownerId: an ownerId query value or an /owners/{barred} path
// segment. Matching only the owner-ID path position keeps valid direct-ID
// routes like /v1/services/{id} working even when BEX_WORKSPACE is named
// "services", "postgres", or "v1". Other uses of the value (the ?name=
// resolve lookup runs before the bar is set) are untouched.
func carriesBlockedOwner(req *http.Request, barred string) bool {
	for _, v := range req.URL.Query()["ownerId"] {
		if v == barred {
			return true
		}
	}
	segs := strings.Split(req.URL.Path, "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "owners" && segs[i+1] == barred {
			return true
		}
	}
	return false
}

// blockedOwnerRefusal answers a request carrying the barred name locally, in
// the words of the startup refusal, so the command fails fast instead of
// sending the raw name.
func blockedOwnerRefusal(req *http.Request, barred blockedOwnerInfo) *http.Response {
	reason := barred.reason
	if reason == "" {
		reason = bexWorkspace + `: no workspace named "` + barred.name + `"; run ` + "`bex workspaces`" + ` to list yours`
	}
	msg, _ := json.Marshal(map[string]string{
		"message": reason,
	})
	return &http.Response{
		Status:        "400 Bad Request",
		StatusCode:    http.StatusBadRequest,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(strings.NewReader(string(msg))),
		ContentLength: int64(len(msg)),
		Request:       req,
	}
}

// upstreamLatestRelease is the URL the imported CLI's release lookups poll
// (cfg.RepoURL + "/releases/latest": pkg/client/version, and since v2.38.0
// internal/updatenotice, both through http.DefaultTransport).
var upstreamLatestRelease = func() *url.URL {
	parsed, _ := url.Parse(strings.TrimSuffix(cfg.RepoURL, "/") + "/releases/latest")
	return parsed
}()

// isUpstreamReleaseCheck reports whether u is the imported CLI's own release
// check. Upstream compares the pinned Render CLI version against render-oss/cli
// releases and, whenever upstream has moved on, tells a bex user to install
// Render's CLI from render.com: after `bex login` up to v2.37.0, and after any
// command since v2.38.0 (its "Update available" notice, which Apply also turns
// off at its own switch, because a cached answer needs no request). Bex's own
// release channel (internal/update) already owns update hints, so that check is
// answered locally instead of reaching GitHub.
func isUpstreamReleaseCheck(u *url.URL) bool {
	return upstreamLatestRelease != nil &&
		strings.EqualFold(u.Host, upstreamLatestRelease.Host) &&
		strings.TrimSuffix(u.Path, "/") == upstreamLatestRelease.Path
}

// upstreamReleaseCheckRefusal is a non-200 reply: the upstream version client
// treats it as an error, and every caller drops the banner on error.
func upstreamReleaseCheckRefusal(req *http.Request) *http.Response {
	return &http.Response{
		Status:        "404 Not Found",
		StatusCode:    http.StatusNotFound,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{},
		Body:          http.NoBody,
		ContentLength: 0,
		Request:       req,
	}
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
