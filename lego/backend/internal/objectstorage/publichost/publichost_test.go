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

package publichost

import (
	"strings"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestLabelAndBucketIDAreInverses(t *testing.T) {
	for range 50 {
		id := ids.New(ids.ObjectBucket)
		label, ok := Label(id)
		if !ok || !strings.HasPrefix(label, "pub-") || strings.Contains(label, ".") || len(label) != len(id) {
			t.Fatalf("Label(%q) = %q, %v", id, label, ok)
		}
		if back, ok := BucketID(label); !ok || back != id {
			t.Fatalf("BucketID(%q) = %q, %v, want %q", label, back, ok, id)
		}
	}
}

// TestOnlyABucketIDMakesALabel: no other kind of id, and nothing that is not an
// id, names a bucket's host, so a request cannot reach another resource by
// spelling its id after pub-.
func TestOnlyABucketIDMakesALabel(t *testing.T) {
	for name, id := range map[string]string{
		"a workspace": ids.New(ids.Workspace), "a service": ids.New(ids.Service), "empty": "", "no xid": "obk-",
		"short": "obk-abc", "long": "obk-" + strings.Repeat("0", 21), "upper": "obk-" + strings.Repeat("A", 20), "outside the alphabet": "obk-" + strings.Repeat("w", 20),
	} {
		if label, ok := Label(id); ok {
			t.Errorf("%s: Label(%q) = %q, want none", name, id, label)
		}
	}
	xid := strings.TrimPrefix(ids.New(ids.ObjectBucket), "obk-")
	for name, label := range map[string]string{
		"no prefix": xid, "wrong prefix": "pud-" + xid, "obk prefix": "obk-" + xid, "upper": "PUB-" + xid, "a tea id": "pub-" + strings.TrimPrefix(ids.New(ids.Workspace), "tea-") + "x",
		"empty xid": "pub-", "short": "pub-abc", "extra": "pub-" + xid + "0", "a dot": "pub-" + xid[:10] + "." + xid[11:], "traversal": "pub-../../" + xid[:10],
	} {
		if id, ok := BucketID(label); ok {
			t.Errorf("%s: BucketID(%q) = %q, want none", name, label, id)
		}
	}
}

func TestHostAndBucketIDOfHost(t *testing.T) {
	id := ids.New(ids.ObjectBucket)
	host, ok := Host(id, "onbex.co")
	if !ok || !strings.HasSuffix(host, ".onbex.co") || strings.Count(host, ".") != 2 {
		t.Fatalf("Host = %q, %v: the label must be a single label under the domain", host, ok)
	}
	for _, in := range []string{host, strings.ToUpper(host), host + ":443", host + ".", host + ".:8080"} {
		if got, ok := BucketIDOfHost(in, "onbex.co"); !ok || got != id {
			t.Errorf("BucketIDOfHost(%q) = %q, %v, want %q", in, got, ok, id)
		}
	}
	label, _ := Label(id)
	for name, in := range map[string]string{
		"another domain": label + ".example.org", "the apex": "onbex.co", "deeper": "www." + host, "a lookalike domain": label + ".notonbex.co",
		"a suffix only": label + "onbex.co", "empty": "", "a service": "api.onbex.co", "no label": ".onbex.co", "ipv6": "[::1]:8080",
	} {
		if got, ok := BucketIDOfHost(in, "onbex.co"); ok {
			t.Errorf("%s: BucketIDOfHost(%q) = %q, want none", name, in, got)
		}
	}
	// No hosting domain, no host: nothing can be served or reserved without one.
	if _, ok := Host(id, ""); ok {
		t.Error("Host with no domain")
	}
	if _, ok := BucketIDOfHost(host, ""); ok {
		t.Error("BucketIDOfHost with no domain")
	}
	if _, ok := Host("obk-bad", "onbex.co"); ok {
		t.Error("Host of a malformed id")
	}
}

func TestReservedIsExactlyTheShapeOfABucketHost(t *testing.T) {
	label, _ := Label(ids.New(ids.ObjectBucket))
	if !Reserved(label) {
		t.Errorf("%q is a bucket's host label and must be reserved", label)
	}
	for _, slug := range []string{"", "pub", "pub-", "pub-site", "my-pub-site", label + "-abcd", label + "x", strings.ToUpper(label), "xpub" + label[3:], strings.Repeat("a", 24)} {
		if Reserved(slug) {
			t.Errorf("%q is reserved; only the exact shape of a bucket's host is (Render lets a service be called pub-site)", slug)
		}
	}
}

// The direct host of the Cloudflare-shaped API is objects.<base domain>
// (ADR112 D3): a service cannot take that label either, and only that exact one.
func TestReservedIncludesTheDirectAPIHost(t *testing.T) {
	if APILabel != "objects" {
		t.Fatalf("APILabel = %q: the IngressRoute in config/api and the docs name objects.<base domain>", APILabel)
	}
	if !Reserved(APILabel) {
		t.Errorf("%q is the direct API host's label and must be reserved", APILabel)
	}
	for _, slug := range []string{"object", "objects-", "my-objects", "objects2", "Objects", "objects-abcd"} {
		if Reserved(slug) {
			t.Errorf("%q is reserved; only the exact label %q is", slug, APILabel)
		}
	}
}

func TestNormalizeHost(t *testing.T) {
	for in, want := range map[string]string{
		"Pub-X.OnBex.Co": "pub-x.onbex.co", "a.b:8443": "a.b", "a.b.": "a.b", "a.b.:80": "a.b", "[::1]:8080": "[::1]", "[::1]": "[::1]", "": "",
	} {
		if got := NormalizeHost(in); got != want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}
