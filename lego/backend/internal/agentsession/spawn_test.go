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

package agentsession

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

type recordingSpawns struct{ creates, reads []SpawnEnvelope }

func (s *recordingSpawns) SpawnSession(_ context.Context, e SpawnEnvelope) (SpawnResponse, error) {
	s.creates = append(s.creates, e)
	return SpawnResponse{SessionID: "ags-child", State: "working", Created: true}, nil
}

func (s *recordingSpawns) ReadChildSession(_ context.Context, e SpawnEnvelope) (ChildView, error) {
	s.reads = append(s.reads, e)
	return ChildView{SessionID: e.ChildID, State: "working"}, nil
}

func (s *recordingSpawns) MessageChildSession(_ context.Context, e SpawnEnvelope) (ChildView, error) {
	s.reads = append(s.reads, e)
	return ChildView{SessionID: e.ChildID, State: "working"}, nil
}

func (s *recordingSpawns) CancelChildSession(_ context.Context, e SpawnEnvelope) (ChildView, error) {
	s.reads = append(s.reads, e)
	return ChildView{SessionID: e.ChildID, State: "canceled"}, nil
}

func (s *recordingSpawns) WaitChildSessions(_ context.Context, e SpawnEnvelope) (SpawnWaitResponse, error) {
	s.reads = append(s.reads, e)
	return SpawnWaitResponse{Settled: []string{}}, nil
}

func validSpawnEnvelope() SpawnEnvelope {
	return SpawnEnvelope{Kind: SpawnKind, Op: SpawnOpCreate, Workspace: "tea-a", SessionID: "ags-one", PodName: "sbx-1-0", PodUID: "uid-1",
		Spawn: &SpawnContent{Task: "t"}, Nonce: "nonce-1"}
}

func serveSpawn(t *testing.T, sink SpawnSink, raw []byte, now time.Time, sign func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, InternalSpawnPath, bytes.NewReader(raw))
	signHeaders(request.Header, []byte("s"), spawnMACInput(raw), now)
	if sign != nil {
		sign(request)
	}
	recorder := httptest.NewRecorder()
	(&SpawnHandler{Secret: []byte("s"), Spawns: sink, Nonce: &fakeNonceClaimer{}, Now: func() time.Time { return now }}).ServeHTTP(recorder, request)
	return recorder
}

func TestSpawnHandlerAuthenticatesAndDomainSeparatesTheEnvelope(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	raw, _ := json.Marshal(validSpawnEnvelope())
	for name, tc := range map[string]struct {
		sign func(*http.Request)
		want int
	}{
		"valid":          {nil, http.StatusOK},
		"wrong secret":   {func(r *http.Request) { r.Header.Set(SignatureHeader, strings.Repeat("0", 64)) }, http.StatusUnauthorized},
		"undomained MAC": {func(r *http.Request) { signHeaders(r.Header, []byte("s"), raw, now) }, http.StatusUnauthorized},
		// A result envelope's signature is another verb's domain.
		"result domain": {func(r *http.Request) { signHeaders(r.Header, []byte("s"), resultMACInput(raw), now) }, http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			sink := &recordingSpawns{}
			recorder := serveSpawn(t, sink, raw, now, tc.sign)
			if recorder.Code != tc.want || (tc.want == http.StatusOK) != (len(sink.creates) == 1) {
				t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body, tc.want)
			}
		})
	}
}

