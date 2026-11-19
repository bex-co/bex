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
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CORS on the S3 host (ADR112 D3, D4). The host is reached by a browser only with a
// presigned URL, and the bucket's rules apply there as on a public bucket's host: a
// preflight is answered from the rules and a response to a presigned request carries
// the matching rule's headers. A preflight carries no credentials, so it cannot name
// its workspace: it resolves its bucket only from the key id inside the
// X-Amz-Credential of a presigned URL's query, which is a routing hint and never an
// authentication. An unknown or revoked id, a missing bucket and a refused origin all
// answer the same 403 with no CORS header, so a preflight cannot tell them apart. A
// header-signed browser SDK cannot be served, because its preflight carries no key id.
// Credentials are never allowed.

// preflight answers an OPTIONS request.
func (g *Gateway) preflight(w http.ResponseWriter, r *http.Request) {
	deny := func() { writeError(w, r, errAccessDenied("CORSResponse: This CORS request is not allowed.")) }
	origin, method := r.Header.Get("Origin"), r.Header.Get("Access-Control-Request-Method")
	if origin == "" || method == "" {
		deny()
		return
	}
	client := g.clientKey(r)
	if g.failures.exhausted(client) {
		deny()
		return
	}
	keyID, _, _ := strings.Cut(r.URL.Query().Get("X-Amz-Credential"), "/")
	key, err := g.cfg.Keys.Resolve(r.Context(), keyID)
	if err != nil {
		g.failures.record(client)
		deny()
		return
	}
	bucketName, _ := splitPath(r.URL.Path)
	b, err := g.cfg.Service.ResolveBucket(r.Context(), key.WorkspaceID, bucketName)
	if err != nil {
		g.failures.record(client)
		deny()
		return
	}
	rules, err := objectstorage.ParseCORS(b.CORS)
	requested := objectstorage.SplitHeaderList(r.Header.Get("Access-Control-Request-Headers"))
	m, ok := objectstorage.MatchCORS(rules, origin, method, requested)
	if err != nil || !ok {
		deny()
		return
	}
	h := w.Header()
	m.SetPreflight(h, origin, requested)
	h.Add("Vary", "Origin")
	h.Add("Vary", "Access-Control-Request-Method")
	h.Add("Vary", "Access-Control-Request-Headers")
	w.WriteHeader(http.StatusNoContent)
}

// addCORS adds the matching rule's headers to a response to a presigned request that
// carries an Origin, and Vary: Origin to every answer of a bucket that has rules.
func (rq *request) addCORS(b store.ObjectBucket) {
	origin := rq.r.Header.Get("Origin")
	if origin == "" || len(b.CORS) == 0 {
		return
	}
	rules, err := objectstorage.ParseCORS(b.CORS)
	if err != nil || len(rules) == 0 {
		return
	}
	rq.w.Header().Add("Vary", "Origin")
	if m, ok := objectstorage.MatchCORS(rules, origin, rq.r.Method, nil); ok {
		m.SetActual(rq.w.Header(), origin)
	}
}
