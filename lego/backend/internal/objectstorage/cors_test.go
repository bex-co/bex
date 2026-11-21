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

package objectstorage_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

func rule(origins []string, methods []string) objectstorage.CORSRule {
	return objectstorage.CORSRule{Allowed: objectstorage.CORSAllowed{Origins: origins, Methods: methods}}
}

func TestValidateCORSAcceptsTheStoredShape(t *testing.T) {
	age := int64(3600)
	rules := []objectstorage.CORSRule{
		{ID: "web", Allowed: objectstorage.CORSAllowed{Origins: []string{"https://example.com", "http://localhost:3000"}, Methods: []string{"GET", "HEAD"}, Headers: []string{"*"}},
			ExposeHeaders: []string{"ETag"}, MaxAgeSeconds: &age},
		rule([]string{"*"}, []string{"GET"}),
		rule([]string{"https://*.example.com"}, []string{"PUT", "POST", "DELETE"}),
	}
	if err := objectstorage.ValidateCORS(rules); err != nil {
		t.Fatal(err)
	}
	// What is stored parses to the same rules and back.
	raw, _ := json.Marshal(rules)
	parsed, err := objectstorage.ParseCORS(raw)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(parsed)
	if string(raw) != string(again) {
		t.Fatalf("round trip changed the rules:\n%s\n%s", raw, again)
	}
	if err := objectstorage.ValidateCORS(nil); err != nil {
		t.Fatalf("no rules is valid: %v", err)
	}
}

func TestValidateCORSRefusesWhatBreaksTheShapeOrTheCaps(t *testing.T) {
	neg := int64(-1)
	huge := int64(objectstorage.MaxCORSMaxAgeSeconds + 1)
	// Each refusal reaches an S3 client as it is, so it names the S3 element the
	// client sent, never the stored field.
	for name, c := range map[string]struct {
		rules []objectstorage.CORSRule
		names string
	}{
		"no origin":         {[]objectstorage.CORSRule{rule(nil, []string{"GET"})}, "AllowedOrigin"},
		"no method":         {[]objectstorage.CORSRule{rule([]string{"*"}, nil)}, "AllowedMethod"},
		"unknown method":    {[]objectstorage.CORSRule{rule([]string{"*"}, []string{"PATCH"})}, `AllowedMethod "PATCH"`},
		"lower-case method": {[]objectstorage.CORSRule{rule([]string{"*"}, []string{"get"})}, `AllowedMethod "get"`},
		"origin with path":  {[]objectstorage.CORSRule{rule([]string{"https://example.com/a"}, []string{"GET"})}, `AllowedOrigin "https://example.com/a"`},
		"origin no scheme":  {[]objectstorage.CORSRule{rule([]string{"example.com"}, []string{"GET"})}, `AllowedOrigin "example.com"`},
		"two wildcards":     {[]objectstorage.CORSRule{rule([]string{"https://*.*.example.com"}, []string{"GET"})}, "AllowedOrigin"},
		"space in header":   {[]objectstorage.CORSRule{{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}, Headers: []string{"X A"}}}}, `AllowedHeader "X A"`},
		"empty expose":      {[]objectstorage.CORSRule{{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}}, ExposeHeaders: []string{""}}}, `ExposeHeader ""`},
		"negative max age":  {[]objectstorage.CORSRule{{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}}, MaxAgeSeconds: &neg}}, "MaxAgeSeconds"},
		"huge max age":      {[]objectstorage.CORSRule{{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}}, MaxAgeSeconds: &huge}}, "MaxAgeSeconds"},
	} {
		err := objectstorage.ValidateCORS(c.rules)
		if err == nil || errors.Is(err, objectstorage.ErrCORSOverCap) {
			t.Errorf("%s: %v, want a malformed-rule error", name, err)
			continue
		}
		if !strings.Contains(err.Error(), c.names) {
			t.Errorf("%s: %q does not name %s", name, err, c.names)
		}
		for _, field := range []string{"allowed.", "maxAgeSeconds", "exposeHeaders"} {
			if strings.Contains(err.Error(), field) {
				t.Errorf("%s: %q names the stored field %s", name, err, field)
			}
		}
	}
	// 86400 seconds is the cap, inclusive.
	edge := int64(objectstorage.MaxCORSMaxAgeSeconds)
	if err := objectstorage.ValidateCORS([]objectstorage.CORSRule{{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}}, MaxAgeSeconds: &edge}}); err != nil {
		t.Errorf("a max age of exactly %d: %v", edge, err)
	}
	// The error names the rule.
	if err := objectstorage.ValidateCORS([]objectstorage.CORSRule{rule([]string{"*"}, []string{"GET"}), rule(nil, []string{"GET"})}); err == nil || !strings.Contains(err.Error(), "rule 2") {
		t.Errorf("the error does not say which rule: %v", err)
	}

	many := make([]objectstorage.CORSRule, objectstorage.MaxCORSRules+1)
	for i := range many {
		many[i] = rule([]string{"*"}, []string{"GET"})
	}
	if err := objectstorage.ValidateCORS(many[:objectstorage.MaxCORSRules]); err != nil {
		t.Fatalf("exactly %d rules is the cap: %v", objectstorage.MaxCORSRules, err)
	}
	if err := objectstorage.ValidateCORS(many); !errors.Is(err, objectstorage.ErrCORSOverCap) {
		t.Errorf("%d rules = %v, want ErrCORSOverCap", len(many), err)
	}
	big := objectstorage.CORSRule{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}}}
	for range 400 {
		big.Allowed.Headers = append(big.Allowed.Headers, strings.Repeat("h", 200))
	}
	if err := objectstorage.ValidateCORS([]objectstorage.CORSRule{big}); !errors.Is(err, objectstorage.ErrCORSOverCap) {
		t.Errorf("a rule set over %d bytes = %v, want ErrCORSOverCap", objectstorage.MaxCORSBytes, err)
	}
}

func TestParseCORSToleratesUnknownFieldsAndRefusesNonLists(t *testing.T) {
	rules, err := objectstorage.ParseCORS(json.RawMessage(`[{"allowed":{"origins":["*"],"methods":["GET"]},"somethingNew":1}]`))
	if err != nil || len(rules) != 1 {
		t.Fatalf("unknown field = %v, %v", rules, err)
	}
	if rules, err := objectstorage.ParseCORS(nil); err != nil || rules != nil {
		t.Fatalf("nothing = %v, %v", rules, err)
	}
	if _, err := objectstorage.ParseCORS(json.RawMessage(`{"rules":[]}`)); err == nil {
		t.Fatal("an object is not a rule list")
	}
}
