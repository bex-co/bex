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
)

type recordingResults struct {
	envelopes []ResultEnvelope
	refuse    error
}

func (s *recordingResults) SubmitResult(_ context.Context, e ResultEnvelope) (ResultResponse, error) {
	s.envelopes = append(s.envelopes, e)
	if s.refuse != nil {
		return ResultResponse{}, s.refuse
	}
	return ResultResponse{Accepted: true, Final: e.Final}, nil
}

func validResultEnvelope() ResultEnvelope {
	return ResultEnvelope{Kind: ResultKind, Workspace: "tea-a", SessionID: "ags-one", PodName: "sbx-1-0", PodUID: "uid-1",
		Result: json.RawMessage(`{"ok":true}`), Final: true, Nonce: "nonce-1"}
}

func signedResult(t *testing.T, secret []byte, raw []byte, at time.Time, tamper func(*http.Request)) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, InternalResultPath, bytes.NewReader(raw))
	signHeaders(request.Header, secret, resultMACInput(raw), at)
	if tamper != nil {
		tamper(request)
	}
	return request
}

func resultHandler(secret []byte, sink ResultSink, now time.Time) *ResultHandler {
	return &ResultHandler{Secret: secret, Results: sink, Nonce: &fakeNonceClaimer{}, Now: func() time.Time { return now }}
}

func TestResultHandlerAuthenticatesTheEnvelope(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	raw, _ := json.Marshal(validResultEnvelope())
	cases := map[string]struct {
		at     time.Time
		tamper func(*http.Request)
		want   int
	}{
		"valid":           {now, nil, http.StatusOK},
		"stale timestamp": {now.Add(-2 * time.Minute), nil, http.StatusUnauthorized},
		"wrong secret":    {now, func(r *http.Request) { r.Header.Set(SignatureHeader, strings.Repeat("0", 64)) }, http.StatusUnauthorized},
		"swapped body": {now, func(r *http.Request) {
			other := validResultEnvelope()
			other.SessionID = "ags-two"
			swapped, _ := json.Marshal(other)
			r.Body = http.NoBody
			*r = *r.Clone(r.Context())
			r.Body = nopCloser{bytes.NewReader(swapped)}
		}, http.StatusUnauthorized},
		// The bare-body MAC the other verbs use does not verify here.
		"undomained MAC": {now, func(r *http.Request) { signHeaders(r.Header, []byte("s"), raw, now) }, http.StatusUnauthorized},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sink := &recordingResults{}
			recorder := httptest.NewRecorder()
			resultHandler(secret, sink, now).ServeHTTP(recorder, signedResult(t, secret, raw, tc.at, tc.tamper))
			if recorder.Code != tc.want || (tc.want == http.StatusOK) != (len(sink.envelopes) == 1) {
				t.Fatalf("status=%d body=%s sink=%d, want %d", recorder.Code, recorder.Body, len(sink.envelopes), tc.want)
			}
		})
	}
}

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }

// A correctly signed request that is not exactly a result envelope is refused
// before the sink: the kind, the gateway-verified Pod, and no field another
// verb might carry.
func TestResultHandlerDecodesOnlyAResultEnvelope(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	for name, raw := range map[string]string{
		"no kind":       `{"workspace":"tea-a","sessionId":"ags-one","podName":"p","podUid":"u","result":1,"final":true,"nonce":"n"}`,
		"share kind":    `{"kind":"share-file","workspace":"tea-a","sessionId":"ags-one","podName":"p","podUid":"u","result":1,"final":true,"nonce":"n"}`,
		"unknown field": `{"kind":"submit-result","cause":"x","workspace":"tea-a","sessionId":"ags-one","podName":"p","podUid":"u","result":1,"final":true,"nonce":"n"}`,
		"no pod":        `{"kind":"submit-result","workspace":"tea-a","sessionId":"ags-one","result":1,"final":true,"nonce":"n"}`,
	} {
		t.Run(name, func(t *testing.T) {
			sink := &recordingResults{}
			recorder := httptest.NewRecorder()
			resultHandler(secret, sink, now).ServeHTTP(recorder, signedResult(t, secret, []byte(raw), now, nil))
			if recorder.Code != http.StatusBadRequest || len(sink.envelopes) != 0 {
				t.Fatalf("status=%d sink=%d, want 400 before the sink", recorder.Code, len(sink.envelopes))
			}
		})
	}
}

