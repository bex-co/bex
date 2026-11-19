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
	// Now supplies the clock; nil is time.Now. SigV4's 15-minute window and the
	// presigned expiry read it.
	Now func() time.Time
}

// Gateway is the S3 surface. It is an http.Handler that serves S3 and nothing
// else.
type Gateway struct {
	cfg      Config
	degraded bool
}

// New returns the gateway.
func New(cfg Config) *Gateway {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Gateway{cfg: cfg}
}

// NewDegraded returns a gateway that answers 503 ServiceUnavailable to every
// request: the process is up and Ready, but a credential it needs is not
// installed, so a cluster without Wasabi or without a control plane does not
// crash-loop (ADR112 D9).
func NewDegraded() *Gateway { return &Gateway{degraded: true, cfg: Config{Now: time.Now}} }

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
	g.serve(w, r)
}

// serve is a configured gateway's request path: authenticate, route, authorize,
// run. Each step refuses with S3's XML before the next costs more.
func (g *Gateway) serve(w http.ResponseWriter, r *http.Request) {
	rq, e := g.authenticate(w, r)
	if e != nil {
		writeError(w, r, e)
		return
	}
	if e := g.dispatch(rq); e != nil {
		writeError(w, r, e)
	}
}

// authenticate verifies the request's SigV4 signature against its access key's
// secret and returns the request bound to the key's workspace. An OpenBao failure
// is a 503, never a 403.
func (g *Gateway) authenticate(w http.ResponseWriter, r *http.Request) (*request, *Error) {
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
	cred, err := g.cfg.Keys.Authenticate(r.Context(), p.keyID)
	switch {
	case errors.Is(err, objectstorage.ErrInvalidAccessKey):
		return nil, errInvalidAccessKeyID()
	case errors.Is(err, objectstorage.ErrSecretStoreUnavailable):
		return nil, errServiceUnavailable("The secret store is temporarily unavailable")
	case err != nil:
		log.Printf("s3gw: authenticate %s: %v", p.keyID, err)
		return nil, errInternal()
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
	if e := rq.authorize(rt.relation, rt.verb, rq.target()); e != nil {
		return e
	}
	h, ok := handlers[rt.op]
	if !ok {
		return errNotImplemented("The " + rt.op + " operation is not implemented")
	}
	return h(rq)
}
