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
	"context"
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// requestBudgetExempt names the REST routes that outlive core.RequestBudget on
// purpose (w5/m286). Each one extends its own write deadline or streams, so a
// fixed deadline would cut it: the log tail, a sandbox's exec stream and
// long resume waits, and agent attachment transfers.
var requestBudgetExempt = []struct{ method, prefix string }{
	{http.MethodGet, "/v1/logs/subscribe"},
	{"", "/v1/sandboxes/"},
	{"", agentsessions.AttachmentUploadPath},
}

func budgetExempt(r *http.Request) bool {
	for _, e := range requestBudgetExempt {
		if (e.method == "" || e.method == r.Method) && strings.HasPrefix(r.URL.Path, e.prefix) {
			return true
		}
	}
	return false
}

// withRequestBudget cancels a REST handler's context at core.RequestBudget,
// inside the server's write timeout, so a slow dependency ends in
// REQUEST_TIMEOUT (core.WriteErr) instead of a 502 whose work keeps running.
func (s *Server) withRequestBudget(next http.Handler) http.Handler {
	budget := s.restBudget
	if budget <= 0 {
		budget = core.RequestBudget
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if budgetExempt(r) {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
