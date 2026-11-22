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

package sandbox

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	// CodeSandboxCredentialInvalid refuses malformed sandbox group credential inputs.
	CodeSandboxCredentialInvalid = "SANDBOX_CREDENTIAL_INVALID"
	// CodeSandboxCredentialNotFound is returned when a credential does not exist
	// in the target sandbox group.
	CodeSandboxCredentialNotFound = "SANDBOX_CREDENTIAL_NOT_FOUND"
	// CodeSandboxCredentialVersionNotFound is returned when a requested version
	// ID does not exist or has been evicted from retention.
	CodeSandboxCredentialVersionNotFound = "SANDBOX_CREDENTIAL_VERSION_NOT_FOUND"
	// CodeSandboxCredentialExists is returned (409) when creating a credential
	// whose case-sensitive ID already exists in the sandbox group.
	CodeSandboxCredentialExists = "SANDBOX_CREDENTIAL_ALREADY_EXISTS"
	// CodeSandboxCredentialBusy is returned (429) on temporary write contention
	// on the sandbox group lock.
	CodeSandboxCredentialBusy = "SANDBOX_CREDENTIAL_CONFLICT"

	// MaxSandboxCredentialValueBytes is Render's 5 KiB (5120 UTF-8 bytes) secret limit.
	MaxSandboxCredentialValueBytes = 5120

	credentialTemplatePlaceholder = "${SECRET}"
	maxCredentialTemplateCount    = 5
)

const (
	CredentialInjectionHeader = "header"
	CredentialInjectionBasic  = "basic"
	CredentialInjectionCookie = "cookie"
	CredentialInjectionQuery  = "query"
)

var (
	credentialIDPattern     = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
	credentialDomainPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$`)
	httpTokenPattern        = regexp.MustCompile("^[!#$%&'*+\\-.^_`|~0-9A-Za-z]+$")

	supportedCredentialMethods = map[string]struct{}{
		"GET": {}, "HEAD": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {}, "OPTIONS": {},
	}

	// forbiddenCredentialHeaders are HTTP routing, framing, hop-by-hop, and
	// proxy headers that cannot be set by credential injection (ADR122 D1).
	forbiddenCredentialHeaders = map[string]struct{}{
		"host":                {},
		"connection":          {},
		"keep-alive":          {},
		"proxy-authenticate":  {},
		"proxy-authorization": {},
		"proxy-connection":    {},
		"te":                  {},
		"trailer":             {},
		"transfer-encoding":   {},
		"upgrade":             {},
		"content-length":      {},
		"forwarded":           {},
		"via":                 {},
	}
)

func credBadRequest(field, msg string) error {
	return core.NewBadRequestError(CodeSandboxCredentialInvalid, msg, map[string]any{"field": field})
}

// ValidateCredentialID enforces Render's credential ID rules:
// ^[A-Za-z0-9._~-]+$, minLength 1, case-sensitive, forbidding ".", "..", and "rnd-*".
func ValidateCredentialID(id string) error {
	if id == "" || !credentialIDPattern.MatchString(id) {
		return credBadRequest("id", "credential id must be non-empty and use only letters, numbers, periods, underscores, tildes, and hyphens")
	}
	if id == "." || id == ".." {
		return credBadRequest("id", fmt.Sprintf("credential id %q is reserved", id))
	}
	if strings.HasPrefix(id, "rnd-") {
		return credBadRequest("id", `credential ids starting with "rnd-" are reserved`)
	}
	return nil
}

// NormalizeCredentialDomain validates and lowercases a destination hostname.
// Must be a public multi-label DNS hostname; rejects IP literals, ports,
// wildcards, schemes, paths, and local/private/cluster hostnames.
func NormalizeCredentialDomain(domain string) (string, error) {
	if domain == "" {
		return "", credBadRequest("domain", "domain is required")
	}
	if strings.ContainsAny(domain, ":/?#@[]* \t\r\n") {
		return "", credBadRequest("domain", "domain must be a public DNS hostname without wildcards, ports, schemes, or paths")
	}
	d := strings.ToLower(domain)
	if len(d) > 253 {
		return "", credBadRequest("domain", "domain must be at most 253 characters")
	}
	if net.ParseIP(d) != nil {
		return "", credBadRequest("domain", "IP addresses are not supported; provide a public DNS hostname")
	}
	if !credentialDomainPattern.MatchString(d) || len(validation.IsDNS1123Subdomain(d)) != 0 {
		return "", credBadRequest("domain", "domain must be a valid public DNS hostname")
	}
	if isPrivateOrNonPublicCredentialHost(d) {
		return "", credBadRequest("domain", "local, private, and cluster hostnames are not supported")
	}
	return d, nil
}

