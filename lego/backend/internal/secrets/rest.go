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

package secrets

import (
	"context"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the env-vars + secret-files REST fragment — Render public-API
// compatible: all five of Render's env-var endpoints (list, retrieve-one,
// replace-all, upsert-one, delete-one) and its four secret-file endpoints (list,
// retrieve-one, upsert-one, delete-one). Served under Render's noun /v1/services
// service routes. Behavior lives in the Service, so GraphQL and MCP stay
// identical.

// envVarWithCursor is Render's env-vars list-item envelope
// ({envVar:{key,value}, cursor}); GET/PUT return an array of these.
type envVarWithCursor struct {
	EnvVar EnvVarView `json:"envVar"`
	Cursor string     `json:"cursor"`
}

// toEnvVarList wraps key-sorted env vars in Render's cursor envelope; the key is
// a stable, opaque-enough cursor (as the App name is for services).
func toEnvVarList(vars []EnvVarView) []envVarWithCursor {
	out := make([]envVarWithCursor, 0, len(vars))
	for _, v := range vars {
		out = append(out, envVarWithCursor{EnvVar: v, Cursor: v.Key})
	}
	return out
}

// secretFileWithCursor is Render's secret-files list-item envelope
// ({secretFile:{name}, cursor}); GET returns an array of these.
type secretFileWithCursor struct {
	SecretFile SecretFileView `json:"secretFile"`
	Cursor     string         `json:"cursor"`
}

// toSecretFileList wraps name-sorted files in Render's cursor envelope.
func toSecretFileList(files []SecretFileView) []secretFileWithCursor {
	out := make([]secretFileWithCursor, 0, len(files))
	for _, f := range files {
		out = append(out, secretFileWithCursor{SecretFile: f, Cursor: f.Name})
	}
	return out
}

// RegisterREST adds the Render-shaped env-vars endpoints. Store unconfigured =>
// the Service returns core.ErrSecretsUnavailable => 503 on these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	const base = "/v1/services"
	// Bex's coherent environment-save extension. Existing Render-compatible
	// env-var and secret-file routes below retain their immediate-roll behavior.
	mux.HandleFunc("PATCH "+base+"/{id}/environment", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		in, err := core.DecodeBody[EnvironmentPatch](r)
		if err != nil {
			return nil, err
		}
		return s.PatchEnvironment(r.Context(), r.PathValue("id"), in)
	}))

	mux.HandleFunc("GET "+base+"/{id}/env-vars", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		vars, err := listPagedOrAll(r, s.ListEnvVarsPage, s.ListEnvVars)
		if err != nil {
			return nil, err
		}
		return toEnvVarList(vars), nil
	}))
	mux.HandleFunc("PUT "+base+"/{id}/env-vars", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		in, err := core.DecodeBody[[]EnvVarView](r)
		if err != nil {
			return nil, err
		}
		vars, err := s.SetEnvVars(r.Context(), r.PathValue("id"), in)
		if err != nil {
			return nil, err
		}
		return toEnvVarList(vars), nil
	}))
	// Render: bare {key,value}, no cursor envelope.
	mux.HandleFunc("GET "+base+"/{id}/env-vars/{key}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetEnvVar(r.Context(), r.PathValue("id"), r.PathValue("key"))
	}))
	mux.HandleFunc("PUT "+base+"/{id}/env-vars/{key}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[struct {
			Value         string `json:"value"`
			GenerateValue bool   `json:"generateValue"`
		}](r)
		if err != nil {
			return nil, err
		}
		return s.SetEnvVar(r.Context(), r.PathValue("id"), r.PathValue("key"), EnvVarWrite{Value: req.Value, GenerateValue: req.GenerateValue})
	}))
	// Render: delete => 204.
	mux.HandleFunc("DELETE "+base+"/{id}/env-vars/{key}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteEnvVar(r.Context(), r.PathValue("id"), r.PathValue("key"))
	}))

	// Secret files (Render's /v1/services/{id}/secret-files) — names in the
	// list, contents on a single GET, same store + roll mechanism as env vars.
	mux.HandleFunc("GET "+base+"/{id}/secret-files", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		files, err := listPagedOrAll(r, s.ListSecretFilesPage, s.ListSecretFiles)
		if err != nil {
			return nil, err
		}
		return toSecretFileList(files), nil
	}))
	// Bare {name, content}.
	mux.HandleFunc("GET "+base+"/{id}/secret-files/{name}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetSecretFile(r.Context(), r.PathValue("id"), r.PathValue("name"))
	}))
	mux.HandleFunc("PUT "+base+"/{id}/secret-files/{name}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[struct {
			Content string `json:"content"`
		}](r)
		if err != nil {
			return nil, err
		}
		return s.SetSecretFile(r.Context(), r.PathValue("id"), r.PathValue("name"), req.Content)
	}))
	// Render: delete => 204.
	mux.HandleFunc("DELETE "+base+"/{id}/secret-files/{name}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteSecretFile(r.Context(), r.PathValue("id"), r.PathValue("name"))
	}))
}

// listPagedOrAll routes a list read to its paged or unpaged verb. Callers that
// omit both cursor and limit predate pagination and still get the complete
// list, so the absence of the parameters — not their values — picks the verb.
func listPagedOrAll[T any](r *http.Request, page func(context.Context, string, string, int) ([]T, error), all func(context.Context, string) ([]T, error)) ([]T, error) {
	q := r.URL.Query()
	if !q.Has("cursor") && !q.Has("limit") {
		return all(r.Context(), r.PathValue("id"))
	}
	after, limit := core.PageParams(q)
	return page(r.Context(), r.PathValue("id"), after, limit)
}
