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

package oidcissuer

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

// The two documents under <issuer>/, and their shared cache header (Render's,
// probed 2026-11-18; ADR116 D1).
const (
	discoveryDocument = ".well-known/openid-configuration"
	jwksDocument      = ".well-known/jwks.json"
	cacheControl      = "public, max-age=300"
)

// discovery is Render's five fields, in Render's order.
type discovery struct {
	Issuer                           string   `json:"issuer"`
	JWKSURI                          string   `json:"jwks_uri"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

// ServeHTTP answers the issuer host: every workspace's discovery document and
// JWKS, and Go's 404 for anything else, Render's answers byte for byte. Any
// well-formed workspace id answers; nothing checks that it exists.
func (i *Issuer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Render's order: a path of a document's shape with another method is a
	// 405 whatever the id; only a GET has its id checked.
	workspace, document, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if workspace == "" || (document != discoveryDocument && document != jwksDocument) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !id.Is(id.Workspace, workspace) {
		http.NotFound(w, r)
		return
	}
	if document == discoveryDocument {
		issuer := i.URL(workspace)
		writeDocument(w, discovery{
			Issuer:                           issuer,
			JWKSURI:                          issuer + "/" + jwksDocument,
			ResponseTypesSupported:           []string{"id_token"},
			SubjectTypesSupported:            []string{"public"},
			IDTokenSigningAlgValuesSupported: []string{"ES256"},
		}, true)
		return
	}
	keys, ok := i.keys.published(i.keys.now())
	if !ok {
		http.Error(w, "the signing keys are not loaded yet", http.StatusServiceUnavailable)
		return
	}
	set := struct {
		Keys []jwk `json:"keys"`
	}{Keys: make([]jwk, 0, len(keys))}
	for _, k := range keys {
		set.Keys = append(set.Keys, k.public)
	}
	writeDocument(w, set, false)
}

// writeDocument writes v as JSON. Render's discovery document ends in a
// newline and its JWKS does not; newline keeps that difference.
func writeDocument(w http.ResponseWriter, v any, newline bool) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if newline {
		body = append(body, '\n')
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", cacheControl)
	_, _ = w.Write(body)
}
