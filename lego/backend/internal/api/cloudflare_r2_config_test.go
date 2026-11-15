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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// The rules exactly as wrangler 4.162.0 writes them for `cors set` and for
// `lifecycle add` (an Age rule in seconds, a Date rule, an abort rule).
const (
	wranglerCORS = `{"rules":[{"allowed":{"origins":["https://example.com","*"],"methods":["GET","PUT"],"headers":["content-type"]},"exposeHeaders":["ETag"],"maxAgeSeconds":3600},{"id":"read","allowed":{"origins":["*"],"methods":["GET","HEAD"]}}]}`
	wranglerLife = `{"rules":[` +
		`{"id":"expire-logs","enabled":true,"conditions":{"prefix":"logs/"},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":604800}}},` +
		`{"id":"sunset","enabled":false,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Date","date":"2027-01-01T00:00:00.000Z"}}},` +
		`{"id":"tidy","enabled":true,"conditions":{"prefix":"up/"},"abortMultipartUploadsTransition":{"condition":{"type":"Age","maxAge":86400}}}]}`
)

// sameJSON compares two JSON documents by value.
func sameJSON(t *testing.T, got, want json.RawMessage) bool {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("%s: %v", got, err)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(g) == fmt.Sprint(w)
}

func TestCloudflareCORSRulesRoundTripThroughSetAndList(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")

	// Nothing set yet: an empty list, never null (wrangler iterates it).
	empty := h.do(t, http.MethodGet, h.url("/buckets/photos/cors"), nil)
	wantOK(t, empty)
	if !strings.Contains(string(empty.Env.Result), `"rules":[]`) {
		t.Fatalf("an unset CORS config = %s", empty.Env.Result)
	}

	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/cors"), []byte(wranglerCORS), "Content-Type", "application/json"))
	list := h.do(t, http.MethodGet, h.url("/buckets/photos/cors"), nil)
	wantOK(t, list)
	var want struct{ Rules json.RawMessage }
	_ = json.Unmarshal([]byte(wranglerCORS), &want)
	var got struct{ Rules json.RawMessage }
	_ = json.Unmarshal(list.Env.Result, &got)
	if !sameJSON(t, got.Rules, want.Rules) {
		t.Fatalf("list = %s\nwant the rules set = %s", got.Rules, want.Rules)
	}

	// Set replaces; delete empties.
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/cors"), []byte(`{"rules":[{"allowed":{"origins":["*"],"methods":["GET"]}}]}`)))
	if r := h.do(t, http.MethodGet, h.url("/buckets/photos/cors"), nil); strings.Count(string(r.Env.Result), `"origins"`) != 1 {
		t.Fatalf("a second set did not replace the first: %s", r.Env.Result)
	}
	wantOK(t, h.do(t, http.MethodDelete, h.url("/buckets/photos/cors"), nil))
	if r := h.do(t, http.MethodGet, h.url("/buckets/photos/cors"), nil); !strings.Contains(string(r.Env.Result), `"rules":[]`) {
		t.Fatalf("after delete = %s", r.Env.Result)
	}
	// The rules live on the bucket row, where objectgw reads them (w21/m417).
	row, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos")
	if string(row.CORS) != "[]" {
		t.Errorf("the row holds %s after delete", row.CORS)
	}
}