func isPrivateOrNonPublicCredentialHost(host string) bool {
	if sessionegress.PrivateOrClusterHost(host) {
		return true
	}
	for _, suffix := range []string{".svc", ".home.arpa", ".arpa", ".localdomain", ".lan", ".corp", ".home"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	// Reject all-numeric TLDs (catches non-standard numeric IPv4 forms).
	if idx := strings.LastIndexByte(host, '.'); idx >= 0 {
		tld := host[idx+1:]
		allDigits := len(tld) > 0
		for i := range len(tld) {
			if tld[i] < '0' || tld[i] > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return true
		}
	}
	return false
}

// ValidateCredentialValue enforces Render's 5 KiB UTF-8 byte bound without
// ever echoing the secret value in error messages.
func ValidateCredentialValue(value string) error {
	if value == "" {
		return credBadRequest("value", "secret value must not be empty")
	}
	if !utf8.ValidString(value) {
		return credBadRequest("value", "secret value must be valid UTF-8")
	}
	if len(value) > MaxSandboxCredentialValueBytes {
		return credBadRequest("value", fmt.Sprintf("secret value exceeds maximum size of %d bytes", MaxSandboxCredentialValueBytes))
	}
	if strings.ContainsRune(value, 0) {
		return credBadRequest("value", "secret value must not contain NUL bytes")
	}
	return nil
}

// NormalizeCredentialMethods validates, uppercases, deduplicates, and sorts
// HTTP methods. Empty input returns an empty non-nil slice.
func NormalizeCredentialMethods(methods []string) ([]string, error) {
	if len(methods) == 0 {
		return []string{}, nil
	}
	out := make([]string, 0, len(methods))
	seen := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		up := strings.ToUpper(m)
		if _, ok := supportedCredentialMethods[up]; !ok {
			return nil, credBadRequest("methods", fmt.Sprintf("unsupported HTTP method %q; use GET, HEAD, POST, PUT, PATCH, DELETE, or OPTIONS", m))
		}
		if _, dup := seen[up]; dup {
			continue
		}
		seen[up] = struct{}{}
		out = append(out, up)
	}
	slices.Sort(out)
	return out, nil
}

// NormalizeCredentialPaths validates path patterns per Render's contract:
// absolute path starting with "/", "*" for one nonempty segment, "**" for one
// or more segments, whole-segment wildcards only, no "?", "#", "\", or control
// characters.
func NormalizeCredentialPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return []string{}, nil
	}
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if err := validateCredentialPathPattern(p); err != nil {
			return nil, err
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	slices.Sort(out)
	return out, nil
}

func validateCredentialPathPattern(p string) error {
	if p == "" || p[0] != '/' {
		return credBadRequest("paths", "each path pattern must be an absolute path starting with /")
	}
	if strings.ContainsAny(p, "?#\\") {
		return credBadRequest("paths", "path patterns do not support query strings, fragments, or escaping")
	}
	if strings.ContainsFunc(p, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return credBadRequest("paths", "path patterns must not contain whitespace or control characters")
	}
	if p == "/" {
		return nil
	}
	for _, seg := range strings.Split(p[1:], "/") {
		if strings.Contains(seg, "*") && seg != "*" && seg != "**" {
			return credBadRequest("paths", fmt.Sprintf("wildcard in path pattern %q must occupy a whole segment (* or **)", p))
		}
	}
	return nil
}

