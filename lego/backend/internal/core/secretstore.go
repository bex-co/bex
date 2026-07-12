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

// SecretKV is the shared seam to the versioned secret backend — OpenBao KV v2 in
// production (secrets.NewOpenBaoStore), a fake in tests. Both config features read
// and write string maps at tenant-scoped logical paths through it: the env-vars +
// secret-files feature (internal/secrets) at "services/<service>/{env,files}" and
// the env-groups feature (internal/envgroups) at "env-groups/<id>/{meta,env,files}".
// The concrete store prepends the tenant prefix (docs/ADR013-secrets.md §4) and translates
// each verb to KV v2. Keeping the interface here (not in a feature) lets both
// features share ONE store instance with no feature-to-feature import. nil => the
// owning feature's verbs report their "…Unavailable" sentinel.
//
// A path is a "/"-joined logical key WITHOUT the mount or tenant prefix. Get on a
// never-written path returns an empty map (not an error); Delete of an absent path
// is a no-op; List returns the immediate child keys under a path (empty when none).
type SecretKV interface {
	Get(ctx context.Context, path string) (map[string]string, error)
	Put(ctx context.Context, path string, data map[string]string) error
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, path string) ([]string, error)
}