// A correctly signed request that is not exactly a well-formed envelope is
// refused before the sink: the kind, the verified Pod, and the payload its
// operation takes (a create without content, or a read that also carries
// content, is not something the gateway sends).
func TestSpawnHandlerDecodesOnlyAWellFormedEnvelope(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	childID := id.New(id.AgentSession)
	for name, raw := range map[string]string{
		"no kind":          `{"op":"create","workspace":"w","sessionId":"s","podName":"p","podUid":"u","spawn":{"task":"t"},"nonce":"n"}`,
		"result kind":      `{"kind":"submit-result","op":"create","workspace":"w","sessionId":"s","podName":"p","podUid":"u","spawn":{"task":"t"},"nonce":"n"}`,
		"unknown field":    `{"kind":"spawn-session","op":"create","parentSessionId":"x","workspace":"w","sessionId":"s","podName":"p","podUid":"u","spawn":{"task":"t"},"nonce":"n"}`,
		"unknown in spawn": `{"kind":"spawn-session","op":"create","workspace":"w","sessionId":"s","podName":"p","podUid":"u","spawn":{"task":"t","parentSessionId":"x"},"nonce":"n"}`,
		"no pod":           `{"kind":"spawn-session","op":"create","workspace":"w","sessionId":"s","spawn":{"task":"t"},"nonce":"n"}`,
		"create no spawn":  `{"kind":"spawn-session","op":"create","workspace":"w","sessionId":"s","podName":"p","podUid":"u","nonce":"n"}`,
		"create with id":   `{"kind":"spawn-session","op":"create","workspace":"w","sessionId":"s","podName":"p","podUid":"u","spawn":{"task":"t"},"childId":"` + childID + `","nonce":"n"}`,
		"read no id":       `{"kind":"spawn-session","op":"read","workspace":"w","sessionId":"s","podName":"p","podUid":"u","nonce":"n"}`,
		"read with spawn":  `{"kind":"spawn-session","op":"read","workspace":"w","sessionId":"s","podName":"p","podUid":"u","childId":"` + childID + `","spawn":{"task":"t"},"nonce":"n"}`,
		"unknown op":       `{"kind":"spawn-session","op":"delete","workspace":"w","sessionId":"s","podName":"p","podUid":"u","childId":"` + childID + `","nonce":"n"}`,
	} {
		t.Run(name, func(t *testing.T) {
			sink := &recordingSpawns{}
			recorder := serveSpawn(t, sink, []byte(raw), now, nil)
			if recorder.Code != http.StatusBadRequest || len(sink.creates)+len(sink.reads) != 0 {
				t.Fatalf("status=%d, want 400 before the sink", recorder.Code)
			}
		})
	}
	// A replayed nonce is refused.
	raw, _ := json.Marshal(validSpawnEnvelope())
	handler := &SpawnHandler{Secret: []byte("s"), Spawns: &recordingSpawns{}, Nonce: &fakeNonceClaimer{}, Now: func() time.Time { return now }}
	for i, want := range []int{http.StatusOK, http.StatusUnauthorized} {
		request := httptest.NewRequest(http.MethodPost, InternalSpawnPath, bytes.NewReader(raw))
		signHeaders(request.Header, []byte("s"), spawnMACInput(raw), now)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Fatalf("delivery %d = %d, want %d", i, recorder.Code, want)
		}
	}
}

func TestSpawnClientRoundTripKeepsNamedRefusals(t *testing.T) {
	sink := &refusingSpawns{err: NewSpawnError("AGENT_SESSION_CHILD_LIMIT", "a session can have at most 5 live children")}
	server := httptest.NewServer(&SpawnHandler{Secret: []byte("s"), Spawns: sink, Nonce: &fakeNonceClaimer{}})
	defer server.Close()
	client := &SpawnClient{URL: server.URL, Secret: []byte("s"), HTTP: server.Client()}
	_, err := client.Spawn(context.Background(), SpawnEnvelope{Workspace: "w", SessionID: "s", PodName: "p", PodUID: "u", Spawn: &SpawnContent{Task: "t"}})
	var refusal *SpawnError
	if !asSpawnError(err, &refusal) || refusal.Code != "AGENT_SESSION_CHILD_LIMIT" || refusal.Message != "a session can have at most 5 live children" || refusal.Hint == "" {
		t.Fatalf("err = %#v", err)
	}
	wrong := &SpawnClient{URL: server.URL, Secret: []byte("other"), HTTP: server.Client()}
	if _, err := wrong.Spawn(context.Background(), SpawnEnvelope{Workspace: "w", SessionID: "s", PodName: "p", PodUID: "u", Spawn: &SpawnContent{Task: "t"}}); !asSpawnError(err, &refusal) || refusal.Code != SpawnForbidden {
		t.Fatalf("a wrong secret = %#v, want forbidden", err)
	}
}

type refusingSpawns struct{ err error }

