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

package agentcred

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// recordingSink is bex-api's share verb with the storage replaced: it records
// what the signed envelope named and the bytes that arrived.
type recordingSink struct {
	envelopes []agentsession.ShareFileEnvelope
	bodies    []string
	refuse    *agentsession.ShareFileError
}

func (s *recordingSink) ShareFile(_ context.Context, envelope agentsession.ShareFileEnvelope, body io.Reader) (agentsession.ShareFileResponse, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return agentsession.ShareFileResponse{}, err
	}
	s.envelopes = append(s.envelopes, envelope)
	s.bodies = append(s.bodies, string(raw))
	if s.refuse != nil {
		return agentsession.ShareFileResponse{}, s.refuse
	}
	return agentsession.ShareFileResponse{AttachmentID: "att-out", Name: envelope.Name, MediaType: "text/plain", Size: envelope.Size}, nil
}

// onceNonces is the durable nonce store's single-use contract in memory.
type onceNonces struct{ seen map[string]bool }

func (n *onceNonces) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	if n.seen[nonce] {
		return false, nil
	}
	n.seen[nonce] = true
	return true, nil
}

func shareHarness(t *testing.T) (*Broker, *recordingSink, *credentialAudit) {
	t.Helper()
	secret := []byte("share-secret")
	sink := &recordingSink{}
	api := httptest.NewServer(&agentsession.ShareFileHandler{Secret: secret, Files: sink, Nonce: &onceNonces{seen: map[string]bool{}}})
	t.Cleanup(api.Close)
	audit := &credentialAudit{}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: credentialLabels(t, "tea-a", "ags-one", "bex-agent/task-1")},
			"10.0.0.2": {Name: "sbx-b-0", UID: "uid-b", Labels: credentialLabels(t, "tea-a", "ags-two", "bex-agent/task-1")},
			"10.0.0.4": {Name: "sbx-a-0", UID: "uid-d", Labels: credentialLabels(t, "tea-b", "ags-one", "bex-agent/task-1")},
		}},
		API:   &agentsession.Client{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Files: &agentsession.ShareFileClient{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Audit: audit,
	}
	return broker, sink, audit
}

func shareURL(t *testing.T, namespace, session string) string {
	t.Helper()
	u, err := agentsession.ShareFileURL("http://gateway.internal:8082", namespace, session)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

type shareReply struct {
	code  int
	raw   string
	error struct{ Code, Message, Hint string }
}

func postShare(t *testing.T, broker *Broker, target, sourceIP, name, body string, mutate func(*http.Request)) shareReply {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	request.RemoteAddr = sourceIP + ":43210"
	sum := sha256.Sum256([]byte(body))
	request.Header.Set(agentsession.ShareFileNameHeader, url.PathEscape(name))
	request.Header.Set(agentsession.ShareFileSHA256Header, hex.EncodeToString(sum[:]))
	if mutate != nil {
		mutate(request)
	}
	recorder := httptest.NewRecorder()
	broker.ShareFileHandler().ServeHTTP(recorder, request)
	reply := shareReply{code: recorder.Code, raw: recorder.Body.String()}
	var envelope struct {
		Error *struct{ Code, Message, Hint string } `json:"error"`
	}
	if json.Unmarshal(recorder.Body.Bytes(), &envelope) == nil && envelope.Error != nil {
		reply.error = *envelope.Error
	}
	return reply
}

func TestShareFileRouteStoresForTheVerifiedSession(t *testing.T) {
	broker, sink, audit := shareHarness(t)
	reply := postShare(t, broker, shareURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", "report é.csv", "a,b\n1\n", nil)
	if reply.code != http.StatusOK {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}
	if len(sink.envelopes) != 1 {
		t.Fatalf("sink got %d shares", len(sink.envelopes))
	}
	got := sink.envelopes[0]
	if got.Kind != agentsession.ShareFileKind || got.Workspace != "tea-a" || got.SessionID != "ags-one" || got.PodName != "sbx-a-0" || got.PodUID != "uid-a" ||
		got.Name != "report é.csv" || got.Size != 6 || sink.bodies[0] != "a,b\n1\n" {
		t.Fatalf("envelope = %+v body = %q", got, sink.bodies[0])
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbShareFile || audit.events[0].Target != "agent-session:ags-one" {
		t.Fatalf("audit = %+v", audit.events)
	}
}

func TestShareFileRouteRefusesForeignPods(t *testing.T) {
	cases := map[string]struct{ target, ip string }{
		"unresolved source pod":      {shareURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.9"},
		"sibling session's pod":      {shareURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.2"},
		"other workspace's pod":      {shareURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.4"},
		"URL claims another session": {shareURL(t, "tea-a-sandbox", "ags-two"), "10.0.0.1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, sink, _ := shareHarness(t)
			reply := postShare(t, broker, tc.target, tc.ip, "a.txt", "x", nil)
			if reply.code != http.StatusForbidden || reply.error.Code != agentsession.ShareFileForbidden {
				t.Fatalf("status=%d body=%s, want 403 forbidden", reply.code, reply.raw)
			}
			if len(sink.envelopes) != 0 {
				t.Fatal("a refused share reached bex-api")
			}
		})
	}
}

func TestShareFileRouteValidatesTheDeclaration(t *testing.T) {
	cases := map[string]struct {
		mutate func(*http.Request)
		code   string
	}{
		"missing digest":     {func(r *http.Request) { r.Header.Del(agentsession.ShareFileSHA256Header) }, agentsession.ShareFileInvalidRequest},
		"missing name":       {func(r *http.Request) { r.Header.Del(agentsession.ShareFileNameHeader) }, agentsession.ShareFileInvalidRequest},
		"bad percent-encode": {func(r *http.Request) { r.Header.Set(agentsession.ShareFileNameHeader, "%zz") }, agentsession.ShareFileInvalidRequest},
		"over the cap": {func(r *http.Request) {
			r.ContentLength = agentsession.MaxShareFileBytes + 1
			r.Header.Set("Content-Length", strconv.Itoa(agentsession.MaxShareFileBytes+1))
		}, agentsession.ShareFileTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, sink, _ := shareHarness(t)
			reply := postShare(t, broker, shareURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", "a.txt", "x", tc.mutate)
			if reply.error.Code != tc.code || len(sink.envelopes) != 0 {
				t.Fatalf("status=%d body=%s, want %s and nothing forwarded", reply.code, reply.raw, tc.code)
			}
		})
	}
}

func TestShareFileRouteRelaysRefusalsAndUnavailability(t *testing.T) {
	broker, sink, _ := shareHarness(t)
	sink.refuse = agentsession.NewShareFileError(agentsession.ShareFileQuotaExceeded, "a session holds at most 50 files")
	reply := postShare(t, broker, shareURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", "a.txt", "x", nil)
	if reply.code != http.StatusConflict || reply.error.Code != agentsession.ShareFileQuotaExceeded || reply.error.Message != "a session holds at most 50 files" {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}

	broker.Files = nil
	reply = postShare(t, broker, shareURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", "a.txt", "x", nil)
	if reply.code != http.StatusServiceUnavailable || reply.error.Code != agentsession.ShareFileUnavailable {
		t.Fatalf("status=%d body=%s, want unavailable without the verb", reply.code, reply.raw)
	}
}
