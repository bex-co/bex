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

package s3gw

import (
	"net/http"
	"strings"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/types/netutil"
)

func (g *rig) from(r *http.Request, addr string) *http.Request {
	r.RemoteAddr = addr + ":4242"
	return r
}

// unknownKey is a well-formed key id no one holds: every call is a new one.
func unknownKey(int) testKey { return testKey{id: ids.New(ids.ObjectAccessKey), secret: "x"} }

func TestAFloodOfUnknownKeyIdsFromOneAddressStopsReachingTheStores(t *testing.T) {
	g := newRig(t)
	good := g.newKey(objectstorage.KeyPermissionRead, 0)
	spyOn(t, "ListBuckets", func(*request) *Error { return nil })
	// The key this replica already serves from this address is in its caches.
	if rec, _ := g.do(g.from(g.signed(good, "GET", "/", nil), "203.0.113.9")); rec.Code != 200 {
		t.Fatalf("warm-up = %d", rec.Code)
	}
	for i := 0; i < objectstorage.MaxFailedAuthsPerMinute; i++ {
		rec, doc := g.do(g.from(g.signed(unknownKey(i), "GET", "/", nil), "203.0.113.9"))
		if doc.Code != "InvalidAccessKeyId" {
			t.Fatalf("guess %d = %d %q, want InvalidAccessKeyId", i, rec.Code, doc.Code)
		}
	}
	lookups, reads := g.repo.Gets(), g.kv.Gets()
	for i := 100; i < 300; i++ {
		rec, doc := g.do(g.from(g.signed(unknownKey(i), "GET", "/", nil), "203.0.113.9"))
		if rec.Code != http.StatusServiceUnavailable || doc.Code != "SlowDown" {
			t.Fatalf("guess %d over the budget = %d %q, want 503 SlowDown", i, rec.Code, doc.Code)
		}
	}
	if g.repo.Gets() != lookups || g.kv.Gets() != reads {
		t.Errorf("200 guesses over the budget cost %d database lookups and %d OpenBao reads, want none", g.repo.Gets()-lookups, g.kv.Gets()-reads)
	}
	// A key the caches hold still works from that address, and another address is unaffected.
	if rec, doc := g.do(g.from(g.signed(good, "GET", "/", nil), "203.0.113.9")); rec.Code != 200 {
		t.Errorf("a cached key from the flooding address = %d %q, want it served", rec.Code, doc.Code)
	}
	other := g.newKey(objectstorage.KeyPermissionRead, 0)
	if rec, doc := g.do(g.from(g.signed(other, "GET", "/", nil), "198.51.100.4")); rec.Code != 200 {
		t.Errorf("a new key from another address = %d %q, want it served", rec.Code, doc.Code)
	}
	// A key this replica has not cached is refused from the flooding address without a lookup,
	// even when it is real: the budget holds until it refills.
	fresh := g.newKey(objectstorage.KeyPermissionRead, 0)
	before := g.repo.Gets()
	if rec, doc := g.do(g.from(g.signed(fresh, "GET", "/", nil), "203.0.113.9")); doc.Code != "SlowDown" || g.repo.Gets() != before {
		t.Errorf("an uncached key from the flooding address = %d %q (lookups %d), want SlowDown with no lookup", rec.Code, doc.Code, g.repo.Gets()-before)
	}
}

func TestAnIPv6AddressIsLimitedByItsSlash64(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"2001:db8:1:2:aaaa::1", "2001:db8:1:2:bbbb::2"}, // one /64
		{"203.0.113.9", "203.0.113.9"},
	} {
		if netutil.SourceKeyString(c.a) != netutil.SourceKeyString(c.b) {
			t.Errorf("%s and %s are one subscriber and must share a budget", c.a, c.b)
		}
	}
	if netutil.SourceKeyString("2001:db8:1:2::1") == netutil.SourceKeyString("2001:db8:1:3::1") {
		t.Error("two /64s share a budget")
	}
	if netutil.SourceKeyString("::ffff:203.0.113.9") != "203.0.113.9" {
		t.Errorf("an IPv4-mapped address is its IPv4 address: %q", netutil.SourceKeyString("::ffff:203.0.113.9"))
	}
	if got := netutil.SourceKeyString("not an address"); got != "not an address" {
		t.Errorf("a non-address is its own key, got %q", got)
	}
	g := newRig(t)
	spyOn(t, "ListBuckets", func(*request) *Error { return nil })
	for i := 0; i < objectstorage.MaxFailedAuthsPerMinute; i++ {
		g.do(g.from(g.signed(unknownKey(i), "GET", "/", nil), "2001:db8:1:2:"+string(rune('a'+i%6))+"::"+string(rune('1'+i%9))))
	}
	// The next address of the same /64 is over the budget at once.
	before := g.repo.Gets()
	_, doc := g.do(g.from(g.signed(unknownKey(500), "GET", "/", nil), "2001:db8:1:2:ffff::7"))
	if doc.Code != "SlowDown" || g.repo.Gets() != before {
		t.Errorf("a sibling address of the /64 = %q (lookups %d), want SlowDown with no lookup", doc.Code, g.repo.Gets()-before)
	}
}

