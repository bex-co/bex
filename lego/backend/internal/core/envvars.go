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

package core

import "context"

// EnvVar is the neutral env-var projection the apps GraphQL surface renders
// nested under a Service (Render dashboard shape: `service{ envVarKeys{ id key } }`).
// The secrets feature (internal/secrets) produces these; apps' GraphQL consumes
// them through the EnvVarReader seam below, so neither feature imports the other —
// the composition root injects the reader into the request context. bex has no
// separate id per variable (the key is unique within a service), so ID == Key.
type EnvVar struct {
	ID    string
	Key   string
	Value string
}

// EnvVarReader is the seam apps' Service GraphQL type uses to nest a service's
// env vars: EnvVarKeys lists keys only (value empty — Render fetches values on
// demand, "Show secret"), EnvVarValue reads one variable's value. The secrets
// Service satisfies it. This lives in the kernel (not apps or secrets) so the
// nesting needs no feature-to-feature import.
type EnvVarReader interface {
	EnvVarKeys(ctx context.Context, service string) ([]EnvVar, error)
	EnvVarValue(ctx context.Context, service, key string) (EnvVar, error)
}

type envVarReaderCtxKey struct{}

// WithEnvVars returns ctx carrying the env-var reader — the composition root's
// setter, so apps' GraphQL resolvers reach the secrets feature via context
// instead of a closure (the shared Service GraphQL type stays stateless).
func WithEnvVars(ctx context.Context, r EnvVarReader) context.Context {
	return context.WithValue(ctx, envVarReaderCtxKey{}, r)
}

// EnvVarsFrom returns the env-var reader the root attached (false when none is
// wired — the env-var fields then report ErrSecretsUnavailable).
func EnvVarsFrom(ctx context.Context) (EnvVarReader, bool) {
	r, ok := ctx.Value(envVarReaderCtxKey{}).(EnvVarReader)
	return r, ok && r != nil
}
