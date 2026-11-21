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
	"fmt"
	"sync/atomic"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// gqlMaxNestedReads caps how many times one document reads a service's env
// vars, secret files or outbound IPs through the fields the Service type
// nests (envVarKeys, envVar, secretFileNames, secretFile, outboundIps).
//
// Each such read goes to the Kubernetes API (and the secret store) once per
// service it is selected on, so a services list multiplies it by its page:
// a 2 KB document selecting the five fields eleven times each under
// services(limit: 100) made 5,500 reads and 12,000 Kubernetes API calls for
// one rate-limit token, 3 s against an in-memory fake API server, where a
// real one takes milliseconds a call. The cost gate cannot see that
// multiplier, which is the data's. bex's clients read these fields only
// under service(id), once a document (2026-11-20).
const gqlMaxNestedReads = 20

// errNestedReads answers each nested read past gqlMaxNestedReads.
var errNestedReads = core.NewBadRequestError("NESTED_READ_LIMIT",
	fmt.Sprintf("query rejected: reads services' env vars, secret files or outbound IPs more than %d times; read them for one service at a time", gqlMaxNestedReads),
	map[string]any{"limit": gqlMaxNestedReads})

// nestedReads is the env-var, secret-file and outbound-IP reader one document
// executes with: each read draws on the document's gqlMaxNestedReads, and a
// read past it is refused without reaching the reader.
type nestedReads struct {
	envVars     core.EnvVarReader
	secretFiles core.SecretFileReader
	outboundIPs core.OutboundIPReader
	// spent counts the reads, refused ones included; atomic, so resolvers
	// that read in parallel share it safely.
	spent atomic.Int32
}

func (r *nestedReads) spend() error {
	if r.spent.Add(1) > gqlMaxNestedReads {
		return errNestedReads
	}
	return nil
}

func (r *nestedReads) EnvVarKeys(ctx context.Context, service string) ([]core.EnvVar, error) {
	if err := r.spend(); err != nil {
		return nil, err
	}
	return r.envVars.EnvVarKeys(ctx, service)
}

func (r *nestedReads) EnvVarValue(ctx context.Context, service, key string) (core.EnvVar, error) {
	if err := r.spend(); err != nil {
		return core.EnvVar{}, err
	}
	return r.envVars.EnvVarValue(ctx, service, key)
}

func (r *nestedReads) SecretFileNames(ctx context.Context, service string) ([]core.SecretFile, error) {
	if err := r.spend(); err != nil {
		return nil, err
	}
	return r.secretFiles.SecretFileNames(ctx, service)
}

func (r *nestedReads) SecretFileContent(ctx context.Context, service, name string) (core.SecretFile, error) {
	if err := r.spend(); err != nil {
		return core.SecretFile{}, err
	}
	return r.secretFiles.SecretFileContent(ctx, service, name)
}

func (r *nestedReads) OutboundIPs(ctx context.Context, service string) (core.OutboundIPs, error) {
	if err := r.spend(); err != nil {
		return core.OutboundIPs{}, err
	}
	return r.outboundIPs.OutboundIPs(ctx, service)
}
