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

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"
	"sort"
)

// config.go holds the validation rules shared by the two config features
// (internal/secrets env vars + secret files, internal/envgroups) — both write
// into per-app Kubernetes Secrets, so both must reject names that aren't legal
// Secret keys before the write reaches the apiserver with a cryptic error.

// GenerateValue mints a cryptographically random secret value in Render's
// documented generateValue shape: a base64-encoded 256-bit value (32 random
// bytes → 44-char standard-base64 string, e.g. "B0jrphAPOY7pg…KFUk="). Shared by
// the two features that honor render.yaml's generateValue — env vars
// (internal/secrets) and env groups (internal/envgroups) — so both mint the same
// format. A rand.Read failure is surfaced, never silently substituted.
func GenerateValue() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}

// SortedKeys returns a string-map's keys in sorted order — the deterministic
// iteration order the two config features (secrets seeding, env-group apply) rely
// on so a blueprint seed's minting order (and any rand failure) is reproducible.
func SortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ValidEnvKey reports whether k is a C-locale environment variable name
// ([A-Za-z_][A-Za-z0-9_]*): what a shell and Kubernetes' Secret-key validation
// both accept. Rejecting the rest keeps a bad name from failing the Secret write
// with a cryptic error later.
func ValidEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// ReservedEnvKeys are environment variable names bex owns, so a user write of
// one is refused instead of stored-and-ignored.
//
// Today that is exactly PORT. The operator strips a user PORT out of spec.env
// and appends its own container-level PORT entry, which wins over every envFrom
// key (lego/operator/internal/controller/app_controller.go appEnv,
// docs/ADR004-app-deployment.md). That injection happens for **every** App type,
// not just web services, so the rule is reserved-everywhere: one rule, no
// per-type exception to remember. Render diverges here — it lets you override
// PORT — and docs/ADR018-render-parity.md records that.
//
// A reserved key is refused only on a **write**. Deleting one still works, so a
// service or group that already stored a PORT before this rule can be cleaned
// up through the ordinary delete verbs.
var ReservedEnvKeys = []string{"PORT"}

// IsReservedEnvKey reports whether k is a name bex owns. The match is exact and
// case-sensitive: environment variables are case-sensitive, and a lowercase
// "port" is an ordinary application variable the operator never touches.
func IsReservedEnvKey(k string) bool {
	return slices.Contains(ReservedEnvKeys, k)
}

// ReservedEnvKeySentence is the one wording for the refusal. Every surface —
// REST, GraphQL, MCP, Blueprint validation, and the dashboard's inline hint —
// renders this exact string, so a caller who learns it on one surface
// recognizes it on the next.
//
// It now names WHERE. Until w4/m121 it ended at "change the service port
// instead", and the service port was create-only: no update verb on any
// surface, no dashboard control, and unreadable even on GraphQL. The sentence
// sent every caller to a setting that did not exist. The port field and its
// verb are named explicitly so the instruction is followable from the text
// alone, on a surface that renders strings rather than links.
func ReservedEnvKeySentence(k string) string {
	return fmt.Sprintf("environment variable %q is reserved: bex sets it from the service port; change the service's port field instead (REST PATCH /v1/services/{id} port, GraphQL setPort, MCP update_service port, or Settings in the dashboard)", k)
}

// ReservedEnvKeyError is the coded refusal every environment write path
// returns. The Blueprint parser is the one exception: it wraps the sentence
// with the manifest location instead, because its validation surface renders
// error strings rather than codes — see parseServiceEnv and parseEnvGroup.
func ReservedEnvKeyError(k string) error {
	return NewBadRequestError("ENVIRONMENT_VARIABLE_RESERVED", ReservedEnvKeySentence(k), nil)
}

// CheckEnvKey is the single gate for an environment variable name a caller
// wants to set: it must be well-formed and must not be one bex owns. Every
// write path funnels through it so the two refusals cannot drift apart.
func CheckEnvKey(k string) error {
	if !ValidEnvKey(k) {
		// Names only in the error — never the value (docs/ADR013-secrets.md).
		return fmt.Errorf("%w: invalid environment variable name %q", ErrBadRequest, k)
	}
	if IsReservedEnvKey(k) {
		return ReservedEnvKeyError(k)
	}
	return nil
}

// ValidSecretFileName reports whether name is a legal Kubernetes Secret key
// ([-._a-zA-Z0-9]+, not "."/".."): the file is mounted at /etc/secrets/<name>, so
// a name outside this set (a path, in particular) would fail the Secret write.
func ValidSecretFileName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		switch {
		case r == '_', r == '-', r == '.':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
