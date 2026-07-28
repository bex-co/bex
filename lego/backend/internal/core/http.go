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

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

type strictJSONDecodingKey struct{}

// WithStrictJSONDecoding marks a request whose JSON body must reject unknown
// fields and trailing values. The Render OpenAPI gate sets it only for the
// bex∩Render route intersection; bex-native extension routes retain their
// existing decoding behavior unless they opt in separately.
func WithStrictJSONDecoding(ctx context.Context) context.Context {
	return context.WithValue(ctx, strictJSONDecodingKey{}, true)
}

func strictJSONDecoding(ctx context.Context) bool {
	strict, _ := ctx.Value(strictJSONDecodingKey{}).(bool)
	return strict
}

var safeJSONField = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)

// DecodeJSON preserves encoding/json's historical one-value behavior for
// ordinary requests. A request marked by WithStrictJSONDecoding additionally
// rejects unknown struct fields and any second JSON value.
func DecodeJSON(r *http.Request, dst any) error {
	return decodeJSON(r.Context(), r.Body, dst)
}

func decodeJSON(ctx context.Context, reader io.Reader, dst any) error {
	if !strictJSONDecoding(ctx) {
		return json.NewDecoder(reader).Decode(dst)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return errors.New("invalid JSON request body")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(dst); err != nil {
		return safeJSONDecodeError(err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON value")
	}
	// A type with UnmarshalJSON controls its own nested decode, so the outer
	// decoder's DisallowUnknownFields cannot see unknown keys inside it. Walk the
	// already-validated JSON shape against the Go destination as a second guard;
	// maps and RawMessage remain deliberately open, and known RawMessage fields
	// are decoded again by their owning handler.
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("invalid JSON request body")
	}
	return rejectUnknownJSONFields(value, reflect.TypeOf(dst))
}

func rejectUnknownJSONFields(value any, target reflect.Type) error {
	for target != nil && target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	if target == nil {
		return nil
	}
	switch target.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		fields := jsonFieldTypes(target)
		for name, child := range object {
			fieldType, ok := fields[name]
			if !ok {
				for candidate, candidateType := range fields {
					if strings.EqualFold(candidate, name) {
						fieldType, ok = candidateType, true
						break
					}
				}
			}
			if !ok {
				if safeJSONField.MatchString(name) {
					return fmt.Errorf("request body contains unknown field %q", name)
				}
				return errors.New("request body contains an unknown field")
			}
			if err := rejectUnknownJSONFields(child, fieldType); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		array, ok := value.([]any)
		if !ok {
			return nil
		}
		for _, child := range array {
			if err := rejectUnknownJSONFields(child, target.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		for _, child := range object {
			if err := rejectUnknownJSONFields(child, target.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func jsonFieldTypes(target reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "-" {
			continue
		}
		if tag == "" && field.Anonymous {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				for name, fieldType := range jsonFieldTypes(embedded) {
					fields[name] = fieldType
				}
				continue
			}
		}
		if tag == "" {
			tag = field.Name
		}
		fields[tag] = field.Type
	}
	return fields
}

// UnmarshalJSON applies the same context-selected strictness to nested raw JSON
// fragments decoded after the top-level request body.
func UnmarshalJSON(ctx context.Context, data []byte, dst any) error {
	if !strictJSONDecoding(ctx) {
		return json.Unmarshal(data, dst)
	}
	return decodeJSON(ctx, bytes.NewReader(data), dst)
}

func safeJSONDecodeError(err error) error {
	if errors.Is(err, io.EOF) {
		return io.EOF
	}
	const unknownPrefix = "json: unknown field \""
	message := err.Error()
	if strings.HasPrefix(message, unknownPrefix) && strings.HasSuffix(message, "\"") {
		field := strings.TrimSuffix(strings.TrimPrefix(message, unknownPrefix), "\"")
		if safeJSONField.MatchString(field) {
			return fmt.Errorf("request body contains unknown field %q", field)
		}
		return errors.New("request body contains an unknown field")
	}
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) && safeJSONField.MatchString(typeError.Field) {
		return fmt.Errorf("request body contains an invalid value for field %q", typeError.Field)
	}
	return errors.New("invalid JSON request body")
}

// WriteJSON writes body as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteErr maps a domain error sentinel onto its HTTP status and writes the
// body. The envelope carries BOTH bex's original {"error": msg} shape (kept
// for existing tooling) and Render's public-API error schema
// (components.schemas.error: {"id", "message"}, verified against the
// render-oss/cli generated client) — a client written for Render (the
// official CLI, any Render SDK) reads `.message` to report a real failure
// reason instead of falling back to a generic "unknown error"; bex-only
// callers keep reading `.error` unchanged. One mapper for every feature's
// REST fragment, so the surfaces answer identically.
func WriteErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ErrLogsUnavailable), errors.Is(err, ErrLogStoreUnavailable),
		errors.Is(err, ErrAPIKeysUnavailable), errors.Is(err, ErrSSHKeysUnavailable),
		errors.Is(err, ErrMetricsUnavailable), errors.Is(err, ErrAuthzUnavailable),
		errors.Is(err, ErrSecretsUnavailable), errors.Is(err, ErrWorkspacesUnavailable),
		errors.Is(err, ErrUsageUnavailable), errors.Is(err, ErrBillingUnavailable), errors.Is(err, ErrDeploysUnavailable),
		errors.Is(err, ErrAuditUnavailable), errors.Is(err, ErrGitHubUnavailable),
		errors.Is(err, ErrEventsUnavailable), errors.Is(err, ErrRegistryCredentialsUnavailable),
		errors.Is(err, ErrWebhooksUnavailable), errors.Is(err, ErrLogoutUnavailable),
		errors.Is(err, ErrShellUnavailable):
		code = http.StatusServiceUnavailable
	case errors.Is(err, ErrBadRequest):
		code = http.StatusBadRequest
	case errors.Is(err, ErrForbidden):
		code = http.StatusForbidden
	case errors.Is(err, ErrConflict), errors.Is(err, ErrBillingEnforced):
		code = http.StatusConflict
	}
	msg := err.Error()
	var ce *CodedError
	if errors.As(err, &ce) {
		WriteJSON(w, code, map[string]any{"error": msg, "message": msg, "id": statusErrID(code), "code": ce.Code, "params": ce.Params})
		return
	}
	// The plain path is exactly WriteErrStatus's envelope — delegate so the
	// {"error","message","id"} literal lives in one place and can't drift.
	WriteErrStatus(w, code, msg)
}

// WriteErrStatus writes an error response with an explicit status and message
// in WriteErr's exact envelope ({"id","error","message"}, Content-Type
// application/json). Use it for a failure that no domain-error sentinel carries
// — the auth gate's 401/503, a handler's parameter-validation 400, a
// method-not-allowed 405 — so every non-2xx path speaks the one Render-shaped
// error dialect a Render client (the official CLI, any SDK) keys on (w9/m38).
func WriteErrStatus(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]any{"error": msg, "message": msg, "id": statusErrID(status)})
}