// A result envelope can never be replayed against the model mint, and the
// mint-shaped verbs refuse any body naming a verb kind.
func TestAResultEnvelopeCannotMintTheModelKey(t *testing.T) {
	secret := []byte("s")
	now := time.Now()
	raw, _ := json.Marshal(validResultEnvelope())
	minter := &ModelMinter{Keys: modelKeysFor("sk-ant-api03-REAL"), Sessions: fakeSessions{session: modelSession("https://api.anthropic.com/v1")}, Audit: &auditRecorder{}}
	handler := &ModelHandler{Secret: secret, Minter: minter, Now: func() time.Time { return now }}
	for name, signed := range map[string][]byte{"replayed result signature": resultMACInput(raw), "bare-body signature (kind)": raw} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, InternalModelMintPath, bytes.NewReader(raw))
			signHeaders(request.Header, secret, signed, now)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusOK || strings.Contains(recorder.Body.String(), "sk-ant") {
				t.Fatalf("status=%d body=%s: a result envelope minted a model credential", recorder.Code, recorder.Body)
			}
		})
	}
}

func TestResultHandlerRefusesAReplayedEnvelope(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	raw, _ := json.Marshal(validResultEnvelope())
	sink := &recordingResults{}
	handler := resultHandler(secret, sink, now)
	for i, want := range []int{http.StatusOK, http.StatusUnauthorized} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, signedResult(t, secret, raw, now, nil))
		if recorder.Code != want {
			t.Fatalf("attempt %d: status=%d, want %d", i, recorder.Code, want)
		}
	}
	if len(sink.envelopes) != 1 {
		t.Fatalf("a replay reached the sink: %d", len(sink.envelopes))
	}
	unavailable := httptest.NewRecorder()
	(&ResultHandler{Secret: secret, Results: sink, Now: func() time.Time { return now }}).ServeHTTP(unavailable, signedResult(t, secret, raw, now, nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("without a nonce store: status=%d, want unavailable", unavailable.Code)
	}
}

