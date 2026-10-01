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
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// serveMuxJSON changes only the mux's generated routing errors. Matched
// handlers keep their writer (including streaming interfaces) and ServeMux
// still populates wildcard path values. Canonical-path redirects pass through.
func serveMuxJSON(mux *http.ServeMux, w http.ResponseWriter, r *http.Request) {
	_, pattern := mux.Handler(r)
	if pattern == "" {
		mux.ServeHTTP(&routingErrorWriter{ResponseWriter: w}, r)
		return
	}
	mux.ServeHTTP(w, r)
}

type routingErrorWriter struct {
	http.ResponseWriter
	translated bool
}

func (w *routingErrorWriter) WriteHeader(status int) {
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		w.translated = true
		core.WriteErrStatus(w.ResponseWriter, status, strings.ToLower(http.StatusText(status)))
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *routingErrorWriter) Write(body []byte) (int, error) {
	if w.translated {
		return len(body), nil
	}
	return w.ResponseWriter.Write(body)
}