func TestCloudflareCORSRefusalsCarryTheRightCode(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	put := func(body string) *cfReply {
		return h.do(t, http.MethodPut, h.url("/buckets/photos/cors"), []byte(body))
	}

	// Over the rule cap: an exceeded quota, 10042, naming the cap.
	many := `{"rules":[` + strings.Repeat(`{"allowed":{"origins":["*"],"methods":["GET"]}},`, objectstorage.MaxCORSRules) + `{"allowed":{"origins":["*"],"methods":["GET"]}}]}`
	r := put(many)
	wantCF(t, r, http.StatusForbidden, 10042)
	if !strings.Contains(r.Env.Errors[0].Message, fmt.Sprint(objectstorage.MaxCORSRules)) {
		t.Errorf("the refusal does not name the cap: %q", r.Env.Errors[0].Message)
	}
	// Over the byte cap.
	big := fmt.Sprintf(`{"rules":[{"allowed":{"origins":["*"],"methods":["GET"],"headers":[%s]}}]}`,
		strings.TrimSuffix(strings.Repeat(`"`+strings.Repeat("h", 200)+`",`, 400), ","))
	wantCF(t, put(big), http.StatusForbidden, 10042)
	// A body far over the cap is refused unread.
	wantCF(t, put(`{"rules":[],"pad":"`+strings.Repeat("p", cfCORSBodyLimit)+`"}`), http.StatusForbidden, 10042)
	// Malformed: 400 with bex's code, and the old rules are untouched.
	wantOK(t, put(wranglerCORS))
	for name, body := range map[string]string{
		"not json":       `nope`,
		"aws keys":       `{"CORSRules":[]}`,
		"extra field":    `{"rules":[],"more":1}`,
		"age too long":   `{"rules":[{"allowed":{"origins":["*"],"methods":["GET"]},"maxAgeSeconds":86401}]}`,
		"bad method":     `{"rules":[{"allowed":{"origins":["*"],"methods":["TRACE"]}}]}`,
		"bad origin":     `{"rules":[{"allowed":{"origins":["example.com"],"methods":["GET"]}}]}`,
		"no origins":     `{"rules":[{"allowed":{"methods":["GET"]}}]}`,
		"negative age":   `{"rules":[{"allowed":{"origins":["*"],"methods":["GET"]},"maxAgeSeconds":-5}]}`,
		"rules not list": `{"rules":{"a":1}}`,
	} {
		wantCF(t, put(body), http.StatusBadRequest, 7400)
		_ = name
	}
	list := h.do(t, http.MethodGet, h.url("/buckets/photos/cors"), nil)
	if strings.Count(string(list.Env.Result), `"origins"`) != 2 {
		t.Fatalf("a refused set changed the stored rules: %s", list.Env.Result)
	}
	// The cap is 86400 seconds, as Cloudflare's schema shows, and inclusive.
	wantOK(t, put(`{"rules":[{"allowed":{"origins":["*"],"methods":["GET"]},"maxAgeSeconds":86400}]}`))
	// Cloudflare documents rules as optional: a body with none clears the set.
	wantOK(t, put(`{}`))
	if r := h.do(t, http.MethodGet, h.url("/buckets/photos/cors"), nil); !strings.Contains(string(r.Env.Result), `"rules":[]`) {
		t.Fatalf("a body with no rules left %s", r.Env.Result)
	}
}

func TestCloudflareLifecycleRulesRoundTripAndAreStoredForTheSweep(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	if r := h.do(t, http.MethodGet, h.url("/buckets/photos/lifecycle"), nil); !strings.Contains(string(r.Env.Result), `"rules":[]`) {
		t.Fatalf("an unset lifecycle = %s", r.Env.Result)
	}
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/lifecycle"), []byte(wranglerLife), "cf-r2-data-catalog-check", "true"))

	list := h.do(t, http.MethodGet, h.url("/buckets/photos/lifecycle"), nil)
	var want, got struct{ Rules json.RawMessage }
	_ = json.Unmarshal([]byte(wranglerLife), &want)
	_ = json.Unmarshal(list.Env.Result, &got)
	if !sameJSON(t, got.Rules, want.Rules) {
		t.Fatalf("list = %s\nwant %s", got.Rules, want.Rules)
	}
	// The sweep reads the row: wrangler's `lifecycle list` needs conditions.prefix on every rule.
	row, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos")
	if !strings.Contains(string(row.Lifecycle), `"expire-logs"`) || !strings.Contains(string(row.Lifecycle), `"prefix":""`) {
		t.Fatalf("the row holds %s", row.Lifecycle)
	}
	// The hourly sweep parses what the route stored, and every stored rule is
	// one it will run: the route and the sweep cannot disagree about a rule.
	parsed, err := objectstorage.ParseLifecycle(row.Lifecycle)
	if err != nil || len(parsed) != 3 {
		t.Fatalf("the sweep reads %d rules from the stored row: %v", len(parsed), err)
	}
	for _, rule := range parsed {
		if err := rule.Validate(); err != nil {
			t.Errorf("the route stored a rule the sweep will not run: %v", err)
		}
	}
	if d := parsed[0].DeleteObjectsTransition; parsed[0].ID != "expire-logs" || parsed[0].Conditions.Prefix != "logs/" || d == nil || d.Condition.MaxAge == nil || *d.Condition.MaxAge != 604800 {
		t.Errorf("the first stored rule reads as %+v", parsed[0])
	}
	// `lifecycle remove` is a put of the set without the rule; an empty set clears.
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/lifecycle"), []byte(`{"rules":[]}`)))
	if r := h.do(t, http.MethodGet, h.url("/buckets/photos/lifecycle"), nil); !strings.Contains(string(r.Env.Result), `"rules":[]`) {
		t.Fatalf("after clearing = %s", r.Env.Result)
	}
}