func TestResultHandlerValidatesBeforeTheSinkAndRelaysItsRefusals(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_800_000_000, 0)
	for name, mutate := range map[string]func(*ResultEnvelope){
		"oversize":      func(e *ResultEnvelope) { e.Result = json.RawMessage(`"` + strings.Repeat("a", MaxResultBytes) + `"`) },
		"NUL character": func(e *ResultEnvelope) { e.Result = json.RawMessage(`"a\u0000b"`) },
	} {
		t.Run(name, func(t *testing.T) {
			envelope := validResultEnvelope()
			mutate(&envelope)
			raw, _ := json.Marshal(envelope)
			sink := &recordingResults{}
			recorder := httptest.NewRecorder()
			resultHandler(secret, sink, now).ServeHTTP(recorder, signedResult(t, secret, raw, now, nil))
			if recorder.Code < 400 || len(sink.envelopes) != 0 {
				t.Fatalf("status=%d sink=%d, want a refusal before the sink", recorder.Code, len(sink.envelopes))
			}
		})
	}

	raw, _ := json.Marshal(validResultEnvelope())
	sink := &recordingResults{refuse: NewResultError(ResultSchemaMismatch, "/x: wrong")}
	recorder := httptest.NewRecorder()
	resultHandler(secret, sink, now).ServeHTTP(recorder, signedResult(t, secret, raw, now, nil))
	var body struct {
		Error ResultError `json:"error"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	if recorder.Code != http.StatusUnprocessableEntity || body.Error.Code != ResultSchemaMismatch || body.Error.Message != "/x: wrong" || body.Error.Hint == "" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body)
	}
}

func TestResultClientRoundTripsAndRelaysOnlyItsOwnVocabulary(t *testing.T) {
	secret := []byte("s")
	sink := &recordingResults{}
	api := httptest.NewServer(&ResultHandler{Secret: secret, Results: sink, Nonce: &fakeNonceClaimer{}})
	defer api.Close()
	client := &ResultClient{URL: api.URL, Secret: secret, HTTP: api.Client()}

	envelope := validResultEnvelope()
	envelope.Kind, envelope.Nonce = "", ""
	got, err := client.Submit(context.Background(), envelope)
	if err != nil || !got.Accepted || !got.Final || len(sink.envelopes) != 1 || sink.envelopes[0].Nonce == "" || sink.envelopes[0].Kind != ResultKind {
		t.Fatalf("submit = %+v err=%v sink=%+v", got, err, sink.envelopes)
	}

	sink.refuse = NewResultError(ResultNotExpected, "this turn has no outputSchema")
	if _, err := client.Submit(context.Background(), envelope); err == nil || err.(*ResultError).Code != ResultNotExpected || err.(*ResultError).Status != http.StatusConflict {
		t.Fatalf("refusal = %v, want not_expected re-derived with its own status", err)
	}
	wrong := &ResultClient{URL: api.URL, Secret: []byte("other"), HTTP: api.Client()}
	if _, err := wrong.Submit(context.Background(), envelope); err == nil || err.(*ResultError).Code != ResultForbidden {
		t.Fatalf("bad signature = %v, want forbidden", err)
	}
	api.Close()
	if _, err := client.Submit(context.Background(), envelope); err == nil || err.(*ResultError).Code != ResultUnavailable {
		t.Fatalf("unreachable = %v, want unavailable", err)
	}
}

func TestDecodeResultContentIsStrict(t *testing.T) {
	if c, err := DecodeResultContent([]byte(`{"result":{"a":1},"final":false}`)); err != nil || c.Final || string(c.Result) != `{"a":1}` {
		t.Fatalf("valid = %+v %v", c, err)
	}
	if c, err := DecodeResultContent([]byte(`{"result":null,"final":true}`)); err != nil || string(c.Result) != "null" {
		t.Fatalf("a null result is a value the schema may allow: %+v %v", c, err)
	}
	for name, body := range map[string]string{
		"unknown field": `{"result":1,"final":true,"session":"ags-other"}`,
		"no result":     `{"final":true}`,
		"not an object": `[1]`,
		"trailing":      `{"result":1,"final":true} {"result":2}`,
		"empty":         ``,
	} {
		if _, err := DecodeResultContent([]byte(body)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := DecodeResultContent([]byte(`{"result":"` + strings.Repeat("a", MaxResultBytes) + `","final":true}`)); err == nil || err.Code != ResultTooLarge {
		t.Errorf("oversize = %v, want too_large", err)
	}
}

func TestResultEnvelopeFitsItsCap(t *testing.T) {
	envelope := validResultEnvelope()
	envelope.Workspace, envelope.SessionID = strings.Repeat("w", 64), strings.Repeat("s", 64)
	envelope.PodName, envelope.PodUID, envelope.Nonce = strings.Repeat("p", 253), strings.Repeat("u", 64), strings.Repeat("n", 64)
	envelope.Result = json.RawMessage(`"` + strings.Repeat("a", MaxResultBytes-2) + `"`)
	raw, _ := json.Marshal(envelope)
	if len(raw) > maxResultEnvelopeBytes {
		t.Fatalf("worst-case envelope is %d bytes, cap %d", len(raw), maxResultEnvelopeBytes)
	}
	body, _ := json.Marshal(ResultContent{Result: envelope.Result, Final: true})
	if len(body) > MaxResultRequestBytes {
		t.Fatalf("worst-case sandbox request is %d bytes, cap %d", len(body), MaxResultRequestBytes)
	}
}

func TestResultURLRoundTrips(t *testing.T) {
	u, err := ResultURL("http://gw:8082/", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	ns, sid, err := ParseResultPath(strings.TrimPrefix(u, "http://gw:8082"))
	if err != nil || ns != "tea-a-sandbox" || sid != "ags-one" {
		t.Fatalf("parse = %q %q %v", ns, sid, err)
	}
	for _, bad := range []string{"/agent-results/", "/agent-results/a", "/agent-results/YQ/Yg/extra", "/agent-attachments/YQ/Yg", "/agent-results/!!/b"} {
		if _, _, err := ParseResultPath(bad); err == nil {
			t.Errorf("ParseResultPath(%q) accepted", bad)
		}
	}
}
