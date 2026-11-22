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

package sandboxbroker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestSandboxPodWiringEndToEnd verifies the complete w2/m535 runtime path end
// to end on loopback:
//   - ssh-gateway serves ONLY the public Sandbox Broker CA PEM on /sandbox-broker-ca.crt
//   - bex-api's sandbox.BrokerConfig fetches that public CA and produces the
//     exact Pod env + POSIX CA bootstrap script for allow-all & allow-list Pods
//   - running the real bootstrap script merges the public root CA + Broker CA
//   - a standard HTTPS client (and real curl when present) configured ONLY with
//     the Pod's injected HTTPS_PROXY + CA bundle trusts both pass-through public
//     hosts and selectively-intercepted credentialed hosts with zero secrets in
//     env or filesystem
//   - live credential disable takes effect on already-running sandboxes with no
//     restart, and resume + snapshot restore inherit live group credentials
//   - allow-list restricts destinations and deny-all gets neither proxy wiring
//     nor broker access.
func TestSandboxPodWiringEndToEnd(t *testing.T) {
	const (
		ws            = "tea-0123456789abcdefghij"
		groupID       = "sbg-0123456789abcdefghij"
		sbxAllowAll   = "sbx-allowall000000000001"
		sbxRestored   = "sbx-restored000000000002"
		sbxAllowList  = "sbx-allowlst000000000003"
		sbxDenyAll    = "sbx-denyall0000000000004"
		secretInitial = "sk-e2e-initial-secret-999"
		secretRotated = "sk-e2e-rotated-secret-777"
		ipUpstream    = "93.184.216.34"
	)
	hmacSecret := strings.Repeat("a", 32)

	// 1. Upstream HTTPS echo server signed by a test "public" root CA.
	echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := make(map[string]string, len(r.Header))
		for k := range r.Header {
			hdr[k] = strings.Join(r.Header.Values(k), "; ")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(capturedUpstreamReq{
			Host:   r.Host,
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Header: hdr,
		})
	})
	upstreams, upstreamRoots := makeUpstreamRootWithServers(t, echoHandler, echoHandler)
	brokerCA, _ := makeBrokerCA(t)

	// 2. Internal :8081 public-only Broker CA endpoint (mirrors cmd/ssh-gateway).
	caSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sandbox.GatewayBrokerCACertPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(brokerCA.CertPEM())
	}))
	defer caSrv.Close()

	// 3. Control-plane store + OpenBao fake + internal :8091 BrokerLookupHandler.
	st := &e2eStore{
		records: map[string]store.SandboxRecord{},
		members: map[string]store.SandboxGroupMember{},
		groups:  map[string]store.SandboxGroup{ws + "|" + groupID: {ID: groupID, WorkspaceID: ws}},
		creds:   map[string][]store.SandboxCredential{},
	}
	registerSandbox := func(sbxID string, policy *sandbox.NetworkPolicy) {
		view, err := json.Marshal(sandbox.Sandbox{ID: sbxID, SandboxGroupID: groupID, NetworkPolicy: policy})
		if err != nil {
			t.Fatal(err)
		}
		st.mu.Lock()
		st.members[ws+"|"+sbxID] = store.SandboxGroupMember{GroupID: groupID}
		st.records[ws+"|"+sbxID] = store.SandboxRecord{WorkspaceID: ws, SandboxID: sbxID, View: view}
		st.mu.Unlock()
	}
	registerSandbox(sbxAllowAll, &sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll})
	registerSandbox(sbxRestored, &sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll})
	registerSandbox(sbxAllowList, &sandbox.NetworkPolicy{
		Default: sandbox.NetworkPolicyAllowList,
		Rules:   []sandbox.NetworkPolicyRule{{Domain: "api.example.com", Protocol: sandbox.NetworkPolicyProtocolHTTPS}},
	})
	registerSandbox(sbxDenyAll, &sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyDenyAll})

	kv := &e2eKV{}
	vault := sandbox.NewCredentialVault(kv)
	if err := vault.Put(context.Background(), ws, groupID, "cred-api", "ref-1", secretInitial); err != nil {
		t.Fatal(err)
	}
	if err := vault.Put(context.Background(), ws, groupID, "cred-api", "ref-2", secretRotated); err != nil {
		t.Fatal(err)
	}
	setCred := func(enabled bool, ref string) {
		st.mu.Lock()
		st.creds[ws+"|"+groupID] = []store.SandboxCredential{{
			ID:               "cred-api",
			CurrentSecretRef: ref,
			CurrentVersion:   store.SandboxCredentialVersion{ID: 1, Fingerprint: ref},
			Config: store.SandboxCredentialConfig{
				Domain:  "api.example.com",
				Enabled: enabled,
				Methods: []string{"GET"},
				Paths:   []string{"/v1/**"},
				Injection: store.SandboxCredentialInjection{
					Type:     "header",
					Name:     "Authorization",
					Template: "Bearer ${SECRET}",
				},
			},
		}}
		st.mu.Unlock()
	}
	setCred(true, "ref-1")

	var nowMu sync.Mutex
	now := time.Now().UTC()
	clock := func() time.Time {
		nowMu.Lock()
		defer nowMu.Unlock()
		return now
	}
	advanceClock := func(d time.Duration) {
		nowMu.Lock()
		now = now.Add(d)
		nowMu.Unlock()
	}

	lookupSrv := httptest.NewServer(&sandbox.BrokerLookupHandler{
		Secret:   []byte(hmacSecret),
		Resolver: &sandbox.BrokerLookupResolver{Store: st, Vault: vault, Now: clock},
		Nonce:    &e2eNonce{},
		Now:      clock,
	})
	defer lookupSrv.Close()

	// 4. Real KubeSandboxPodResolver over fake k8s clientset.
	k8s := fake.NewSimpleClientset()
	ns := store.SandboxNamespace(ws)
	setActivePod := func(podName, uid, sbxID, egress string) {
		list, _ := k8s.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{})
		for _, p := range list.Items {
			_ = k8s.CoreV1().Pods(ns).Delete(context.Background(), p.Name, metav1.DeleteOptions{})
		}
		_, err := k8s.CoreV1().Pods(ns).Create(context.Background(), &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
				UID:       types.UID(uid),
				Labels: map[string]string{
					appv1alpha1.LabelRegime:         appv1alpha1.RegimeSandbox,
					agentsession.LabelWorkspace:     ws,
					sessionegress.SandboxIDLabel:    sbxID,
					sessionegress.SandboxGroupLabel: groupID,
					sessionegress.EgressLabel:       egress,
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "127.0.0.1"},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	setActivePod("sbx-pod-1", "uid-1", sbxAllowAll, "allow-all")

	metrics := sshgateway.NewMetrics(prometheus.NewRegistry())
	lookupClient := &sandbox.BrokerLookupClient{
		URL:      lookupSrv.URL + sandbox.InternalBrokerLookupPath,
		Secret:   []byte(hmacSecret),
		HTTP:     &http.Client{Timeout: 5 * time.Second},
		CacheTTL: 10 * time.Millisecond,
		Now:      clock,
	}
	podResolver := &KubeSandboxPodResolver{
		Client:     k8s,
		Namespaces: func(context.Context) ([]string, error) { return []string{ns}, nil },
	}
	broker := &Broker{
		Pods:            podResolver,
		API:             lookupClient,
		CA:              brokerCA,
		Metrics:         metrics,
		UpstreamRootCAs: upstreamRoots,
		Resolver: staticResolver{
			"api.example.com":    {netip.MustParseAddr(ipUpstream)},
			"uncred.example.com": {netip.MustParseAddr(ipUpstream)},
		},
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, upstreams.srvA.Listener.Addr().String())
		},
	}
	brokerSrv := httptest.NewServer(broker.Handler())
	defer brokerSrv.Close()

	// 5. Wire bex-api's BrokerConfig to brokerSrv + caSrv and materialize the
	// Pod's CA trust bundle by executing the REAL generated /bin/sh bootstrap.
	brokerCfg := &sandbox.BrokerConfig{
		ProxyURL:  brokerSrv.URL,
		CACertURL: caSrv.URL + sandbox.GatewayBrokerCACertPath,
	}
	baseTmpl := sandbox.Template{
		Image:      "bash:5.3",
		Entrypoint: []string{"/bin/sh", "-c", "exec sleep infinity"},
		Env:        map[string]string{"HOME": "/home/render"},
	}

	materializeRootfs := func(t *testing.T, root string, entry []string) string {
		t.Helper()
		sslDir := filepath.Join(root, "etc", "ssl", "certs")
		if err := os.MkdirAll(sslDir, 0o755); err != nil {
			t.Fatal(err)
		}
		sysBundle := filepath.Join(sslDir, "ca-certificates.crt")
		if _, err := os.Stat(sysBundle); os.IsNotExist(err) {
			if err := os.WriteFile(sysBundle, upstreams.rootPEM, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		script := strings.Replace(entry[2], "exec sleep infinity", ":", 1)
		script = strings.ReplaceAll(script, "/etc/bex", filepath.Join(root, "etc", "bex"))
		script = strings.ReplaceAll(script, "/etc/ssl", filepath.Join(root, "etc", "ssl"))
		script = strings.ReplaceAll(script, "/etc/pki", filepath.Join(root, "etc", "pki"))
		if out, err := exec.Command("/bin/sh", "-c", script).CombinedOutput(); err != nil {
			t.Fatalf("bootstrap script failed: %v\n%s", err, string(out))
		}
		return filepath.Join(root, "etc", "bex", "sandbox-broker-ca-bundle.crt")
	}

	entry, env := brokerCfg.Inject(context.Background(), &sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll}, baseTmpl, nil)
	if env["HTTPS_PROXY"] != brokerSrv.URL || env["SSL_CERT_FILE"] != sandbox.SandboxBrokerCABundleFile {
		t.Fatalf("injected env missing broker wiring: %v", env)
	}
	for k, v := range env {
		if strings.Contains(v, secretInitial) || strings.Contains(v, secretRotated) || strings.Contains(v, "PRIVATE KEY") {
			t.Fatalf("secret/key leaked into env[%q]", k)
		}
	}
	if strings.Contains(entry[2], secretInitial) || strings.Contains(entry[2], "PRIVATE KEY") {
		t.Fatalf("secret/key leaked into entrypoint")
	}

	rootfs := t.TempDir()
	bundlePath := materializeRootfs(t, rootfs, entry)
	bundleBytes, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bundleBytes), secretInitial) || strings.Contains(string(bundleBytes), "PRIVATE KEY") {
		t.Fatalf("secret/key leaked into rootfs CA bundle")
	}
	clientRoots := x509.NewCertPool()
	if !clientRoots.AppendCertsFromPEM(bundleBytes) {
		t.Fatal("failed to parse combined CA bundle")
	}
	proxyParsed, err := url.Parse(env["HTTPS_PROXY"])
	if err != nil {
		t.Fatal(err)
	}
	newSandboxHTTPClient := func() *http.Client {
		return &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(proxyParsed),
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: clientRoots},
			},
		}
	}

	// 6a. Credentialed domain: injected with secretInitial, verified against Broker CA in bundle.
	client := newSandboxHTTPClient()
	resp, err := client.Get("https://api.example.com/v1/items")
	if err != nil {
		t.Fatalf("credentialed HTTPS via broker failed: %v", err)
	}
	var got capturedUpstreamReq
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got.Header["Authorization"] != "Bearer "+secretInitial {
		t.Fatalf("Authorization = %q, want Bearer %s", got.Header["Authorization"], secretInitial)
	}

	// 6b. Uncredentialed domain: pass-through TLS verified against upstream public root CA (NOT Broker CA).
	resp, err = client.Get("https://uncred.example.com/public")
	if err != nil {
		t.Fatalf("uncredentialed pass-through HTTPS failed: %v", err)
	}
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 || resp.TLS.PeerCertificates[0].Issuer.CommonName == "bex-sandbox-broker-ca" {
		t.Fatalf("uncredentialed host must pass through without Broker CA MITM")
	}
	_ = resp.Body.Close()

	// 6c. Real curl check (when installed) using ONLY HTTPS_PROXY + CURL_CA_BUNDLE from the Pod wiring.
	if curlBin, err := exec.LookPath("curl"); err == nil {
		cmd := exec.Command(curlBin, "-fsS", "https://api.example.com/v1/from-curl")
		cmd.Env = []string{
			"HTTPS_PROXY=" + env["HTTPS_PROXY"],
			"NO_PROXY=" + env["NO_PROXY"],
			"CURL_CA_BUNDLE=" + bundlePath,
			"SSL_CERT_FILE=" + bundlePath,
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("curl via broker failed: %v\n%s", err, string(out))
		}
		var curlGot capturedUpstreamReq
		if err := json.Unmarshal(out, &curlGot); err != nil || curlGot.Header["Authorization"] != "Bearer "+secretInitial {
			t.Fatalf("curl upstream got = %s, want Bearer %s", string(out), secretInitial)
		}
	}

	// 6d. Disable credential live -> already-running sandbox sees immediate cutoff without restart.
	setCred(false, "ref-1")
	advanceClock(time.Minute)
	client = newSandboxHTTPClient()
	resp, err = client.Get("https://api.example.com/v1/after-disable")
	if err != nil {
		t.Fatalf("post-disable request failed: %v", err)
	}
	var afterDisable capturedUpstreamReq
	_ = json.NewDecoder(resp.Body).Decode(&afterDisable)
	_ = resp.Body.Close()
	if afterDisable.Header["Authorization"] != "" {
		t.Fatalf("disabled credential still injected: %q", afterDisable.Header["Authorization"])
	}

	// 6e. Resume (re-run bootstrap on same rootfs, new Pod UID) + rotated credential -> receives secretRotated.
	setCred(true, "ref-2")
	advanceClock(time.Minute)
	materializeRootfs(t, rootfs, entry)
	setActivePod("sbx-pod-1-resumed", "uid-1-resumed", sbxAllowAll, "allow-all")
	client = newSandboxHTTPClient()
	resp, err = client.Get("https://api.example.com/v1/after-resume")
	if err != nil {
		t.Fatalf("post-resume request failed: %v", err)
	}
	var afterResume capturedUpstreamReq
	_ = json.NewDecoder(resp.Body).Decode(&afterResume)
	_ = resp.Body.Close()
	if afterResume.Header["Authorization"] != "Bearer "+secretRotated {
		t.Fatalf("post-resume Authorization = %q, want Bearer %s", afterResume.Header["Authorization"], secretRotated)
	}

	// 6f. Snapshot restore (new sandbox ID, rootfs copied from snapshot with zero secrets) -> receives secretRotated.
	restoredEntry, _ := brokerCfg.Inject(context.Background(), &sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll}, baseTmpl, nil)
	materializeRootfs(t, rootfs, restoredEntry)
	setActivePod("sbx-pod-restored", "uid-restored", sbxRestored, "allow-all")
	client = newSandboxHTTPClient()
	resp, err = client.Get("https://api.example.com/v1/after-restore")
	if err != nil {
		t.Fatalf("post-restore request failed: %v", err)
	}
	var afterRestore capturedUpstreamReq
	_ = json.NewDecoder(resp.Body).Decode(&afterRestore)
	_ = resp.Body.Close()
	if afterRestore.Header["Authorization"] != "Bearer "+secretRotated {
		t.Fatalf("post-restore Authorization = %q, want Bearer %s", afterRestore.Header["Authorization"], secretRotated)
	}

	// 6g. allow-list sandbox reaches listed domain with credential, refused on unlisted domain.
	setActivePod("sbx-pod-allowlist", "uid-allowlist", sbxAllowList, "allow-list")
	client = newSandboxHTTPClient()
	resp, err = client.Get("https://api.example.com/v1/allowlisted")
	if err != nil {
		t.Fatalf("allow-list listed domain failed: %v", err)
	}
	var alGot capturedUpstreamReq
	_ = json.NewDecoder(resp.Body).Decode(&alGot)
	_ = resp.Body.Close()
	if alGot.Header["Authorization"] != "Bearer "+secretRotated {
		t.Fatalf("allow-list Authorization = %q, want Bearer %s", alGot.Header["Authorization"], secretRotated)
	}
	if resp, err := client.Get("https://uncred.example.com/blocked"); err == nil {
		_ = resp.Body.Close()
		t.Fatalf("allow-list unlisted domain unexpectedly succeeded: HTTP %d", resp.StatusCode)
	}

	// 6h. deny-all sandbox gets ZERO broker wiring AND is refused by the broker if dialed directly.
	denyEntry, denyEnv := brokerCfg.Inject(context.Background(), &sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyDenyAll}, baseTmpl, nil)
	if _, hasProxy := denyEnv["HTTPS_PROXY"]; hasProxy || strings.Contains(strings.Join(denyEntry, " "), "sandbox-broker-ca") {
		t.Fatalf("deny-all received broker wiring: env=%v", denyEnv)
	}
	setActivePod("sbx-pod-denyall", "uid-denyall", sbxDenyAll, "deny-all")
	client = newSandboxHTTPClient()
	if resp, err := client.Get("https://api.example.com/v1/denied"); err == nil {
		_ = resp.Body.Close()
		t.Fatalf("deny-all sandbox unexpectedly reached broker: HTTP %d", resp.StatusCode)
	}
}
