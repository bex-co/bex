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

// SecretFile is the neutral secret-file projection the apps GraphQL surface
// renders nested under a Service (Render dashboard shape, mirroring env vars:
// `service{ secretFileNames{ id name } }`). The secrets feature produces these;
// apps' GraphQL consumes them through the SecretFileReader seam below, so neither
// feature imports the other. bex has no separate id per file (the name is unique
// within a service), so ID == Name.
type SecretFile struct {
	ID      string
	Name    string
	Content string
}

// SecretFileReader is the seam apps' Service GraphQL type uses to nest a service's
// secret files: SecretFileNames lists names only (content empty — fetched on
// demand, like env-var "Show secret"), SecretFileContent reads one file. The
// secrets Service satisfies it. Lives in the kernel so the nesting needs no
// feature-to-feature import.
type SecretFileReader interface {
	SecretFileNames(ctx context.Context, service string) ([]SecretFile, error)
	SecretFileContent(ctx context.Context, service, name string) (SecretFile, error)
}

type secretFileReaderCtxKey struct{}

// WithSecretFiles returns ctx carrying the secret-file reader — the composition
// root's setter, so apps' GraphQL resolvers reach the secrets feature via context
// (the shared Service GraphQL type stays stateless).
func WithSecretFiles(ctx context.Context, r SecretFileReader) context.Context {
	return context.WithValue(ctx, secretFileReaderCtxKey{}, r)
}

// SecretFilesFrom returns the secret-file reader the root attached (false when
// none is wired — the secret-file fields then report ErrSecretsUnavailable).
func SecretFilesFrom(ctx context.Context) (SecretFileReader, bool) {
	r, ok := ctx.Value(secretFileReaderCtxKey{}).(SecretFileReader)
	return r, ok && r != nil
}
