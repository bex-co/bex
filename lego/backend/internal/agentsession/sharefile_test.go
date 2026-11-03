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
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type countingSink struct{ calls int }

func (s *countingSink) ShareFile(_ context.Context, e ShareFileEnvelope, body io.Reader) (ShareFileResponse, error) {
	s.calls++
	_, _ = io.Copy(io.Discard, body)
	return ShareFileResponse{AttachmentID: "att-x", Name: e.Name, Size: e.Size}, nil
}

// validEnvelope names the session modelSession describes, so the same bytes
// are a plausible model mint request too.
func validEnvelope() ShareFileEnvelope {
	return ShareFileEnvelope{Kind: ShareFileKind, Workspace: "tea-a", SessionID: "ags-one",
		PodName: "sbx-1-0", PodUID: "uid-1", Name: "a.txt", Size: 1,
		SHA256: strings.Repeat("ab", 32), Nonce: "nonce-1"}
}

// signedShare builds the gateway's request by hand, signed in the share MAC
// domain, so each test can corrupt exactly one part of it.
func signedShare(t *testing.T, secret []byte, envelope ShareFileEnvelope, at time.Time, tamper func(*http.Request)) *http.Request {
	t.Helper()
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, InternalShareFilePath, strings.NewReader("x"))
	request.Header.Set(ShareFileEnvelopeHeader, base64.RawURLEncoding.EncodeToString(raw))
	signHeaders(request.Header, secret, shareFileMACInput(raw), at)
	if tamper != nil {
		tamper(request)
	}
	return request
}

func shareHandler(secret []byte, sink ShareFileSink, now time.Time) *ShareFileHandler {
	return &ShareFileHandler{Secret: secret, Files: sink, Nonce: &fakeNonceClaimer{}, Now: func() time.Time { return now }}
}

func TestShareFileHandlerAuthenticatesTheEnvelope(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	other := validEnvelope()
	other.SessionID = "ags-two"
	otherRaw, _ := json.Marshal(other)
	cases := map[string]struct {
		at     time.Time
		tamper func(*http.Request)
		want   int
	}{
		"valid":           {now, nil, http.StatusOK},
		"stale timestamp": {now.Add(-2 * time.Minute), nil, http.StatusUnauthorized},
		"wrong secret":    {now, func(r *http.Request) { r.Header.Set(SignatureHeader, strings.Repeat("0", 64)) }, http.StatusUnauthorized},
		"no envelope":     {now, func(r *http.Request) { r.Header.Del(ShareFileEnvelopeHeader) }, http.StatusBadRequest},
		"swapped envelope": {now, func(r *http.Request) {
			r.Header.Set(ShareFileEnvelopeHeader, base64.RawURLEncoding.EncodeToString(otherRaw))
		}, http.StatusUnauthorized},
		// The bare-body MAC the other verbs use does not verify here.
		"undomained MAC": {now, func(r *http.Request) {
			raw, _ := json.Marshal(validEnvelope())
			signHeaders(r.Header, []byte("s"), raw, now)
		}, http.StatusUnauthorized},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sink := &countingSink{}
			recorder := httptest.NewRecorder()
			shareHandler(secret, sink, now).ServeHTTP(recorder, signedShare(t, secret, validEnvelope(), tc.at, tc.tamper))
			if recorder.Code != tc.want {
				t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body, tc.want)
			}
			if (tc.want == http.StatusOK) != (sink.calls == 1) {
				t.Fatalf("sink calls = %d", sink.calls)
			}
		})
	}
}

