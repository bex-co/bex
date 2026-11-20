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

package appearance

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST mounts the bex extension beside Render's whoami (GET
// /v1/users): Render's public API has no theme route.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/users/theme-settings", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.Get(r.Context())
	}))
	mux.HandleFunc("PATCH /v1/users/theme-settings", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		// An unknown field is a 400, not a silent no-op.
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		patch, err := core.DecodeBody[Patch](r)
		if err != nil {
			return nil, err
		}
		return s.Update(r.Context(), patch)
	}))
}
