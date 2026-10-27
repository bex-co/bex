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
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
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
	// rule, when set, is the wording a refusal names instead of Kubernetes'.
	rule string
}

var (
	envVarKey     = configKeyKind{noun: "environment variable", code: "ENVIRONMENT_VARIABLE_INVALID", field: "key", rule: EnvKeyRule}
	secretFileKey = configKeyKind{noun: "secret file", code: "SECRET_FILE_INVALID", field: "name", rule: SecretFileNameRule}
)

// EnvKeyRule is the one wording of what an environment variable may be named:
// a Kubernetes Secret key. Render's own templates use dotted names such as
// Elasticsearch's discovery.type, so bex no longer requires a C identifier
// (w5/m150); a native build's shell skips a name it cannot export.
var EnvKeyRule = fmt.Sprintf("letters, digits, '-', '_' and '.', at most %d characters", validation.DNS1123SubdomainMaxLength)

// SecretFileNameRule is EnvKeyRule's twin for secret files: the same
// Kubernetes Secret-key rule, worded in bex's own terms so a refusal never
// leaks the config-key regex (w4/403).
var SecretFileNameRule = EnvKeyRule

// problem names the rule key breaks, or "" when it is a valid name.
func (k configKeyKind) problem(key string) string {
	errs := validation.IsConfigMapKey(key)
	if len(errs) == 0 {
		return ""
	}
	// The kubelet's atomic-writer layout owns ".."-prefixed names; that
	// specific refusal keeps its own wording under either kind's rule.
	if strings.HasPrefix(key, "..") {
		return "must not start with '..'"
	}
	if k.rule != "" {
		return "use " + k.rule
	}
	return strings.Join(errs, "; ")
}

// refuse is the coded 400 for a key problem rejects, naming the rule it broke.
// An over-long key is not echoed: the length is the whole story.
func (k configKeyKind) refuse(key string) error {
	if len(key) > validation.DNS1123SubdomainMaxLength {
		return NewBadRequestError(k.code, fmt.Sprintf("%s name is longer than %d characters", k.noun, validation.DNS1123SubdomainMaxLength), k.params())
	}
	return NewBadRequestError(k.code, fmt.Sprintf("invalid %s name %q: %s", k.noun, key, k.problem(key)), k.params())
}

// projectionRefused is the coded 400 for a write Kubernetes refused to project
// over key, for the API server's reason rule; the write was rolled back. A key
// this kind's own rule rejects is refused by that rule instead. key or rule is
// "" when the refusal named none.
func (k configKeyKind) projectionRefused(key, rule string) error {
	if key != "" && k.problem(key) != "" {
		return k.refuse(key)
	}
	subject := "the change"
	if key != "" {
		subject = fmt.Sprintf("%s %q", k.noun, key)
	}
	if rule != "" {
		rule = ": " + rule
	}
	return NewBadRequestError(k.code, subject+" cannot be applied to the service"+rule+"; it was not saved", k.params())
}

func (k configKeyKind) params() map[string]any {
	return map[string]any{"field": k.field, "maxLength": validation.DNS1123SubdomainMaxLength}
}

// ValidEnvKey reports whether k is an environment variable name bex accepts: a
// valid Secret key. Rejecting the rest keeps a bad name from failing the Secret
// write with a cryptic error later.
func ValidEnvKey(k string) bool {
	return envVarKey.problem(k) == ""
}

// CheckEnvKey is the single gate for an environment variable name a caller
// wants to set: it must be well-formed. Every write path funnels through it.
// No name is reserved: PORT, like on Render, is the service's to set
// (w5/m149).
func CheckEnvKey(k string) error {
	if !ValidEnvKey(k) {
		// Names only in the error — never the value (docs/ADR013-secrets.md).
		return envVarKey.refuse(k)
	}
	return nil
}

// ValidEnvValue reports whether a process environment can carry v: a POSIX
// environment string ends at its first NUL (w8/m62, docs/ADR004-app-deployment.md).
func ValidEnvValue(v string) bool {
	return strings.IndexByte(v, 0) < 0
}

// EnvValueSentence is the one wording for a value ValidEnvValue rejects. It
// names the key, never the value (docs/ADR013-secrets.md).
func EnvValueSentence(k string) string {
	return fmt.Sprintf("environment variable %q contains a NUL byte, which a process environment cannot carry; use a secret file for binary content", k)
}

// MaxEnvEntryBytes is Linux's MAX_ARG_STRLEN: execve refuses a process whose
// environment holds one KEY=value string, NUL terminator included, longer than
// this, so a service carrying one could never start an instance (w4/422).
const MaxEnvEntryBytes = 131072

// CheckEnvValue is the gate for a value a caller newly supplies. Only new
// values are checked, so a legacy stored value stays replaceable/deletable.
func CheckEnvValue(k, value string) error {
	if !ValidEnvValue(value) {
		return NewBadRequestError("ENVIRONMENT_VARIABLE_VALUE_INVALID", EnvValueSentence(k), map[string]any{"key": k})
	}
	if len(k)+len("=")+len(value)+len("\x00") > MaxEnvEntryBytes {
		return NewBadRequestError("ENVIRONMENT_VARIABLE_TOO_LARGE",
			fmt.Sprintf("environment variable %q is too large: a process cannot start with a KEY=value entry over %d bytes; use a secret file for large payloads", k, MaxEnvEntryBytes-1),
			map[string]any{"key": k, "maxBytes": MaxEnvEntryBytes - 1})
	}
	if k == portEnvKey && !ValidPortValue(value) {
		return NewBadRequestError("ENVIRONMENT_VARIABLE_PORT_INVALID", PortValueSentence, map[string]any{"key": k})
	}
	return nil
}

// portEnvKey is the variable that selects a service's serving port (Render
// lets a service set it, w5/m149).
const portEnvKey = "PORT"

// PortValueSentence is the one wording for a PORT value ValidPortValue rejects.
// It never echoes the value.
const PortValueSentence = "PORT must be an integer from 1 to 65535 (not 18012, 18013, 18014 or 19099, which the platform reserves); the operator would otherwise ignore it and serve the service's own port"

// ValidPortValue reports whether v is a PORT the operator will serve on:
// digits only, 1-65535, not a platform-reserved port (the operator's own rule,
// env_port.go).
func ValidPortValue(v string) bool {
	port, err := strconv.Atoi(v)
	return err == nil && port >= 1 && port <= 65535 && !appv1alpha1.IsReservedImagePort(int32(port))
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

// EnvKeyProjectionRefused and SecretFileProjectionRefused answer Kubernetes
// refusing a service's env or files Secret over key, for rule: the same code
// and params as the name's own refusal.
func EnvKeyProjectionRefused(key, rule string) error {
	return envVarKey.projectionRefused(key, rule)
}

func SecretFileProjectionRefused(key, rule string) error {
	return secretFileKey.projectionRefused(key, rule)
}
