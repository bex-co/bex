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
	"strings"

	"k8s.io/apimachinery/pkg/api/validate/content"
	"k8s.io/apimachinery/pkg/util/validation"
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

// configKeyKind is one of the two names bex projects into a Kubernetes
// Secret's keys, and so validates by Kubernetes' own Secret-key rule
// (validation.IsConfigMapKey) rather than a copy of it: a copy let "..data"
// through, which the store accepted and the projection then refused (w5/m118),
// and a name over 253 characters wedged a service's environment (w4/m168).
type configKeyKind struct {
	noun, code, field string
	// identifier also holds the key to a C identifier, what a shell reads.
	identifier bool
	// reserved reports the names bex owns; nil when the kind has none.
	reserved func(string) bool
}

var (
	envVarKey = configKeyKind{noun: "environment variable", code: "ENVIRONMENT_VARIABLE_INVALID", field: "key",
		identifier: true, reserved: IsReservedEnvKey}
	// Secret files have no reserved names: a file called PORT is not an
	// environment variable, and the operator never injects one.
	secretFileKey = configKeyKind{noun: "secret file", code: "SECRET_FILE_INVALID", field: "name"}
)

// EnvKeyRule is the one wording of what an environment variable may be named.
var EnvKeyRule = fmt.Sprintf("letters, digits and underscores, not starting with a digit, at most %d characters", validation.DNS1123SubdomainMaxLength)

// problem names the rule key breaks, or "" when it is a valid name.
func (k configKeyKind) problem(key string) string {
	if k.identifier && len(content.IsCIdentifier(key)) > 0 {
		return "use " + EnvKeyRule
	}
	return strings.Join(validation.IsConfigMapKey(key), "; ")
}

// refuse is the coded 400 for a key problem rejects, naming the rule it broke.
// An over-long key is not echoed: the length is the whole story.
func (k configKeyKind) refuse(key string) error {
	params := map[string]any{"field": k.field, "maxLength": validation.DNS1123SubdomainMaxLength}
	if len(key) > validation.DNS1123SubdomainMaxLength {
		return NewBadRequestError(k.code, fmt.Sprintf("%s name is longer than %d characters", k.noun, validation.DNS1123SubdomainMaxLength), params)
	}
	return NewBadRequestError(k.code, fmt.Sprintf("invalid %s name %q: %s", k.noun, key, k.problem(key)), params)
}


// ValidEnvKey reports whether k is an environment variable name bex accepts: a
// C identifier a shell reads that is also a valid Secret key. Rejecting the
// rest keeps a bad name from failing the Secret write with a cryptic error
// later.
func ValidEnvKey(k string) bool {
	return envVarKey.problem(k) == ""
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
		return envVarKey.refuse(k)
	}
	if IsReservedEnvKey(k) {
		return ReservedEnvKeyError(k)
	}
	return nil
}

// ValidSecretFileName reports whether name is a legal Kubernetes Secret key:
// the file is mounted at /etc/secrets/<name>, so a name outside the rule (a
// path, or "..data", which the kubelet's atomic-writer layout owns) would fail
// the Secret write.
func ValidSecretFileName(name string) bool {
	return secretFileKey.problem(name) == ""
}

// CheckSecretFileName is CheckEnvKey's twin for secret files: the one refusal
// every secret-file write returns for a name ValidSecretFileName rejects.
func CheckSecretFileName(name string) error {
	if !ValidSecretFileName(name) {
		return secretFileKey.refuse(name)
	}
	return nil
}
