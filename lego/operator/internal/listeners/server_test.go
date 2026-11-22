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

package listeners

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func get(t *testing.T, h http.Handler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, body))
	return rec
}

func readURL(uid, ip string) string {
	return Path + "?" + url.Values{"pod": {uid}, "ip": {ip}}.Encode()
}

// Each answer of the reader, through the handler the DaemonSet serves.
func TestHandlerAnswersEachOutcome(t *testing.T) {
	other := process{pid: "100", cgroup: systemdCgroup(otherUID, "besteffort"), fibTrie: fibTrie("10.244.1.6"), tcp: tcpTable("0.0.0.0:7777")}
	h := Handler(fakeProc(t, process{pid: "1", cgroup: "0::/init.scope\n"}, other, podProcess("200", systemdCgroup(podUID, "burstable"))))
	for _, tc := range []struct {
		name, target string
		status       int
		body         string
	}{
		{"the pod's ports", readURL(podUID, podIP), http.StatusOK, `{"ports":[6379,10000,50051],"loopback":[8124,9000,9001]}`},
		{"another pod of the node", readURL(otherUID, "10.244.1.6"), http.StatusOK, `{"ports":[7777]}`},
		{"not on this node", readURL("bb6c1d2e-8a4b-4c5d-9e0f-1a2b3c4d5e71", podIP), http.StatusNotFound, ErrPodNotFound.Error()},
		{"another address", readURL(podUID, "10.244.1.77"), http.StatusConflict, ErrAddressMismatch.Error()},
		{"malformed UID", readURL("../../etc", podIP), http.StatusBadRequest, ErrBadRequest.Error()},
		{"malformed address", readURL(podUID, "10.244.1"), http.StatusBadRequest, ErrBadRequest.Error()},
		{"loopback address", readURL(podUID, "127.0.0.1"), http.StatusBadRequest, ErrBadRequest.Error()},
		{"no address", Path + "?pod=" + podUID, http.StatusBadRequest, ErrBadRequest.Error()},
		{"two pods", readURL(podUID, podIP) + "&pod=" + otherUID, http.StatusBadRequest, ErrBadRequest.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, h, http.MethodGet, tc.target, nil)
			if rec.Code != tc.status || strings.TrimSpace(rec.Body.String()) != tc.body {
				t.Fatalf("GET %s = %d %q, want %d %q", tc.target, rec.Code, rec.Body.String(), tc.status, tc.body)
			}
		})
	}
}