func (s *refusingSpawns) SpawnSession(context.Context, SpawnEnvelope) (SpawnResponse, error) {
	return SpawnResponse{}, s.err
}
func (s *refusingSpawns) ReadChildSession(context.Context, SpawnEnvelope) (ChildView, error) {
	return ChildView{}, s.err
}
func (s *refusingSpawns) MessageChildSession(context.Context, SpawnEnvelope) (ChildView, error) {
	return ChildView{}, s.err
}
func (s *refusingSpawns) CancelChildSession(context.Context, SpawnEnvelope) (ChildView, error) {
	return ChildView{}, s.err
}
func (s *refusingSpawns) WaitChildSessions(context.Context, SpawnEnvelope) (SpawnWaitResponse, error) {
	return SpawnWaitResponse{}, s.err
}

func asSpawnError(err error, target **SpawnError) bool {
	e, ok := err.(*SpawnError)
	if ok {
		*target = e
	}
	return ok
}

func TestParseSpawnPathAndContent(t *testing.T) {
	u, err := SpawnURL("http://gw:8082/", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	path := u[strings.Index(u, SpawnProxyPath):]
	ns, sid, child, err := ParseSpawnPath(path)
	if err != nil || ns != "tea-a-sandbox" || sid != "ags-one" || child != "" {
		t.Fatalf("parse = %q %q %q %v", ns, sid, child, err)
	}
	childID := id.New(id.AgentSession)
	if _, _, got, err := ParseSpawnPath(path + "/" + childID); err != nil || got != childID {
		t.Fatalf("parse with child = %q %v", got, err)
	}
	for _, bad := range []string{path + "/", path + "/x", path + "/" + childID + "/more", SpawnProxyPath + "!!/!!"} {
		if _, _, _, err := ParseSpawnPath(bad); err == nil {
			t.Fatalf("%q parsed", bad)
		}
	}
	// secretNames absent and empty stay different through a decode.
	absent, _ := DecodeSpawnContent([]byte(`{"task":"t"}`))
	empty, _ := DecodeSpawnContent([]byte(`{"task":"t","secretNames":[]}`))
	if absent.SecretNames != nil || empty.SecretNames == nil || len(*empty.SecretNames) != 0 {
		t.Fatalf("absent=%v empty=%v", absent.SecretNames, empty.SecretNames)
	}
}

// Each coordinator op carries exactly its own payload; one op cannot be sent
// with another's (w9/m346).
func TestSpawnEnvelopeCoordinatorOps(t *testing.T) {
	base := validSpawnEnvelope()
	base.Spawn = nil
	child := "ags-c185th5c2rvvnhbfiltg"
	cases := []struct {
		name  string
		edit  func(*SpawnEnvelope)
		valid bool
	}{
		{"message", func(e *SpawnEnvelope) { e.Op, e.ChildID, e.Message = SpawnOpMessage, child, &SpawnMessage{Prompt: "p"} }, true},
		{"message without a child", func(e *SpawnEnvelope) { e.Op, e.Message = SpawnOpMessage, &SpawnMessage{Prompt: "p"} }, false},
		{"cancel", func(e *SpawnEnvelope) { e.Op, e.ChildID = SpawnOpCancel, child }, true},
		{"cancel carrying a message", func(e *SpawnEnvelope) { e.Op, e.ChildID, e.Message = SpawnOpCancel, child, &SpawnMessage{Prompt: "p"} }, false},
		{"wait", func(e *SpawnEnvelope) { e.Op, e.Wait = SpawnOpWait, &SpawnWait{ChildIDs: []string{child}} }, true},
		{"wait naming a child", func(e *SpawnEnvelope) {
			e.Op, e.ChildID, e.Wait = SpawnOpWait, child, &SpawnWait{ChildIDs: []string{child}}
		}, false},
		{"read carrying a spawn", func(e *SpawnEnvelope) { e.Op, e.ChildID, e.Spawn = SpawnOpRead, child, &SpawnContent{Task: "t"} }, false},
	}
	for _, tc := range cases {
		e := base
		tc.edit(&e)
		if got := e.valid(); got != tc.valid {
			t.Errorf("%s: valid=%v, want %v", tc.name, got, tc.valid)
		}
	}
	for timeout, ok := range map[int]bool{0: true, 1: true, MaxSpawnWaitSeconds: true, MaxSpawnWaitSeconds + 1: false, -1: false} {
		wait := SpawnWait{ChildIDs: []string{child}, TimeoutSeconds: timeout}
		if err := wait.Validate(); (err == nil) != ok || wait.TimeoutSeconds != timeout {
			t.Errorf("wait timeout %d: err=%v, want ok=%v and the value unchanged", timeout, err, ok)
		}
	}
}
