package bridge

import (
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client/version"
	"github.com/render-oss/cli/pkg/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

// hostSet builds the resolver newVersionTransport takes, from full URLs.
func hostSet(t *testing.T, raws ...string) func() map[string]struct{} {
	t.Helper()
	set := make(map[string]struct{}, len(raws))
	for _, raw := range raws {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		set[parsed.Host] = struct{}{}
	}
	return func() map[string]struct{} { return set }
}

func TestVersionTransportStampsControlPlaneHost(t *testing.T) {
	var got string
	var seen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, seen = r.Header.Get(VersionHeader), true
	}))
	defer server.Close()

	client := &http.Client{Transport: &controlPlaneTransport{base: http.DefaultTransport, version: "1.2.3", hosts: hostSet(t, server.URL)}}
	resp, err := client.Get(server.URL + "/v1/services")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !seen {
		t.Fatal("handler never ran")
	}
	if got != "1.2.3" {
		t.Fatalf("%s = %q, want the injected bex version", VersionHeader, got)
	}
}

func TestVersionTransportLeavesThirdPartyHostsAlone(t *testing.T) {
	// The release check (GitHub) and provider endpoints go through the same
	// default transport; the launcher must not announce itself to them.
	var header string
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get(VersionHeader)
	}))
	defer third.Close()

	client := &http.Client{Transport: &controlPlaneTransport{base: http.DefaultTransport, version: "1.2.3", hosts: hostSet(t, "https://api.bex.co/v1/")}}
	resp, err := client.Get(third.URL + "/repos/bex-co/bex/releases")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if header != "" {
		t.Fatalf("%s = %q on a third-party host, want it absent", VersionHeader, header)
	}
}

func TestVersionTransportDoesNotMutateCallerRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	transport := &controlPlaneTransport{base: http.DefaultTransport, version: "1.2.3", hosts: hostSet(t, server.URL)}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// http.RoundTripper's contract forbids modifying the supplied request.
	if got := req.Header.Get(VersionHeader); got != "" {
		t.Fatalf("caller request mutated: %s = %q", VersionHeader, got)
	}
}

func TestVersionTransportPreservesExistingHeaders(t *testing.T) {
	var agent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get("User-Agent")
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// The compatibility ledger depends on upstream's User-Agent staying exactly
	// as upstream set it.
	req.Header.Set("User-Agent", "render-cli/2.27.0 (darwin arm64)")
	transport := &controlPlaneTransport{base: http.DefaultTransport, version: "1.2.3", hosts: hostSet(t, server.URL)}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if agent != "render-cli/2.27.0 (darwin arm64)" {
		t.Fatalf("User-Agent = %q, want it forwarded untouched", agent)
	}
}

func TestInstallControlPlaneHeadersIsIdempotentAndSkipsEmpty(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })

	InstallControlPlaneHeaders("")
	if http.DefaultTransport != original {
		t.Fatal("empty version installed a wrapper")
	}

	InstallControlPlaneHeaders("1.2.3")
	first := http.DefaultTransport
	if first == original {
		t.Fatal("version was not installed")
	}
	InstallControlPlaneHeaders("4.5.6")
	if http.DefaultTransport != first {
		t.Fatal("second install replaced the transport; wrapping must happen once")
	}
}

func TestAddHostIgnoresUnusableValues(t *testing.T) {
	set := map[string]struct{}{}
	addHost(set, "")
	addHost(set, "://nonsense")
	addHost(set, "not-a-url-with-no-host")
	if len(set) != 0 {
		t.Fatalf("set = %v, want no hosts from unusable values", set)
	}
	addHost(set, "http://localhost:18090/v1/")
	if _, ok := set["localhost:18090"]; !ok {
		t.Fatalf("set = %v, want the host:port key", set)
	}
}

// TestVersionHeaderNameIsPinned is the launcher half of the cross-module drift
// guard described in lego/backend/internal/clitelemetry (TestBexVersionHeaderNameIsPinned).
// bex-api reads this exact name; this module cannot import it, so the literal
// is pinned on both sides instead.
func TestVersionHeaderNameIsPinned(t *testing.T) {
	if VersionHeader != "X-Bex-CLI-Version" {
		t.Fatalf("VersionHeader = %q; bex-api reads X-Bex-CLI-Version "+
			"(lego/backend/internal/clitelemetry). Change both sides or neither.", VersionHeader)
	}
}

func TestWorkspaceTransportUsesEnvironmentAndSavedSelection(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("RENDER_WORKSPACE", "")
	cfg := &config.Config{Workspace: "tea-saved"}
	if err := cfg.Persist(); err != nil {
		t.Fatal(err)
	}
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Get(WorkspaceHeader) }))
	defer server.Close()
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	InstallControlPlaneHeaders("test")
	transport := http.DefaultTransport.(*controlPlaneTransport)
	transport.hosts = hostSet(t, server.URL)
	for _, tc := range []struct{ name, env, want string }{
		{"saved workspace", "", "tea-saved"},
		{"environment id overrides saved", "tea-env", "tea-env"},
		{"environment name", "canary", "canary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_WORKSPACE", tc.env)
			req, _ := http.NewRequest("GET", server.URL+"/v1/services/web/deploys", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if got != tc.want {
				t.Fatalf("workspace header = %q, want %q", got, tc.want)
			}
			if req.Header.Get(WorkspaceHeader) != "" {
				t.Fatal("mutated caller request")
			}
		})
	}
	transport.hosts = hostSet(t, "https://api.bex.co/v1/")
	resp, err := http.Get(server.URL + "/third-party")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got != "" {
		t.Fatal("workspace leaked to third party")
	}
}

func TestWorkspaceHeaderNameIsPinned(t *testing.T) {
	if WorkspaceHeader != "X-Bex-Workspace" {
		t.Fatal("update CLI and backend header together")
	}
}

// roundTripFunc lets a test observe whether a request reached the network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUpstreamReleaseCheckNeverAdvertisesRenderCLI(t *testing.T) {
	// `bex login` ends with upstream's version check, which would otherwise
	// print "render vX is available … render.com/docs/cli" whenever Render
	// ships a release newer than the pin.
	// Release builds inject the pinned upstream version; "dev" skips the check.
	originalVersion := cfg.Version
	cfg.Version = "2.27.0"
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport, cfg.Version = original, originalVersion })
	http.DefaultTransport = &controlPlaneTransport{
		base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			t.Fatalf("release check reached the network: %s", r.URL)
			return nil, nil
		}),
		version: "1.2.3",
		hosts:   hostSet(t, "https://api.bex.co/v1/"),
	}

	newer, err := version.NewClient(cfg.RepoURL).NewVersionAvailable()
	if err == nil || newer != "" {
		t.Fatalf("NewVersionAvailable = %q, %v; want an error so the banner is skipped", newer, err)
	}
}

func TestOtherReleaseLookupsAreForwarded(t *testing.T) {
	var reached []string
	client := &http.Client{Transport: &controlPlaneTransport{
		base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			reached = append(reached, r.URL.String())
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: r}, nil
		}),
		version: "1.2.3",
		hosts:   hostSet(t, "https://api.bex.co/v1/"),
	}}
	for _, u := range []string{
		"https://api.github.com/repos/bex-co/bex/releases",
		"https://api.github.com/repos/render-oss/cli/releases/tags/v2.27.0",
	} {
		resp, err := client.Get(u)
		if err != nil {
			t.Fatalf("get %s: %v", u, err)
		}
		_ = resp.Body.Close()
	}
	if len(reached) != 2 {
		t.Fatalf("forwarded %v; only upstream's latest-release check may be answered locally", reached)
	}
}