// An empty answer is an empty list, never null, so a client can tell "nothing
// listens" from a malformed reply; a listener only on loopback is named apart.
func TestHandlerAnswersAnEmptyListForNoListener(t *testing.T) {
	p := podProcess("7", systemdCgroup(podUID, "besteffort"))
	p.tcp, p.tcp6 = tcpTable("127.0.0.1:8124"), nil
	rec := get(t, Handler(fakeProc(t, p)), http.MethodGet, readURL(podUID, podIP), nil)
	if want := `{"ports":[],"loopback":[8124]}`; rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("GET = %d %q, want 200 %s", rec.Code, rec.Body.String(), want)
	}
	p.tcp = tcpTable()
	rec = get(t, Handler(fakeProc(t, p)), http.MethodGet, readURL(podUID, podIP), nil)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"ports":[]}` {
		t.Fatalf("GET = %d %q, want 200 {\"ports\":[]}", rec.Code, rec.Body.String())
	}
}

// The answer carries ports only, and an error carries the sentinel's text: no
// path under /proc, process number or kernel message leaves the node.
func TestHandlerRevealsNoPathOrProcess(t *testing.T) {
	gone := process{pid: "4242", cgroup: systemdCgroup(podUID, "burstable"), fibTrie: fibTrie(podIP)}
	root := fakeProc(t, gone)
	rec := get(t, Handler(root), http.MethodGet, readURL(podUID, podIP), nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET with unreadable tables = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, root) || strings.Contains(body, "4242") || strings.Contains(body, "/") {
		t.Fatalf("error reply %q names a path or process", body)
	}
}

// Nothing but a bodiless GET of a read, and health, is served.
func TestHandlerServesNothingElse(t *testing.T) {
	h := Handler(fakeProc(t, podProcess("7", systemdCgroup(podUID, "besteffort"))))
	if rec := get(t, h, http.MethodPost, readURL(podUID, podIP), nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rec.Code)
	}
	if rec := get(t, h, http.MethodGet, readURL(podUID, podIP), strings.NewReader("x")); rec.Code != http.StatusBadRequest {
		t.Fatalf("GET with a body = %d, want 400", rec.Code)
	}
	for _, path := range []string{"/", "/metrics", "/v1/listeners/x", "/7/net/tcp"} {
		if rec := get(t, h, http.MethodGet, path, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

// Ready only while the node's process table is readable.
func TestReadyzReadsTheProcessTable(t *testing.T) {
	root := fakeProc(t, process{pid: "1", cgroup: "0::/init.scope\n"})
	if rec := get(t, Handler(root), http.MethodGet, "/readyz", nil); rec.Code != http.StatusOK {
		t.Fatalf("readyz = %d, want 200", rec.Code)
	}
	if rec := get(t, Handler(t.TempDir()), http.MethodGet, "/readyz", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz without a process table = %d, want 503", rec.Code)
	}
	if rec := get(t, Handler(t.TempDir()), http.MethodGet, "/healthz", nil); rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rec.Code)
	}
}

// The operator's client against the handler: ports and loopback ports on 200,
// and the two refusals the operator acts on as their sentinels.
func TestClientReadsTheHandler(t *testing.T) {
	server := httptest.NewServer(Handler(fakeProc(t, podProcess("200", systemdCgroup(podUID, "burstable")))))
	t.Cleanup(server.Close)
	reader := netip.MustParseAddrPort(strings.TrimPrefix(server.URL, "http://"))
	client := NewClient()
	got, err := client.Read(t.Context(), reader, podUID, podIP)
	if err != nil || !slices.Equal(got.Ports, wantPorts) || !slices.Equal(got.Loopback, []int{8124, 9000, 9001}) {
		t.Fatalf("Read = %+v, %v; want %v and loopback [8124 9000 9001]", got, err, wantPorts)
	}
	if _, err := client.Read(t.Context(), reader, otherUID, podIP); !errors.Is(err, ErrPodNotFound) {
		t.Fatalf("Read of a pod elsewhere: %v, want ErrPodNotFound", err)
	}
	if _, err := client.Read(t.Context(), reader, podUID, "10.244.1.77"); !errors.Is(err, ErrAddressMismatch) {
		t.Fatalf("Read at another address: %v, want ErrAddressMismatch", err)
	}
	if _, err := client.Read(t.Context(), reader, "bad", podIP); err == nil || errors.Is(err, ErrPodNotFound) {
		t.Fatalf("Read of a malformed UID: %v, want a plain failure", err)
	}
}

// A reader from before w8/388 sends no loopback ports, which reads as none.
func TestClientReadsAnAnswerWithoutLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"ports":[8080]}`) }))
	t.Cleanup(server.Close)
	got, err := NewClient().Read(t.Context(), netip.MustParseAddrPort(strings.TrimPrefix(server.URL, "http://")), podUID, podIP)
	if err != nil || !slices.Equal(got.Ports, []int{8080}) || len(got.Loopback) != 0 {
		t.Fatalf("Read = %+v, %v; want [8080] and no loopback port", got, err)
	}
}

// A reply the reader would never send is a failure, not an answer.
func TestClientRefusesAMalformedAnswer(t *testing.T) {
	for _, body := range []string{`{"ports":[0]}`, `{"ports":[65536]}`, `{"ports":[],"loopback":[70000]}`, `not json`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) }))
		_, err := NewClient().Read(t.Context(), netip.MustParseAddrPort(strings.TrimPrefix(server.URL, "http://")), podUID, podIP)
		server.Close()
		if err == nil {
			t.Fatalf("Read of %q succeeded, want a failure", body)
		}
	}
}

// The DaemonSet serves on the port the operator dials, and runs this binary.
func TestTheDaemonSetServesOnPort(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "listener-reader", "daemonset.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var ds struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Command []string `json:"command"`
						Ports   []struct {
							Name          string `json:"name"`
							ContainerPort int    `json:"containerPort"`
						} `json:"ports"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(raw, &ds); err != nil {
		t.Fatal(err)
	}
	containers := ds.Spec.Template.Spec.Containers
	if len(containers) != 1 || !slices.Equal(containers[0].Command, []string{"/listener-reader"}) ||
		len(containers[0].Ports) != 1 || containers[0].Ports[0].ContainerPort != Port {
		t.Fatalf("daemonset containers = %+v, want one /listener-reader serving port %d", containers, Port)
	}
}
