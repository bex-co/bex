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

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/render-oss/cli/pkg/client"
)

// maxResponseBytes bounds one API answer; a transcript page is the largest.
const maxResponseBytes = 32 << 20

// api is bex-api as the upstream client reaches it: the same base URL, the
// same auth and control-plane headers (request editors), and the same token
// refresh, so `bex agent` authenticates exactly like every other command.
type api struct {
	server string // the API base, ending in /v1/
	http   *http.Client
	edit   []client.RequestEditorFn
	// download fetches signed storage URLs: a plain client, because those URLs
	// carry their own authority and must never see the bex bearer.
	download *http.Client
	sleep    func(context.Context, time.Duration) error
}

func defaultAPI() (*api, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	raw, ok := c.ClientInterface.(*client.Client)
	if !ok {
		return nil, errors.New("unexpected API client")
	}
	return &api{
		server:   raw.Server,
		http:     httpClient(raw.Client),
		edit:     raw.RequestEditors,
		download: &http.Client{Timeout: 10 * time.Minute},
		sleep:    sleepCtx,
	}, nil
}

func httpClient(doer client.HttpRequestDoer) *http.Client {
	if c, ok := doer.(*http.Client); ok {
		return c
	}
	return &http.Client{Transport: doerTransport{doer}}
}

// doerTransport adapts the upstream HttpRequestDoer to a RoundTripper.
type doerTransport struct{ doer client.HttpRequestDoer }

func (t doerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t.doer.Do(r) }

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// apiError is a non-2xx answer in bex's error envelope. RetryAfter is the
// answer's Retry-After, when it named a delay in seconds.
type apiError struct {
	Status     int
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (e *apiError) Error() string {
	text := e.Message
	if text == "" {
		text = http.StatusText(e.Status)
	}
	if e.Code != "" {
		return fmt.Sprintf("%d %s: %s", e.Status, e.Code, text)
	}
	return fmt.Sprintf("%d %s", e.Status, text)
}

// retryable is a rate limit or a gateway hiccup worth repeating an
// idempotent request for.
func (e *apiError) retryable() bool {
	switch e.Status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// retryAfter is how long err asks a retry to wait, at least d.
func retryAfter(err error, d time.Duration) time.Duration {
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > d {
		return apiErr.RetryAfter
	}
	return d
}

func decodeAPIError(status int, body []byte) *apiError {
	var envelope struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Code    string `json:"code"`
	}
	e := &apiError{Status: status}
	if json.Unmarshal(body, &envelope) == nil {
		e.Code = envelope.Code
		e.Message = strings.TrimSpace(envelope.Message)
		if e.Message == "" {
			e.Message = strings.TrimSpace(envelope.Error)
		}
	}
	return e
}

type request struct {
	method      string
	path        []string
	query       url.Values
	body        io.Reader
	length      int64
	contentType string
	header      http.Header
	// stream marks an upload, bounded only by the caller's context rather
	// than by requestTimeout.
	stream bool
}

// requestTimeout bounds one JSON exchange. The upstream client sets none, and
// a stalled poll must not hold a CI job until its own timeout.
const requestTimeout = 2 * time.Minute

// errResponseTooLarge refuses an answer over maxResponseBytes rather than
// decoding a truncated one.
var errResponseTooLarge = fmt.Errorf("the API answer is larger than %d MiB", maxResponseBytes>>20)

func (a *api) do(ctx context.Context, r request) ([]byte, error) {
	if !r.stream {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, requestTimeout)
		defer cancel()
	}
	endpoint, err := url.JoinPath(a.server, r.path...)
	if err != nil {
		return nil, err
	}
	if len(r.query) > 0 {
		endpoint += "?" + r.query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, r.method, endpoint, r.body)
	if err != nil {
		return nil, err
	}
	if r.length > 0 {
		req.ContentLength = r.length
	}
	req.Header.Set("Accept", "application/json")
	if r.contentType != "" {
		req.Header.Set("Content-Type", r.contentType)
	}
	for key, values := range r.header {
		req.Header[key] = values
	}
	for _, edit := range a.edit {
		if err := edit(ctx, req); err != nil {
			return nil, err
		}
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBytes {
		return nil, errResponseTooLarge
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError(resp, body)
	}
	return body, nil
}

// responseError is a non-2xx answer as an *apiError, with its Retry-After.
func responseError(resp *http.Response, body []byte) *apiError {
	e := decodeAPIError(resp.StatusCode, body)
	if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
		e.RetryAfter = time.Duration(seconds) * time.Second
	}
	return e
}

func (a *api) getJSON(ctx context.Context, query url.Values, path ...string) ([]byte, error) {
	return a.do(ctx, request{method: http.MethodGet, path: path, query: query})
}

// post sends r as a POST with payload, when not nil, as its JSON body.
func (a *api) post(ctx context.Context, r request, payload any) ([]byte, error) {
	return a.send(ctx, http.MethodPost, r, payload)
}