func TestCloudflareLifecycleRefusals(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	put := func(body string) *cfReply {
		return h.do(t, http.MethodPut, h.url("/buckets/photos/lifecycle"), []byte(body))
	}
	age := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":60}}}`, id)
	}

	// A storage class transition: bex has one class, 10042.
	r := put(`{"rules":[{"id":"ia","enabled":true,"conditions":{"prefix":""},"storageClassTransitions":[{"condition":{"type":"Age","maxAge":86400},"storageClass":"InfrequentAccess"}]}]}`)
	wantCF(t, r, http.StatusForbidden, 10042)
	if !strings.Contains(r.Env.Errors[0].Message, "storage class") {
		t.Errorf("the refusal does not say why: %q", r.Env.Errors[0].Message)
	}
	// Over the rule cap: 10042; exactly at it is stored.
	rules := make([]string, objectstorage.MaxLifecycleRules)
	for i := range rules {
		rules[i] = age(fmt.Sprintf("r%d", i))
	}
	wantOK(t, put(`{"rules":[`+strings.Join(rules, ",")+`]}`))
	wantCF(t, put(`{"rules":[`+strings.Join(append(rules, age("one-more")), ",")+`]}`), http.StatusForbidden, 10042)
	// Malformed rules: 400 with bex's code.
	for name, body := range map[string]string{
		"no id":         `{"rules":[{"enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":1}}}]}`,
		"duplicate id":  `{"rules":[` + age("a") + `,` + age("a") + `]}`,
		"no action":     `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""}}]}`,
		"bad type":      `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Rate","maxAge":1}}}]}`,
		"age no maxAge": `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Age"}}}]}`,
		"negative age":  `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":-1}}}]}`,
		"bad date":      `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Date","date":"someday"}}}]}`,
		"control char":  `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":"a\u0001"},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":1}}}]}`,
		"extra field":   `{"rules":[],"more":1}`,
		"bad abort":     `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"abortMultipartUploadsTransition":{"condition":{"type":"Age","maxAge":-1}}}]}`,
		"abort garbage": `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"abortMultipartUploadsTransition":"soon"}]}`,
		"empty rule":    `{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"storageClassTransitions":[]}]}`,
	} {
		if r := put(body); r.Code != http.StatusBadRequest || r.errCode() != 7400 {
			t.Errorf("%s: %d %s, want 400 with 7400", name, r.Code, r.Raw)
		}
	}
	// A day is a valid Date, and a refused set left the 1000 stored rules alone.
	wantOK(t, put(`{"rules":[{"id":"a","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Date","date":"2027-01-01"}}}]}`))
}

func TestCloudflareCORSAndLifecycleBelongToTheCallersBucket(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/cors"), []byte(wranglerCORS)))
	other := &cfHarness{srv: h.srv, h: h.h, fake: h.fake, repo: h.repo, svc: h.svc, ws: newWS()}
	wantCF(t, other.do(t, http.MethodGet, other.url("/buckets/photos/cors"), nil), http.StatusNotFound, 10006)
	wantCF(t, other.do(t, http.MethodPut, other.url("/buckets/photos/lifecycle"), []byte(`{"rules":[]}`)), http.StatusNotFound, 10006)
	other.createBucket(t, "photos")
	if r := other.do(t, http.MethodGet, other.url("/buckets/photos/cors"), nil); !strings.Contains(string(r.Env.Result), `"rules":[]`) {
		t.Fatalf("a same-named bucket of another workspace shows these rules: %s", r.Env.Result)
	}
}