// statusErrID maps an HTTP status onto the stable error id WriteErr/
// WriteErrStatus report in the "id" field, so both writers of the one dialect
// label a status identically.
func statusErrID(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusServiceUnavailable:
		return "unavailable"
	default:
		return "internal_error"
	}
}

const (
	// PositiveTTL is how long a positive introspection/authz answer may be reused.
	PositiveTTL = 30 * time.Second
	// CacheMax bounds a TTLCache; the expired are swept before a wholesale reset.
	CacheMax = 4096
)

// OryTransport is the one connection pool shared by every Ory/FGA-bound client
// (introspection, whoami, key management, authz checks): the traffic all targets
// one or two hosts, where the default transport's 2 idle conns would re-dial.
var OryTransport = func() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 32
	return tr
}()

// DrainClose drains and closes a response body so its connection returns to the
// pool for reuse.
func DrainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// HTTPStatusError is DoJSON's unexpected-status error; callers may map specific
// codes (e.g. 404 -> ErrNotFound) via errors.As.
type HTTPStatusError struct {
	Code    int
	Summary string // "METHOD path returned 404 Not Found"
}

func (e *HTTPStatusError) Error() string { return e.Summary }

// DoJSON runs one JSON-over-HTTP call against an Ory/FGA-style API: optional JSON
// body, optional bearer, drain-close for connection reuse, unexpected status ->
// *HTTPStatusError, response decoded into out when non-nil. Shared by the Hydra
// admin store and the OpenFGA checker so request mechanics can't drift.
func DoJSON(ctx context.Context, client *http.Client, method, endpoint, bearer string, body []byte, want int, out any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rdr)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer DrainClose(resp)
	if resp.StatusCode != want {
		return &HTTPStatusError{Code: resp.StatusCode, Summary: method + " " + endpoint + " returned " + resp.Status}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// TTLCache is the positive-result cache the auth gate and OpenFGA checker share:
// entries expire after their expiry, expired entries are dropped on lookup, and
// at CacheMax the expired are swept before a wholesale reset.
type TTLCache[V any] struct {
	mu sync.Mutex
	m  map[string]ttlEntry[V]
}

type ttlEntry[V any] struct {
	v       V
	expires time.Time
}

// NewTTLCache returns an empty TTLCache.
func NewTTLCache[V any]() *TTLCache[V] { return &TTLCache[V]{m: map[string]ttlEntry[V]{}} }

// Get returns the live value for key, or false if absent/expired.
func (c *TTLCache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	if !time.Now().Before(e.expires) {
		delete(c.m, key) // dead entry — don't let it linger until CacheMax
		var zero V
		return zero, false
	}
	return e.v, true
}

// Delete evicts key immediately, regardless of its expiry — the counterpart to
// Put for a cached positive answer that must not survive a state change no
// timer alone can observe (e.g. the resource the key resolves to is deleted
// mid-TTL). A no-op if key isn't cached.
func (c *TTLCache[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

// DeleteIf evicts every entry whose value matches. It is intentionally a
// cache-sized linear scan: callers use it for rare upstream revocations where
// all cached aliases of one logical credential must disappear atomically.
func (c *TTLCache[V]) DeleteIf(match func(V) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.m {
		if match(entry.v) {
			delete(c.m, key)
		}
	}
}

// Put caches v until the given expiry (callers may clamp below PositiveTTL, e.g.
// to a token's own exp).
func (c *TTLCache[V]) Put(key string, v V, expires time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= CacheMax {
		now := time.Now()
		for k, e := range c.m { // sweep the expired before nuking the live
			if !now.Before(e.expires) {
				delete(c.m, k)
			}
		}
		if len(c.m) >= CacheMax {
			c.m = map[string]ttlEntry[V]{}
		}
	}
	c.m[key] = ttlEntry[V]{v: v, expires: expires}
}