// send sends r with method and payload, when not nil, as its JSON body.
func (a *api) send(ctx context.Context, method string, r request, payload any) ([]byte, error) {
	r.method = method
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		r.body, r.contentType = bytes.NewReader(encoded), "application/json"
	}
	return a.do(ctx, r)
}

// session reads one session; raw is the API's answer, verbatim.
func (a *api) session(ctx context.Context, id string) (session, []byte, error) {
	raw, err := a.getJSON(ctx, nil, "agent-sessions", id)
	if err != nil {
		return session{}, nil, fmt.Errorf("get agent session %s: %w", id, err)
	}
	return decodeSession(raw)
}

func decodeSession(raw []byte) (session, []byte, error) {
	var s session
	if err := json.Unmarshal(raw, &s); err != nil {
		return session{}, nil, fmt.Errorf("decode agent session: %w", err)
	}
	return s, raw, nil
}

// postAttempts bounds how often a create is repeated after a transport failure
// or a gateway 5xx. The Idempotency-Key makes the repeat safe: the API answers
// the first session (create, ADR094 D4) instead of making a second.
const postAttempts = 3

// postKeyed posts r, which carries an Idempotency-Key, repeating a transient
// failure with the same request. Only a request the key makes safe to repeat
// goes through it: steer does not (see steer).
func (a *api) postKeyed(ctx context.Context, r request, payload any) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= postAttempts; attempt++ {
		raw, err := a.post(ctx, r, payload)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if !transient(err) || ctx.Err() != nil || attempt == postAttempts {
			break
		}
		if err := a.sleep(ctx, retryAfter(err, time.Duration(attempt)*time.Second)); err != nil {
			break
		}
	}
	return nil, lastErr
}

func (a *api) create(ctx context.Context, owner string, in createRequest) (session, []byte, error) {
	raw, err := a.postKeyed(ctx, request{
		path:   []string{"agent-sessions"},
		query:  url.Values{"ownerId": {owner}},
		header: http.Header{"Idempotency-Key": {in.IdempotencyKey}},
	}, in)
	if err != nil {
		return session{}, nil, fmt.Errorf("create agent session: %w", err)
	}
	return decodeSession(raw)
}

// steer sends a message under key, one per typed line or send. A session whose
// turn is running answers 202 and the message in session.Queued; otherwise it
// starts a turn and answers 200.
//
// It is a single attempt, never repeated. The key makes a repeat safe only
// against a message that is still queued: the server records no key for a send
// that started a turn, so repeating one whose 200 was lost would queue the
// message again, and it would run as a second turn. A transient failure is
// reported instead, and the person decides whether to send again. The key is
// still sent: the server dedupes a message that is still queued against it.
func (a *api) steer(ctx context.Context, id string, in steerRequest, key string) (session, []byte, error) {
	raw, err := a.post(ctx, request{
		path:   []string{"agent-sessions", id, "steer"},
		header: http.Header{"Idempotency-Key": {key}},
	}, in)
	if err != nil {
		return session{}, nil, fmt.Errorf("send to agent session %s: %w", id, steerAdvice(err))
	}
	return decodeSession(raw)
}

// steerWording is the CLI's own text for the steer refusals whose API message
// names a REST field or a dashboard control: it says what to do with this
// command's flags.
var steerWording = map[string]string{
	codeQueueFull: "the session's queue is full; wait for a queued message to run, then send again",
	codeQueueInlineSecrets: "a message with inline secrets cannot wait in the queue, because their values are never stored; " +
		"name workspace secrets with --secret instead of --secret-from-env, or send it once the session is idle",
}

// steerAdvice is err with steerWording's text in place of the API's.
func steerAdvice(err error) error {
	var apiErr *apiError
	if !errors.As(err, &apiErr) || steerWording[apiErr.Code] == "" {
		return err
	}
	advised := *apiErr
	advised.Message = steerWording[apiErr.Code]
	return &advised
}

