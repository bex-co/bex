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

package s3gw

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// Addr is the S3 listener and ProbeAddr the probe and /metrics one. Constants,
// not environment variables: an environment variable here carries a credential
// and nothing else (ADR112 D9).
const (
	Addr      = ":8080"
	ProbeAddr = ":9090"
)

// RequestDeadline bounds the read and the write of every request that is not an
// object stream. A stream extends its own deadlines to the stream deadline
// (objectstorage.StreamDeadline), because the server sets none overall.
const RequestDeadline = 30 * time.Second

// Config is what the gateway serves with.
type Config struct {
	Service *objectstorage.Service
	Keys    *objectstorage.AccessKeys
	Base    *core.Base
	// TrustedProxies are the edge proxies whose forwarding headers name the client
	// the failed-authentication budget is kept for (BEX_TRUSTED_PROXY_CIDRS).
	TrustedProxies core.TrustedProxies
	// Metrics is the gateway's telemetry; nil records nothing.
	Metrics *Metrics
	// Now supplies the clock; nil is time.Now. SigV4's 15-minute window and the
	// presigned expiry read it.
	Now func() time.Time
}

// Gateway is the S3 surface. It is an http.Handler that serves S3 and nothing
// else.
type Gateway struct {
	cfg      Config
	degraded bool
	failures *authFailures
	// views dedupes the download row of a get to one per access key, bucket and key in
	// the audit window (ADR112 D6).
	views  *onceWindow
	limits *requestLimits
}

// New returns the gateway.
func New(cfg Config) *Gateway {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	g := &Gateway{cfg: cfg, failures: newAuthFailures(), views: newOnceWindow(), limits: newRequestLimits()}
	g.wireAudit()
	return g
}

// wireAudit puts the denial dedupe in front of the Base's audit sink (ADR112 D6).
func (g *Gateway) wireAudit() {
	if b := g.cfg.Base; b != nil && b.Audit != nil {
		b.Audit = &denialDedupe{inner: b.Audit, once: newOnceWindow(), now: func() time.Time { return g.cfg.Now() }}
	}
}

// NewDegraded returns a gateway that answers 503 ServiceUnavailable to every
// request: the process is up and Ready, but a credential it needs is not
// installed, so a cluster without Wasabi or without a control plane does not
// crash-loop (ADR112 D9).
func NewDegraded() *Gateway {
	return &Gateway{degraded: true, cfg: Config{Now: time.Now}, failures: newAuthFailures(), limits: newRequestLimits()}
}

// ServeHTTP answers one request. Everything that can refuse runs before a body
// byte is read (authentication, authorization, name and key validation, the
// quota gate and the stream slot), so a client that sent Expect: 100-continue is
// refused before it sends a byte.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("x-amz-request-id", newRequestID())
	w.Header().Set("Server", "bex")
	rc := http.NewResponseController(w)
	deadline := g.cfg.Now().Add(RequestDeadline)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
	if g.degraded || !g.cfg.Service.Enabled() || !g.cfg.Keys.Enabled() {
		writeError(w, r, errServiceUnavailable("object storage is not configured on this deployment"))
		return
	}
	if r.Method == http.MethodOptions {
		g.preflight(w, r)
		return
	}
	g.serve(w, r)
}

// serve is a configured gateway's request path: authenticate, route, authorize,
// run. Each step refuses with S3's XML before the next costs more.
func (g *Gateway) serve(w http.ResponseWriter, r *http.Request) {
	cw, record := g.cfg.Metrics.instrument(w, r)
	client := g.clientKey(r)
	rq, e := g.authenticate(cw, r, g.failures.exhausted(client))
	if e != nil {
		if isAuthFailure(e) {
			g.failures.record(client)
		}
		writeError(cw, r, e)
		record("unauthenticated", true, e)
		return
	}
	e = g.dispatch(rq)
	if e != nil {
		writeError(cw, r, e)
	}
	op := "unrouted"
	if rq.route != nil {
		op = rq.route.operation(rq.r)
	}
	record(op, false, e)
}

