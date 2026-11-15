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

package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// The Cloudflare-shaped object storage surface (ADR112 D3): the routes stock
// `wrangler r2` calls when CLOUDFLARE_API_BASE_URL points here. They are not
// Render's API, so ADR018's Render request gate does not cover them; bex
// validates its own input, and every answer is Cloudflare's envelope.
const (
	// cloudflareMount is where the whole surface lives, and the base URL a
	// customer gives wrangler.
	cloudflareMount = "/cloudflare/client/v4"
	// cloudflareAccountR2 is the prefix of the bucket and object routes. The
	// account id is the workspace id (tea-...), which wrangler accepts.
	cloudflareAccountR2 = cloudflareMount + "/accounts/{accountId}/r2"
	// cloudflareMountPattern is the subtree the root mux hands to this surface.
	cloudflareMountPattern = cloudflareMount + "/"
)

// Cloudflare's published R2 error codes (read 2026-11-14 at
// developers.cloudflare.com/r2/api/error-codes), the subset bex answers.
const (
	cfCodeInternal             = 10001
	cfCodeUnauthorized         = 10002
	cfCodeAccessDenied         = 10003
	cfCodeInvalidBucketName    = 10005
	cfCodeNoSuchBucket         = 10006
	cfCodeNoSuchKey            = 10007
	cfCodeBucketNotEmpty       = 10008
	cfCodeTooManyBuckets       = 10009
	cfCodeIncompleteBody       = 10013
	cfCodeInvalidObjectName    = 10020
	cfCodeMissingContentLength = 10033
	cfCodeNotEntitled          = 10042
	cfCodeServiceUnavailable   = 10043
	cfCodeTooManyRequests      = 10058
	cfCodeBucketConflict       = 10073
	cfCodeEntityTooLarge       = 100100

	// Codes the R2 table does not carry. 7003 and 7000 are Cloudflare's own
	// answers for a URI it cannot route; 7400 is bex's for a request it cannot
	// read or whose rules break a limit (recorded in ADR018, unsigned).
	cfCodeNoRoute        = 7003
	cfCodeMethodNotAllow = 7000
	cfCodeInvalidRequest = 7400
)

// cfErrorEntry is one element of the envelope's errors array.
type cfErrorEntry struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// cfEnvelope is the body of every JSON answer, success or failure. Every 2xx
// carries one: wrangler fails on a bodiless 2xx from an object route.
type cfEnvelope struct {
	Success    bool           `json:"success"`
	Errors     []cfErrorEntry `json:"errors"`
	Messages   []string       `json:"messages"`
	Result     any            `json:"result"`
	ResultInfo *cfResultInfo  `json:"result_info,omitempty"`
}

// cfError is a failure with its HTTP status and Cloudflare code.
type cfError struct {
	Status  int
	Code    int
	Message string
}

func (e *cfError) Error() string { return e.Message }