// ValidateCredentialInjection validates injection settings and checks the
// rendered secret value (never echoing the secret in any error).
func ValidateCredentialInjection(inj *GroupCredentialInjection, secretValue string) (store.SandboxCredentialInjection, error) {
	if inj == nil {
		return store.SandboxCredentialInjection{}, credBadRequest("injection", "injection settings are required")
	}
	switch inj.Type {
	case CredentialInjectionHeader:
		if inj.Name == "" || !httpTokenPattern.MatchString(inj.Name) {
			return store.SandboxCredentialInjection{}, credBadRequest("injection.name", "header injection requires a valid HTTP header name")
		}
		lower := strings.ToLower(inj.Name)
		if _, forbidden := forbiddenCredentialHeaders[lower]; forbidden || strings.HasPrefix(lower, "x-forwarded-") {
			return store.SandboxCredentialInjection{}, credBadRequest("injection.name", fmt.Sprintf("header %q controls routing or connection behavior and cannot be injected", inj.Name))
		}
	case CredentialInjectionBasic:
		if inj.Name != "" {
			return store.SandboxCredentialInjection{}, credBadRequest("injection.name", "basic injection must not specify a name")
		}
	case CredentialInjectionCookie:
		if inj.Name == "" || !httpTokenPattern.MatchString(inj.Name) {
			return store.SandboxCredentialInjection{}, credBadRequest("injection.name", "cookie injection requires a valid cookie name")
		}
	case CredentialInjectionQuery:
		if inj.Name == "" || strings.ContainsFunc(inj.Name, func(r rune) bool { return unicode.IsControl(r) || r == '#' }) {
			return store.SandboxCredentialInjection{}, credBadRequest("injection.name", "query injection requires a valid query parameter name")
		}
	default:
		return store.SandboxCredentialInjection{}, credBadRequest("injection.type", "injection.type must be one of header, basic, cookie, or query")
	}

	effectiveTemplate := credentialTemplatePlaceholder
	if inj.Template != "" {
		if !utf8.ValidString(inj.Template) || strings.ContainsRune(inj.Template, 0) {
			return store.SandboxCredentialInjection{}, credBadRequest("injection.template", "injection.template must be valid UTF-8 with no NUL bytes")
		}
		count := strings.Count(inj.Template, credentialTemplatePlaceholder)
		if count < 1 || count > maxCredentialTemplateCount {
			return store.SandboxCredentialInjection{}, credBadRequest("injection.template",
				fmt.Sprintf("injection.template must contain between 1 and %d occurrences of %s", maxCredentialTemplateCount, credentialTemplatePlaceholder))
		}
		effectiveTemplate = inj.Template
	}

	rendered := strings.ReplaceAll(effectiveTemplate, credentialTemplatePlaceholder, secretValue)
	switch inj.Type {
	case CredentialInjectionHeader:
		if hasInvalidHeaderValueChar(rendered) {
			return store.SandboxCredentialInjection{}, credBadRequest("value", "rendered header value must not contain control characters")
		}
	case CredentialInjectionCookie:
		if hasInvalidCookieValueChar(rendered) {
			return store.SandboxCredentialInjection{}, credBadRequest("value", "rendered cookie value must not contain control characters or semicolons")
		}
	case CredentialInjectionBasic:
		if strings.ContainsAny(rendered, "\r\n\x00") {
			return store.SandboxCredentialInjection{}, credBadRequest("value", "rendered basic credential must not contain CR, LF, or NUL bytes")
		}
		user, pass, ok := strings.Cut(rendered, ":")
		if !ok || user == "" || pass == "" {
			return store.SandboxCredentialInjection{}, credBadRequest("value", "for basic injection, the value after applying template must have the form username:password")
		}
	}

	return store.SandboxCredentialInjection{
		Type:     inj.Type,
		Name:     inj.Name,
		Template: inj.Template,
	}, nil
}

func hasInvalidHeaderValueChar(s string) bool {
	for i := range len(s) {
		b := s[i]
		if (b < 0x20 && b != '\t') || b == 0x7f {
			return true
		}
	}
	return false
}

func hasInvalidCookieValueChar(s string) bool {
	for i := range len(s) {
		b := s[i]
		if b < 0x20 || b == 0x7f || b == ';' {
			return true
		}
	}
	return false
}
