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

package objectgw

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// Verbs is bex-api's half of the gateway's two calls (ADR112 D4). It mounts on
// the cluster-internal :8091 listener only, never on the public :8090 mux: the
// bearer is the verbs' whole authentication, so they must not be reachable from
// the internet.
type Verbs struct {
	// Token is BEX_OBJECT_STORAGE_GATEWAY_TOKEN. Empty refuses every call, so a
	// misconfigured mount fails closed.
	Token string
	// Service is the object store.
	Service *objectstorage.Service
	// BaseDomain is BEX_BASE_DOMAIN, the suffix of every public bucket host.
	BaseDomain string
}

// Handler serves both verbs.
func (v *Verbs) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+LookupPath, v.lookup)
	mux.HandleFunc("POST "+EgressPath, v.egress)
	return v.authenticated(mux)
}

// authenticated checks the bearer before anything else, in constant time, so a
// wrong or missing token is refused before any database work and the verbs
// cannot be used to probe which buckets exist.
func (v *Verbs) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v.Token == "" {
			core.WriteErrStatus(w, http.StatusServiceUnavailable, "the object gateway is not configured")
			return
		}
		got := []byte(r.Header.Get("Authorization"))
		want := []byte("Bearer " + v.Token)
		if subtle.ConstantTimeCompare(got, want) != 1 {
			core.WriteErrStatus(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (v *Verbs) lookup(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		core.WriteErrStatus(w, http.StatusBadRequest, "host query parameter is required")
		return
	}
	pb, err := v.Service.PublicBucketForHost(r.Context(), host, v.BaseDomain)
	switch {
	case errors.Is(err, objectstorage.ErrBucketNotFound):
		core.WriteErrStatus(w, http.StatusNotFound, "not found")
		return
	case errors.Is(err, objectstorage.ErrNotConfigured):
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "object storage is not configured")
		return
	case err != nil:
		log.Printf("bex-api: object gateway lookup: %v", err)
		core.WriteErrStatus(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	cors := pb.CORS
	if cors == nil {
		cors = []objectstorage.CORSRule{}
	}
	w.Header().Set("Cache-Control", "no-store")
	core.WriteJSON(w, http.StatusOK, LookupAnswer{
		BucketID: pb.BucketID, WorkspaceID: pb.WorkspaceID, Region: string(pb.Region), CORS: cors,
	})
}

// egress adds a gateway report to the workspaces' egress shadow meters (ADR112
// D8). A malformed or oversized report is refused whole, and an entry that is
// not a workspace id is refused with it: a report is the gateway's own sums, so
// a bad one is a bug to see, not data to trim.
func (v *Verbs) egress(w http.ResponseWriter, r *http.Request) {
	var report EgressReport
	if err := core.DecodeLimitedJSON(r.Body, MaxReportBodyBytes, &report); err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "the report is not valid JSON within its size limit")
		return
	}
	if len(report.Bytes) > MaxReportWorkspaces {
		core.WriteErrStatus(w, http.StatusBadRequest, "the report names too many workspaces")
		return
	}
	for workspaceID, n := range report.Bytes {
		if kind, ok := ids.KindOf(workspaceID); !ok || kind != ids.Workspace || n < 0 || n > maxReportBytesPerWorkspace {
			core.WriteErrStatus(w, http.StatusBadRequest, "the report has an entry that is not a workspace and a byte count")
			return
		}
	}
	for workspaceID, n := range report.Bytes {
		v.Service.RecordEgress(r.Context(), workspaceID, n)
	}
	w.Header().Set("Cache-Control", "no-store")
	core.WriteJSON(w, http.StatusOK, map[string]int{"accepted": len(report.Bytes)})
}