func cfFail(status, code int, format string, args ...any) *cfError {
	return &cfError{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func writeCFJSON(w http.ResponseWriter, status int, env cfEnvelope) {
	if env.Errors == nil {
		env.Errors = []cfErrorEntry{}
	}
	if env.Messages == nil {
		env.Messages = []string{}
	}
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

// writeCFResult answers 200 with result inside the envelope.
func writeCFResult(w http.ResponseWriter, result any) {
	if result == nil {
		result = struct{}{}
	}
	writeCFJSON(w, http.StatusOK, cfEnvelope{Success: true, Result: result})
}

func writeCFError(w http.ResponseWriter, e *cfError) {
	if e.Status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "1")
	}
	writeCFJSON(w, e.Status, cfEnvelope{Errors: []cfErrorEntry{{Code: e.Code, Message: e.Message}}})
}

// cloudflareR2 serves the Cloudflare-shaped routes. Like renderObjects it is a
// thin adapter over objectstorage.Service (ADR006).
type cloudflareR2 struct {
	base *core.Base
	svc  *objectstorage.Service
	// baseDomain is BEX_BASE_DOMAIN, the suffix of a public bucket's host
	// (ADR112 D4). Empty: public buckets are not offered.
	baseDomain string
}

func newCloudflareR2(base *core.Base, svc *objectstorage.Service, baseDomain string) *cloudflareR2 {
	return &cloudflareR2{base: base, svc: svc, baseDomain: baseDomain}
}

// cfCall is one authorized request: the workspace its account id names.
type cfCall struct {
	c           *cloudflareR2
	w           http.ResponseWriter
	r           *http.Request
	workspaceID string
}

// route registers pattern with the relation its caller needs on the account's
// workspace (ADR112 D6). fn returns the failure to answer, or nil after it
// wrote its own answer.
func (c *cloudflareR2) route(mux *http.ServeMux, pattern, relation string, fn func(*cfCall) error) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if !c.svc.Enabled() {
			writeCFError(w, cfNotEntitled("object storage is not available on this bex deployment"))
			return
		}
		workspaceID, err := c.base.AuthorizeWorkspace(r.Context(), relation, r.PathValue("accountId"))
		if err != nil {
			writeCFError(w, cloudflareError(err))
			return
		}
		if err := fn(&cfCall{c: c, w: w, r: r, workspaceID: workspaceID}); err != nil {
			writeCFError(w, cloudflareError(err))
		}
	})
}

func cfNotEntitled(format string, args ...any) *cfError {
	return cfFail(http.StatusForbidden, cfCodeNotEntitled, format, args...)
}

// refuse registers pattern answering 10042 for a feature bex does not offer.
// It still needs a caller who can view the workspace, so the refusal is not an
// unauthenticated oracle for which account ids exist.
func (c *cloudflareR2) refuse(mux *http.ServeMux, pattern, message string) {
	c.route(mux, pattern, core.RelCanView, func(*cfCall) error {
		return cfNotEntitled("%s", message)
	})
}

// register mounts every route of the surface on mux.
func (c *cloudflareR2) register(mux *http.ServeMux) {
	c.registerBuckets(mux)
	c.registerObjects(mux)
	c.registerConfig(mux)
	c.registerManagedDomain(mux)
	c.registerNotOffered(mux)
}