func TestOnlyAFailedAuthenticationSpendsTheBudget(t *testing.T) {
	g := newRig(t)
	good := g.newKey(objectstorage.KeyPermissionRead, 0)
	spyOn(t, "ListBuckets", func(*request) *Error { return nil })
	// Skewed clocks and 503s are not guesses.
	for i := 0; i < 3*objectstorage.MaxFailedAuthsPerMinute; i++ {
		r := g.from(g.signed(good, "GET", "/", nil), "203.0.113.50")
		g.now = testNow.Add(time.Hour)
		if _, doc := g.do(r); doc.Code != "RequestTimeTooSkewed" {
			t.Fatalf("= %q, want RequestTimeTooSkewed", doc.Code)
		}
		g.now = testNow
	}
	if rec, _ := g.do(g.from(g.signed(good, "GET", "/", nil), "203.0.113.50")); rec.Code != 200 {
		t.Errorf("after many skewed requests = %d, want the address still served", rec.Code)
	}
	// Successful requests never spend it either.
	for i := 0; i < 3*objectstorage.MaxFailedAuthsPerMinute; i++ {
		if rec, _ := g.do(g.from(g.signed(good, "GET", "/", nil), "203.0.113.51")); rec.Code != 200 {
			t.Fatalf("request %d = %d", i, rec.Code)
		}
	}
}

func TestKeysAreValidatedAfterAuthorizationAndTheD7RulesHold(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionReadWrite, 0)
	ran := false
	for _, op := range []string{"GetObject", "PutObject", "DeleteObject", "HeadObject"} {
		spyOn(t, op, func(*request) *Error { ran = true; return nil })
	}
	for _, c := range []struct{ target, code string }{
		{"/b/a//b", "InvalidArgument"}, {"/b/a/./b", "InvalidArgument"}, {"/b/a/../b", "InvalidArgument"}, {"/b/..", "InvalidArgument"}, {"/b/.", "InvalidArgument"},
		{"/b/%2e%2e/secret", "InvalidArgument"}, {"/b/a/%2E%2E/b", "InvalidArgument"}, {"/b/a%5Cb", "InvalidArgument"}, {"/b/a%00b", "InvalidArgument"},
		{"/b/a%0Ab", "InvalidArgument"}, {"/b//leading", "InvalidArgument"},
		{"/b/" + strings.Repeat("k", 1025), "KeyTooLongError"},
	} {
		ran = false
		rec, doc := g.do(g.signed(k, "GET", c.target, nil))
		if rec.Code != 400 || doc.Code != c.code || ran {
			t.Errorf("GET %s = %d %q (handler ran: %v), want 400 %s", c.target, rec.Code, doc.Code, ran, c.code)
		}
	}
	for _, target := range []string{"/b/" + strings.Repeat("k", 1024), "/b/%C3%A9/%E4%B8%AD%E6%96%87.txt", "/b/a.b/c..d/.hidden", "/b/dir/"} {
		ran = false
		if rec, doc := g.do(g.signed(k, "GET", target, nil)); rec.Code != 200 || !ran {
			t.Errorf("GET %s = %d %q (ran %v), want it served", target, rec.Code, doc.Code, ran)
		}
	}
	// The rule applies to every object operation, writes included.
	ran = false
	if rec, _ := g.do(g.signed(k, "PUT", "/b/a/../b", nil)); rec.Code != 400 || ran {
		t.Errorf("PUT a/../b = %d (ran %v)", rec.Code, ran)
	}
	// A refused caller's bad key is never validated: authorization comes first, so
	// the refusal is the relation's, not the key's.
	read := g.newKey(objectstorage.KeyPermissionRead, 0)
	if rec, doc := g.do(g.signed(read, "PUT", "/b/a/../b", nil)); rec.Code != 403 || doc.Code != "AccessDenied" {
		t.Errorf("a read key's put of a bad key = %d %q, want AccessDenied before the key is looked at", rec.Code, doc.Code)
	}
}
