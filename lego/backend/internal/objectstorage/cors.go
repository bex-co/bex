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

package objectstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// CORSRule is one rule of a bucket's CORS configuration in the shape the bucket
// row stores (Cloudflare's, which the removed Cloudflare-shaped surface read and
// wrote; the S3 gateway converts to and from it). bex applies the rules itself where it
// serves a bucket to browsers (a public bucket's host, ADR112 D4); Wasabi's own
// CORS stays off.
type CORSRule struct {
	ID            string      `json:"id,omitempty"`
	Allowed       CORSAllowed `json:"allowed"`
	ExposeHeaders []string    `json:"exposeHeaders,omitempty"`
	MaxAgeSeconds *int64      `json:"maxAgeSeconds,omitempty"`
}

// CORSAllowed is what a rule lets a cross-origin caller do.
type CORSAllowed struct {
	Origins []string `json:"origins"`
	Methods []string `json:"methods"`
	Headers []string `json:"headers,omitempty"`
}

var (
	// ErrCORSOverCap refuses a rule set over MaxCORSRules or MaxCORSBytes.
	ErrCORSOverCap = errors.New("the CORS configuration is over bex's limit")
	// ErrInvalidCORS refuses a rule that breaks the stored shape.
	ErrInvalidCORS = errors.New("invalid CORS rule")
)

// CORSMethods are the methods a rule may allow.
var CORSMethods = []string{"GET", "PUT", "POST", "DELETE", "HEAD"}

// MaxCORSMaxAgeSeconds is the largest preflight cache time a rule may ask for:
// the maximum of the stored shape (browsers cap it lower).
const MaxCORSMaxAgeSeconds = 86400

const maxCORSTokenBytes = 256

// ParseCORS reads a stored or submitted rule set. It ignores fields it does not
// know, and treats nothing at all as no rules.
func ParseCORS(raw json.RawMessage) ([]CORSRule, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var rules []CORSRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("the CORS rules are not a list of rules: %w", err)
	}
	return rules, nil
}

// ValidateCORS checks a rule set against bex's limits and the stored shape:
// at most MaxCORSRules rules in at most MaxCORSBytes, each with an origin and a
// method, origins that are "*" or a scheme and host, known methods, header
// names without spaces, and a max age of 0 to 86400 seconds. A breach of a limit is
// ErrCORSOverCap; anything else malformed is an ErrInvalidCORS naming the rule.
func ValidateCORS(rules []CORSRule) error {
	if len(rules) > MaxCORSRules {
		return fmt.Errorf("%w: %d rules, at most %d", ErrCORSOverCap, len(rules), MaxCORSRules)
	}
	if raw, err := json.Marshal(rules); err != nil || len(raw) > MaxCORSBytes {
		return fmt.Errorf("%w: at most %d bytes", ErrCORSOverCap, MaxCORSBytes)
	}
	for i, rule := range rules {
		at := func(format string, args ...any) error {
			return fmt.Errorf("%w: CORS rule %d: %s", ErrInvalidCORS, i+1, fmt.Sprintf(format, args...))
		}
		if len(rule.Allowed.Origins) == 0 {
			return at("allowed.origins needs at least one origin")
		}
		for _, o := range rule.Allowed.Origins {
			if err := validOrigin(o); err != nil {
				return at("%v", err)
			}
		}
		if len(rule.Allowed.Methods) == 0 {
			return at("allowed.methods needs at least one method")
		}
		for _, m := range rule.Allowed.Methods {
			if !slices.Contains(CORSMethods, m) {
				return at("method %q is not one of %s", m, strings.Join(CORSMethods, ", "))
			}
		}
		for _, h := range slices.Concat(rule.Allowed.Headers, rule.ExposeHeaders) {
			if h == "" || len(h) > maxCORSTokenBytes || strings.ContainsAny(h, " \t\r\n,") {
				return at("header name %q is not valid", h)
			}
		}
		if rule.MaxAgeSeconds != nil && (*rule.MaxAgeSeconds < 0 || *rule.MaxAgeSeconds > MaxCORSMaxAgeSeconds) {
			return at("maxAgeSeconds must be between 0 and %d", MaxCORSMaxAgeSeconds)
		}
	}
	return nil
}

// validOrigin accepts "*" or a scheme, a host and an optional port, with at
// most one "*" (a leading subdomain wildcard such as https://*.example.com).
func validOrigin(o string) error {
	if o == "*" {
		return nil
	}
	rest, ok := strings.CutPrefix(o, "https://")
	if !ok {
		rest, ok = strings.CutPrefix(o, "http://")
	}
	if !ok || rest == "" || len(o) > maxCORSTokenBytes || strings.ContainsAny(rest, "/ \t\r\n?#") || strings.Count(o, "*") > 1 {
		return fmt.Errorf("origin %q must be * or a scheme and host such as https://example.com", o)
	}
	return nil
}