// A correctly signed request that is not exactly a share envelope is refused:
// the kind, the gateway-verified Pod, and no field another verb might carry.
func TestShareFileHandlerDecodesOnlyAShareEnvelope(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	for name, raw := range map[string]string{
		"no kind":       `{"workspace":"tea-a","sessionId":"ags-one","podName":"sbx-1-0","podUid":"u","name":"a","size":1,"sha256":"` + strings.Repeat("ab", 32) + `","nonce":"n"}`,
		"other kind":    `{"kind":"model","workspace":"tea-a","sessionId":"ags-one","podName":"sbx-1-0","podUid":"u","name":"a","size":1,"sha256":"` + strings.Repeat("ab", 32) + `","nonce":"n"}`,
		"unknown field": `{"kind":"share-file","cause":"x","workspace":"tea-a","sessionId":"ags-one","podName":"sbx-1-0","podUid":"u","name":"a","size":1,"sha256":"` + strings.Repeat("ab", 32) + `","nonce":"n"}`,
		"no pod":        `{"kind":"share-file","workspace":"tea-a","sessionId":"ags-one","name":"a","size":1,"sha256":"` + strings.Repeat("ab", 32) + `","nonce":"n"}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, InternalShareFilePath, strings.NewReader("x"))
			request.Header.Set(ShareFileEnvelopeHeader, base64.RawURLEncoding.EncodeToString([]byte(raw)))
			signHeaders(request.Header, secret, shareFileMACInput([]byte(raw)), now)
			sink := &countingSink{}
			recorder := httptest.NewRecorder()
			shareHandler(secret, sink, now).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest || sink.calls != 0 {
				t.Fatalf("status=%d calls=%d, want 400 before storage", recorder.Code, sink.calls)
			}
		})
	}
}

// A captured share envelope carries every model mint field, but can never mint
// the workspace's model key: its MAC is domain-separated, and even a request
// signed over the bare envelope is refused for carrying a verb kind.
func TestAShareEnvelopeCannotMintTheModelKey(t *testing.T) {
	secret := []byte("s")
	now := time.Now()
	raw, err := json.Marshal(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	minter := &ModelMinter{Keys: modelKeysFor("sk-ant-api03-REAL"), Sessions: fakeSessions{session: modelSession("https://api.anthropic.com/v1")}, Audit: &auditRecorder{}}
	handler := &ModelHandler{Secret: secret, Minter: minter, Now: func() time.Time { return now }}
	for name, signed := range map[string][]byte{
		"replayed share signature":   shareFileMACInput(raw),
		"bare-body signature (kind)": raw,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, InternalModelMintPath, bytes.NewReader(raw))
			signHeaders(request.Header, secret, signed, now)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusOK || strings.Contains(recorder.Body.String(), "sk-ant") {
				t.Fatalf("status=%d body=%s: a share envelope minted a model credential", recorder.Code, recorder.Body)
			}
		})
	}
}

func TestShareFileHandlerRefusesAReplayedEnvelope(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	sink := &countingSink{}
	handler := shareHandler(secret, sink, now)
	for i, want := range []int{http.StatusOK, http.StatusUnauthorized} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, signedShare(t, secret, validEnvelope(), now, nil))
		if recorder.Code != want {
			t.Fatalf("attempt %d: status=%d, want %d", i, recorder.Code, want)
		}
	}
	if sink.calls != 1 {
		t.Fatalf("a replay reached the sink: %d calls", sink.calls)
	}
}

func TestShareFileHandlerIsUnavailableWithoutANonceStore(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	sink := &countingSink{}
	handler := &ShareFileHandler{Secret: secret, Files: sink, Now: func() time.Time { return now }}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, signedShare(t, secret, validEnvelope(), now, nil))
	if recorder.Code != http.StatusServiceUnavailable || sink.calls != 0 {
		t.Fatalf("status=%d calls=%d, want unavailable", recorder.Code, sink.calls)
	}
}

func TestShareFileHandlerValidatesBeforeStoring(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	for name, mutate := range map[string]func(*ShareFileEnvelope){
		"oversize":   func(e *ShareFileEnvelope) { e.Size = MaxShareFileBytes + 1 },
		"bad digest": func(e *ShareFileEnvelope) { e.SHA256 = "nope" },
		"empty":      func(e *ShareFileEnvelope) { e.Size = 0 },
		"long name":  func(e *ShareFileEnvelope) { e.Name = strings.Repeat("a", MaxShareFileNameBytes+1) },
	} {
		t.Run(name, func(t *testing.T) {
			envelope := validEnvelope()
			mutate(&envelope)
			sink := &countingSink{}
			recorder := httptest.NewRecorder()
			shareHandler(secret, sink, now).ServeHTTP(recorder, signedShare(t, secret, envelope, now, nil))
			if recorder.Code < 400 || sink.calls != 0 {
				t.Fatalf("status=%d sink calls=%d, want a refusal before storage", recorder.Code, sink.calls)
			}
		})
	}
}

// The worst-case envelope — a maximal name of control bytes, each escaped to
// \u00XX, and maximal ids — still fits the handler's cap.
func TestShareFileEnvelopeFitsItsCap(t *testing.T) {
	envelope := validEnvelope()
	envelope.Name = strings.Repeat("\x01", MaxShareFileNameBytes)
	envelope.Workspace, envelope.SessionID = strings.Repeat("w", 64), strings.Repeat("s", 64)
	envelope.PodName, envelope.PodUID = strings.Repeat("p", 253), strings.Repeat("u", 64)
	envelope.Nonce = strings.Repeat("n", 64)
	envelope.Size = MaxShareFileBytes
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > maxShareFileEnvelopeBytes {
		t.Fatalf("worst-case envelope is %d bytes, cap %d", len(raw), maxShareFileEnvelopeBytes)
	}
}

func TestShareFileURLRoundTrips(t *testing.T) {
	u, err := ShareFileURL("http://gw:8082/", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimPrefix(u, "http://gw:8082")
	ns, sid, err := ParseShareFilePath(path)
	if err != nil || ns != "tea-a-sandbox" || sid != "ags-one" {
		t.Fatalf("parse(%q) = %q %q %v", path, ns, sid, err)
	}
	for _, bad := range []string{"/agent-attachments/", "/agent-attachments/a", path + "/", path + "/x", "/git/a/b", "/agent-attachments/!!/b"} {
		if _, _, err := ParseShareFilePath(bad); err == nil {
			t.Errorf("ParseShareFilePath(%q) accepted", bad)
		}
	}
}