// cloudflareError maps whatever a route returned to the answer Cloudflare's
// clients expect: a service error to its published code, an authorization or
// core error to its status. Anything unclassified is logged and answered as a
// bare internal error, so no raw failure text reaches a customer.
func cloudflareError(err error) *cfError {
	var ce *cfError
	if errors.As(err, &ce) {
		return ce
	}
	var notOffered *objectstorage.RegionNotOfferedError
	var quota *objectstorage.QuotaError
	switch {
	case errors.Is(err, objectstorage.ErrNotConfigured):
		return cfNotEntitled("object storage is not available on this bex deployment")
	case errors.As(err, &notOffered):
		return cfFail(http.StatusBadRequest, cfCodeNotEntitled, "%s", notOffered.Error())
	case errors.Is(err, objectstorage.ErrInvalidBucketName), errors.Is(err, objectstorage.ErrReservedBucketName):
		return cfFail(http.StatusBadRequest, cfCodeInvalidBucketName,
			"the bucket name must be 3 to 63 characters of lower-case letters, digits and hyphens, starting and ending with a letter or digit, and must not start with %s", objectstorage.SystemBucketPrefix)
	case errors.Is(err, objectstorage.ErrBucketNotFound):
		return cfFail(http.StatusNotFound, cfCodeNoSuchBucket, "the specified bucket does not exist")
	case errors.Is(err, objectstorage.ErrBucketExists):
		return cfFail(http.StatusConflict, cfCodeBucketConflict, "a bucket with this name already exists")
	case errors.Is(err, objectstorage.ErrBucketNotEmpty):
		return cfFail(http.StatusConflict, cfCodeBucketNotEmpty, "the bucket you tried to delete is not empty")
	case errors.Is(err, objectstorage.ErrTooManyBuckets):
		return cfFail(http.StatusBadRequest, cfCodeTooManyBuckets, "this workspace has reached its limit of %d buckets", objectstorage.MaxUserBuckets)
	case errors.Is(err, objectstorage.ErrInvalidKey):
		return cfFail(http.StatusBadRequest, cfCodeInvalidObjectName,
			"the object key is not valid: it must be 1 to %d bytes of UTF-8, must not start with /, and must have no . or .. segment, backslash or control character", objectstorage.MaxKeyBytes)
	case errors.Is(err, objectstorage.ErrObjectNotFound):
		return cfFail(http.StatusNotFound, cfCodeNoSuchKey, "the specified key does not exist")
	case errors.Is(err, objectstorage.ErrMissingContentLength):
		return cfFail(http.StatusLengthRequired, cfCodeMissingContentLength, "the request must carry a Content-Length header")
	case errors.Is(err, objectstorage.ErrIncompleteBody):
		return cfFail(http.StatusBadRequest, cfCodeIncompleteBody, "the request body ended before its Content-Length")
	case errors.Is(err, objectstorage.ErrObjectTooLarge):
		return cfFail(http.StatusBadRequest, cfCodeEntityTooLarge, "an object must be at most %d bytes", objectstorage.MaxObjectBytesStreamed)
	case errors.As(err, &quota):
		return cfNotEntitled("%s", quota.Error())
	case errors.Is(err, objectstorage.ErrCORSOverCap), errors.Is(err, objectstorage.ErrLifecycleOverCap), errors.Is(err, objectstorage.ErrLifecycleStorageClass):
		return cfNotEntitled("%s", err.Error())
	case errors.Is(err, objectstorage.ErrInvalidCORS), errors.Is(err, objectstorage.ErrInvalidLifecycle):
		return cfFail(http.StatusBadRequest, cfCodeInvalidRequest, "%s", err.Error())
	case errors.Is(err, objectstorage.ErrTooManyStreams), errors.Is(err, objectstorage.ErrRateLimited):
		return cfFail(http.StatusTooManyRequests, cfCodeTooManyRequests, "too many concurrent object requests; retry shortly")
	}
	status, message, _ := core.ErrorReply(err)
	switch {
	case status == http.StatusForbidden || status == http.StatusNotFound:
		// A workspace the caller cannot see answers like one they may not use,
		// so an account id does not reveal whether it exists.
		return cfFail(http.StatusForbidden, cfCodeAccessDenied, "%s", accessMessage(message, status))
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError, status == http.StatusServiceUnavailable:
		return cfFail(status, cfCodeForStatus(status), "%s", message)
	}
	log.Printf("bex-api: cloudflare r2: internal error: %v", err)
	return cfFail(http.StatusInternalServerError, cfCodeInternal, "internal error")
}

// cfCodeForStatus is the Cloudflare code for an HTTP status that a core error or
// shared middleware produced: the code a client sees when no R2 code applies.
func cfCodeForStatus(status int) int {
	switch status {
	case http.StatusUnauthorized:
		return cfCodeUnauthorized
	case http.StatusForbidden, http.StatusPaymentRequired:
		return cfCodeAccessDenied
	case http.StatusNotFound:
		return cfCodeNoRoute
	case http.StatusMethodNotAllowed:
		return cfCodeMethodNotAllow
	case http.StatusRequestEntityTooLarge:
		return cfCodeEntityTooLarge
	case http.StatusTooManyRequests:
		return cfCodeTooManyRequests
	case http.StatusServiceUnavailable:
		return cfCodeServiceUnavailable
	}
	if status >= http.StatusInternalServerError {
		return cfCodeInternal
	}
	return cfCodeInvalidRequest
}

// accessMessage keeps a refusal's own wording (a required second factor, say)
// but never a not-found's, which would name the workspace.
func accessMessage(message string, status int) string {
	if status == http.StatusNotFound {
		return "access denied"
	}
	return message
}

// readCFBody reads a small JSON request body leniently: wrangler sends bucket
// create as text/plain, so the content type is not consulted, and a field bex
// does not know is ignored. A body over max is refused before it is parsed.
func readCFBody(r *http.Request, max int64, dst any) error {
	err := core.DecodeLimitedJSON(r.Body, max, dst)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, core.ErrUpstreamResponseTooLarge):
		return cfNotEntitled("the request body is over bex's limit of %d bytes", max)
	}
	return cfFail(http.StatusBadRequest, cfCodeInvalidRequest, "the request body is not valid JSON for this route")
}