// authenticate verifies the request's SigV4 signature against its access key's
// secret and returns the request bound to the key's workspace. An OpenBao failure
// is a 503, never a 403.
func (g *Gateway) authenticate(w http.ResponseWriter, r *http.Request, overBudget bool) (*request, *Error) {
	form, e := classify(r)
	if e != nil {
		return nil, e
	}
	if form == formAnonymous {
		return nil, errAccessDenied("Anonymous access is forbidden for this operation: sign the request with an access key")
	}
	p, e := parseAuth(r, form, g.cfg.Now())
	if e != nil {
		return nil, e
	}
	if e := p.checkSignedHeaders(r); e != nil {
		return nil, e
	}
	var cred objectstorage.Credential
	if overBudget {
		// This address has failed too often: only a key this replica already serves
		// is answered, from memory.
		if cred, e = g.lookupFromCaches(p.keyID); e != nil {
			return nil, e
		}
	} else {
		var err error
		cred, err = g.cfg.Keys.Authenticate(r.Context(), p.keyID)
		switch {
		case errors.Is(err, objectstorage.ErrInvalidAccessKey):
			return nil, errInvalidAccessKeyID()
		case errors.Is(err, objectstorage.ErrSecretStoreUnavailable):
			return nil, errServiceUnavailable("The secret store is temporarily unavailable")
		case err != nil:
			log.Printf("s3gw: authenticate %s: %v", p.keyID, err)
			return nil, errInternal()
		}
	}
	if e := p.verify(r, cred.Secret); e != nil {
		return nil, e
	}
	// The request acts as the key's creator, on the key's workspace and nothing
	// else; its audit rows name the key.
	ctx := core.WithIdentity(r.Context(), core.Identity{
		Subject: cred.Key.CreatedBy, Method: core.MethodS3Key, SignIn: cred.Key.CreatedSignIn, AuditSubject: cred.Key.ID,
	})
	ctx = core.WithWorkspace(ctx, cred.Key.WorkspaceID)
	bucket, key := splitPath(r.URL.Path)
	return &request{
		g: g, w: w, r: r, ctx: ctx, key: cred.Key, workspaceID: cred.Key.WorkspaceID,
		bucket: bucket, objectKey: key, q: r.URL.Query(),
	}, nil
}

// dispatch routes an authenticated request and runs its operation.
func (g *Gateway) dispatch(rq *request) *Error {
	if !g.limits.allow(rq.key.ID, rq.workspaceID) {
		return errSlowDown("Too many requests from this access key or workspace; slow down.")
	}
	lv := levelObject
	switch {
	case rq.bucket == "":
		lv = levelService
	case rq.objectKey == "":
		lv = levelBucket
	}
	rt, e := match(rq.r.Method, lv, rq.q)
	if e != nil {
		return e
	}
	rq.route = rt
	if e := refuseUnimplementedHeaders(rq.r, rt); e != nil {
		return e
	}
	verb := rt.verb
	switch op := rt.operation(rq.r); op {
	case "CreateBucket":
		// The allowed row is written once the bucket exists (createBucket).
		rq.ctx = core.WithDeferredAllowedWriteAudit(rq.ctx)
	case "CopyObject":
		// A copy is its own verb, and its allowed row names the source too, once the
		// copy happened (copyObject); a denial of it is still recorded at once.
		verb = objectstorage.AuditVerbCopyObject
		rq.ctx = core.WithDeferredAllowedWriteAudit(rq.ctx)
	case "UploadPart", "UploadPartCopy":
		// A part writes no allowed row, however many a large upload has (a 1 GiB `aws s3
		// cp` is 128); the upload's create and complete rows are its record, and a denied
		// part is still recorded at once, once per window. A part copy is a copy.
		if op == "UploadPartCopy" {
			verb = objectstorage.AuditVerbCopyObject
		}
		rq.ctx = core.WithDeferredAllowedWriteAudit(rq.ctx)
	case "DeleteObjects":
		// One row for the batch, once it ran, with its count and first key.
		rq.ctx = core.WithDeferredAllowedWriteAudit(rq.ctx)
	}
	if e := rq.authorize(rt.relation, verb, rq.target()); e != nil {
		return e
	}
	// Names are validated after the caller is authorized, so a refused caller's
	// key is only ever bounded and escaped in the audit row (ADR112 D6, D7).
	if lv == levelObject {
		if e := validateKey(rq.objectKey); e != nil {
			return e
		}
	}
	h, ok := handlers[rt.op]
	if !ok {
		return errNotImplemented("The " + rt.op + " operation is not implemented")
	}
	return h(rq)
}

// validateKey is ADR112 D7 on the S3 routes: an empty key, one over 1,024 bytes, one
// that starts with a slash, has a . or .. segment or a doubled slash (Traefik cleans
// both before bex sees them, which would break the signature), and one with a
// backslash or a control character are refused.
func validateKey(key string) *Error {
	if len(key) > objectstorage.MaxKeyBytes {
		return newError(http.StatusBadRequest, "KeyTooLongError", "Your key is too long.")
	}
	if err := objectstorage.ValidateKey(key); err != nil {
		return errInvalidArgument("The object key is not valid: it must be 1 to 1,024 bytes of UTF-8 with no leading slash, doubled slash, . or .. segment, backslash or control character.")
	}
	return nil
}