// queuedMessages lists the messages waiting on the session, next first.
func (a *api) queuedMessages(ctx context.Context, id string) ([]queuedMessage, error) {
	raw, err := a.getJSON(ctx, nil, "agent-sessions", id, "queued-messages")
	if err != nil {
		return nil, fmt.Errorf("list queued messages of agent session %s: %w", id, err)
	}
	var list struct {
		QueuedMessages []queuedMessage `json:"queuedMessages"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("list queued messages of agent session %s: %w", id, err)
	}
	return list.QueuedMessages, nil
}

// interrupt ends the session's running turn and nothing else (ADR047 D12);
// the API answers once the request is delivered, the turn settles after. raw
// is its answer, the session, verbatim.
func (a *api) interrupt(ctx context.Context, id string) ([]byte, error) {
	raw, err := a.post(ctx, request{path: []string{"agent-sessions", id, "interrupt"}}, nil)
	if err != nil {
		return nil, fmt.Errorf("stop agent session %s: %w", id, err)
	}
	return raw, nil
}

func (a *api) cancel(ctx context.Context, id string) (session, []byte, error) {
	raw, err := a.post(ctx, request{path: []string{"agent-sessions", id, "cancel"}}, nil)
	if err != nil {
		return session{}, nil, fmt.Errorf("cancel agent session %s: %w", id, err)
	}
	return decodeSession(raw)
}

// upload streams one file into the workspace's attachments (ADR095 D2) and
// returns its att- id. The API needs Content-Length, so the size is read
// from the file rather than buffering it.
func (a *api) upload(ctx context.Context, owner, path string) (attachment, error) {
	f, err := os.Open(path)
	if err != nil {
		return attachment{}, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return attachment{}, err
	}
	if err := attachable(path, info); err != nil {
		return attachment{}, err
	}
	raw, err := a.do(ctx, request{
		method: http.MethodPost, path: []string{"agent-attachments"},
		query: url.Values{"name": {filepath.Base(path)}, "ownerId": {owner}},
		body:  f, length: info.Size(), contentType: "application/octet-stream", stream: true,
	})
	if err != nil {
		return attachment{}, fmt.Errorf("attach %s: %w", path, err)
	}
	var out attachment
	if err := json.Unmarshal(raw, &out); err != nil {
		return attachment{}, fmt.Errorf("attach %s: %w", path, err)
	}
	return out, nil
}

// attachable refuses what the API cannot take as an attachment.
func attachable(path string, info os.FileInfo) error {
	switch {
	case !info.Mode().IsRegular():
		return fmt.Errorf("attach %s: not a regular file", path)
	case info.Size() == 0:
		return fmt.Errorf("attach %s: the file is empty", path)
	}
	return nil
}

// checkAttachable is attachable for a path not yet opened.
func checkAttachable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return attachable(path, info)
}

// attachmentsPage is the largest page the API serves (Render's 100).
const attachmentsPage = 100

// attachments lists a session's files, every page; direction "" is both.
func (a *api) attachments(ctx context.Context, id, direction string, turn int) ([]attachment, [][]byte, error) {
	var (
		out  []attachment
		raws [][]byte
	)
	cursor := ""
	for {
		query := url.Values{"limit": {strconv.Itoa(attachmentsPage)}}
		if direction != "" {
			query.Set("direction", direction)
		}
		if turn > 0 {
			query.Set("turn", strconv.Itoa(turn))
		}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		raw, err := a.getJSON(ctx, query, "agent-sessions", id, "attachments")
		if err != nil {
			return nil, nil, fmt.Errorf("list files of agent session %s: %w", id, err)
		}
		var page []json.RawMessage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, nil, fmt.Errorf("list files of agent session %s: %w", id, err)
		}
		for _, item := range page {
			var att attachment
			if err := json.Unmarshal(item, &att); err != nil {
				return nil, nil, fmt.Errorf("list files of agent session %s: %w", id, err)
			}
			out = append(out, att)
			raws = append(raws, item)
		}
		if len(page) < attachmentsPage {
			return out, raws, nil
		}
		cursor = out[len(out)-1].ID
	}
}

// transcriptPageSize is the largest page the API serves (default 200).
const transcriptPageSize = 1000

// transcript reads one page after afterSeq (-1: from the start).
func (a *api) transcript(ctx context.Context, id string, afterSeq int64) (transcriptPage, error) {
	query := url.Values{"limit": {strconv.Itoa(transcriptPageSize)}}
	if afterSeq >= 0 {
		query.Set("afterSeq", strconv.FormatInt(afterSeq, 10))
	}
	raw, err := a.getJSON(ctx, query, "agent-sessions", id, "transcript")
	if err != nil {
		return transcriptPage{}, fmt.Errorf("read transcript of agent session %s: %w", id, err)
	}
	var page transcriptPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return transcriptPage{}, fmt.Errorf("read transcript of agent session %s: %w", id, err)
	}
	return page, nil
}

// downloadError is a storage answer other than 200.
type downloadError struct{ status int }

func (e *downloadError) Error() string {
	return fmt.Sprintf("download: %d %s", e.status, http.StatusText(e.status))
}

// fetch downloads a signed storage URL into w.
func (a *api) fetch(ctx context.Context, signed string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
	if err != nil {
		return err
	}
	resp, err := a.download.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return &downloadError{status: resp.StatusCode}
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// drain prints every transcript part after *afterSeq, from fromTurn on, into
// p, advancing *afterSeq, and returns how many parts arrived.
func (a *api) drain(ctx context.Context, id string, afterSeq *int64, fromTurn int, p *partPrinter) (int, error) {
	n := 0
	for {
		page, err := a.transcript(ctx, id, *afterSeq)
		if err != nil {
			return n, err
		}
		p.turns(page.Turns)
		for _, part := range page.Parts {
			if part.Turn >= fromTurn {
				p.add(part)
			}
		}
		n += len(page.Parts)
		if len(page.Parts) == 0 || page.NextAfterSeq <= *afterSeq {
			return n, nil
		}
		*afterSeq = page.NextAfterSeq
	}
}