// extendStreamDeadlines lifts the server's 30 s read and write deadlines for
// one object transfer, which would cut a large body (precedent:
// deploys.platform_rebuild). A writer that does not support it keeps the
// server's own, which only a test writer does.
func (c *cloudflareR2) extendStreamDeadlines(w http.ResponseWriter) {
	deadline := time.Now().Add(objectstorage.StreamDeadline)
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
}

// cfGateKey carries the gate writer to the route handler, which tells it the
// answer is the route's own.
type cfGateKey struct{}

// cloudflareGate is the outermost wrapper of the surface. Failures before a
// route runs (the auth gate's 401, the rate limiter's 429, the scope-class
// refusal) are written by shared middleware in bex's own JSON shape; this turns
// each into Cloudflare's envelope, keeping its headers and its message, so
// wrangler prints something it understands.
func cloudflareGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate := &cfGateWriter{ResponseWriter: w}
		defer gate.finish()
		next.ServeHTTP(gate, r.WithContext(context.WithValue(r.Context(), cfGateKey{}, gate)))
	})
}

// cloudflareReached marks that the request got through the shared middleware:
// from here on, whatever is written is a route's own answer.
func cloudflareReached(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gate, ok := r.Context().Value(cfGateKey{}).(*cfGateWriter); ok {
			gate.reached = true
		}
		next.ServeHTTP(w, r)
	})
}

type cfGateWriter struct {
	http.ResponseWriter
	reached bool
	pending int // the status of a shared middleware's failure, held until finish
	body    []byte
}

func (g *cfGateWriter) WriteHeader(status int) {
	if !g.reached && status >= http.StatusBadRequest {
		g.pending = status
		return
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *cfGateWriter) Write(b []byte) (int, error) {
	if g.pending != 0 {
		if room := 4096 - len(g.body); room > 0 {
			g.body = append(g.body, b[:min(len(b), room)]...)
		}
		return len(b), nil
	}
	return g.ResponseWriter.Write(b)
}

func (g *cfGateWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }

// finish writes the held failure as an envelope.
func (g *cfGateWriter) finish() {
	if g.pending == 0 {
		return
	}
	status := g.pending
	g.pending = 0
	message := http.StatusText(status)
	var held struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(g.body, &held) == nil {
		if m := cmp.Or(held.Message, held.Error); m != "" {
			message = m
		}
	}
	code := cfCodeForStatus(status)
	switch {
	case status == http.StatusPaymentRequired:
		status = http.StatusForbidden
	case status >= http.StatusInternalServerError && status != http.StatusServiceUnavailable:
		message = "internal error"
	}
	if status == http.StatusTooManyRequests && g.Header().Get("Retry-After") == "" {
		g.Header().Set("Retry-After", "1")
	}
	writeCFJSON(g.ResponseWriter, status, cfEnvelope{Errors: []cfErrorEntry{{Code: code, Message: message}}})
}

// cloudflareMux routes inside the surface and answers a path or method the mux
// does not have with Cloudflare's own codes, as serveMuxJSON does for REST.
func cloudflareMux(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveMuxWith(mux, w, r, func(w http.ResponseWriter, status int) {
			message := "no route for that URI"
			if status == http.StatusMethodNotAllowed {
				message = "the method is not allowed for that URI"
			}
			writeCFError(w, cfFail(status, cfCodeForStatus(status), "%s", message))
		})
	})
}

// isCloudflareObjectPath reports whether path is an object get or put, whose
// bytes must reach the client exactly as stored, uncompressed and with their
// Content-Length (withGzip leaves them alone).
func isCloudflareObjectPath(path string) bool {
	rest, ok := strings.CutPrefix(path, cloudflareMount+"/accounts/")
	if !ok {
		return false
	}
	return strings.Contains(rest, "/r2/buckets/") && strings.Contains(rest, "/objects/")
}
