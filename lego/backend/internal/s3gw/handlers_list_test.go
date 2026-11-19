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
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// listAll pages a listing to its end with the given parameters and returns every entry in
// order, Contents keys and common prefixes interleaved as the pages gave them, with the
// number of pages it took.
func (p *pgRig) listAll(k testKey, bucket string, v2 bool, params url.Values, pageSize int) (entries []string, pages int) {
	p.t.Helper()
	token := ""
	for pages < 200 {
		q := url.Values{}
		for k, v := range params {
			q[k] = v
		}
		q.Set("max-keys", fmt.Sprint(pageSize))
		if v2 {
			q.Set("list-type", "2")
			if token != "" {
				q.Set("continuation-token", token)
			}
		} else if token != "" {
			q.Set("marker", token)
		}
		rec, doc := p.do(p.signed(k, "GET", "/"+bucket+"?"+q.Encode(), nil))
		if rec.Code != 200 {
			p.t.Fatalf("list %v = %d %q %s", q, rec.Code, doc.Code, doc.Message)
		}
		var res listBucketResultXML
		if err := xml.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			p.t.Fatal(err)
		}
		pages++
		// Contents and prefixes are separate lists in the XML; both are sorted, so merge.
		ci, pi := 0, 0
		for ci < len(res.Contents) || pi < len(res.CommonPrefixes) {
			switch {
			case pi >= len(res.CommonPrefixes) || ci < len(res.Contents) && res.Contents[ci].Key < res.CommonPrefixes[pi].Prefix:
				entries = append(entries, res.Contents[ci].Key)
				ci++
			default:
				entries = append(entries, res.CommonPrefixes[pi].Prefix)
				pi++
			}
		}
		if !res.IsTruncated {
			return entries, pages
		}
		if v2 {
			token = res.NextContinuationToken
		} else {
			token = res.NextMarker
		}
		if token == "" {
			p.t.Fatal("a truncated page carries no continuation")
		}
	}
	p.t.Fatal("the listing never ended")
	return nil, 0
}

func (p *pgRig) putKeys(k testKey, bucket string, keys ...string) {
	p.t.Helper()
	for _, key := range keys {
		if rec, doc := p.put(k, "/"+bucket+"/"+uriEncode(key, false), 1); rec.Code != 200 {
			p.t.Fatalf("put %q = %d %q %s", key, rec.Code, doc.Code, doc.Message)
		}
	}
}

// newListRig is a rig whose request and listing rates are out of the way of a test that
// pages through a tree one key at a time.
func newListRig(t *testing.T) *pgRig {
	p := newPGRig(t)
	wide := func() *core.KeyedRateLimiter[string] {
		return core.NewKeyedRateLimiter[string](6_000_000, 100_000, time.Minute, time.Minute)
	}
	p.gw.limits.perWorkspace, p.gw.limits.perKey, p.gw.limits.listings = wide(), wide(), wide()
	return p
}

func TestAListingPagesATreeWithoutRepeatsOrGapsForEveryDelimiterCase(t *testing.T) {
	p := newListRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("tree", objectstorage.Oregon)
	keys := []string{"a.txt", "dir/a", "dir/b", "dir/sub/c", "dir/sub/d", "dir0", "dir2/x", "z", "é/ü", "é0", "x/", "x/y"}
	p.putKeys(k, "tree", keys...)
	for _, v2 := range []bool{true, false} {
		name := map[bool]string{true: "V2", false: "V1"}[v2]
		// Without a delimiter: every key, in order, however small the pages.
		for size := 1; size <= 5; size++ {
			got, _ := p.listAll(k, "tree", v2, url.Values{}, size)
			if !reflect.DeepEqual(got, sortedKeys(keys)) {
				t.Errorf("%s size %d: listing = %v, want %v", name, size, got, sortedKeys(keys))
			}
		}
		// With a delimiter: the keys folded into common prefixes, none repeated, none skipped,
		// including a key (dir0, é0) that sorts right after a prefix it is not under.
		want := []string{"a.txt", "dir/", "dir0", "dir2/", "x/", "z", "é/", "é0"}
		for size := 1; size <= 4; size++ {
			got, _ := p.listAll(k, "tree", v2, url.Values{"delimiter": {"/"}}, size)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s delimiter size %d: listing = %v, want %v", name, size, got, want)
			}
		}
		// A prefix: the keys under it, and the folded level below.
		got, _ := p.listAll(k, "tree", v2, url.Values{"delimiter": {"/"}, "prefix": {"dir/"}}, 1)
		if want := []string{"dir/a", "dir/b", "dir/sub/"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s prefix dir/: %v, want %v", name, got, want)
		}
		// A key that equals the prefix is a Contents entry, and continuing from it does not skip its children.
		got, _ = p.listAll(k, "tree", v2, url.Values{"delimiter": {"/"}, "prefix": {"x/"}}, 1)
		if want := []string{"x/", "x/y"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s prefix x/ (a key that equals it): %v, want %v", name, got, want)
		}
		// A prefix with a multi-byte character before the delimiter.
		got, _ = p.listAll(k, "tree", v2, url.Values{"delimiter": {"/"}, "prefix": {"é/"}}, 1)
		if want := []string{"é/ü"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s prefix é/: %v, want %v", name, got, want)
		}
		// Another delimiter.
		got, _ = p.listAll(k, "tree", v2, url.Values{"delimiter": {"b"}}, 2)
		if want := []string{"a.txt", "dir/a", "dir/b", "dir/sub", "dir0", "dir2/x", "x/", "x/y", "z", "é/ü", "é0"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s delimiter b: %v, want %v", name, got, want)
		}
	}
	// A page that ends on a common prefix does not repeat it.
	got, pages := p.listAll(k, "tree", true, url.Values{"delimiter": {"/"}}, 2)
	seen := map[string]bool{}
	for _, e := range got {
		if seen[e] {
			t.Errorf("%q was returned twice across %d pages", e, pages)
		}
		seen[e] = true
	}
}

func sortedKeys(keys []string) []string {
	out := append([]string(nil), keys...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestAForgedTokenOrAPrefixCannotLeaveTheBucket(t *testing.T) {
	p := newListRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mine", objectstorage.Oregon)
	p.bucket("yours", objectstorage.Oregon)
	p.putKeys(k, "mine", "m1", "m2", "m3")
	p.putKeys(k, "yours", "y1", "y2")
	for _, token := range []string{"not base64!", "", "AAAA", objectstorage.EncodeListToken(false, strings.Repeat("k", 2000)), "Qm9ndXM"} {
		if token == "" {
			continue
		}
		rec, doc := p.do(p.signed(k, "GET", "/mine?list-type=2&continuation-token="+url.QueryEscape(token), nil))
		if rec.Code != 400 || doc.Code != "InvalidArgument" {
			t.Errorf("token %q = %d %q, want 400 InvalidArgument", token, rec.Code, doc.Code)
		}
	}
	// A token for a key of another bucket's physical layout only moves the start inside this bucket.
	forged := objectstorage.EncodeListToken(false, "../yours/y1")
	rec, _ := p.do(p.signed(k, "GET", "/mine?list-type=2&continuation-token="+url.QueryEscape(forged), nil))
	var res listBucketResultXML
	_ = xml.Unmarshal(rec.Body.Bytes(), &res)
	for _, c := range res.Contents {
		if !strings.HasPrefix(c.Key, "m") {
			t.Errorf("a forged token listed %q, which is not in the bucket", c.Key)
		}
	}
	for _, prefix := range []string{"../yours/", "a/../../yours/", "/", "..", "./x"} {
		rec, doc := p.do(p.signed(k, "GET", "/mine?list-type=2&prefix="+url.QueryEscape(prefix), nil))
		if rec.Code != 400 || doc.Code != "InvalidArgument" {
			t.Errorf("prefix %q = %d %q, want 400 InvalidArgument", prefix, rec.Code, doc.Code)
		}
	}
	// A bucket named like another workspace's physical path is just a bucket.
	if rec, doc := p.do(p.signed(k, "GET", "/"+p.ws+"?list-type=2", nil)); rec.Code != 404 || doc.Code != "NoSuchBucket" {
		t.Errorf("a workspace id as a bucket = %d %q, want NoSuchBucket", rec.Code, doc.Code)
	}
}

func TestListingParametersAreValidatedAndEncodedAsS3DoesIt(t *testing.T) {
	p := newListRig(t)
	k := p.newKey(objectstorage.KeyPermissionRead, 0)
	rw := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("params", objectstorage.Oregon)
	p.putKeys(rw, "params", "my file.txt", "é/ü", "plain")
	get := func(q string) (*listBucketResultXML, int, string) {
		rec, doc := p.do(p.signed(k, "GET", "/params?"+q, nil))
		var res listBucketResultXML
		_ = xml.Unmarshal(rec.Body.Bytes(), &res)
		return &res, rec.Code, doc.Code
	}
	if res, code, _ := get("list-type=2&max-keys=0"); code != 200 || len(res.Contents) != 0 || res.IsTruncated {
		t.Errorf("max-keys=0 = %d with %d keys truncated=%v, want an empty untruncated page", code, len(res.Contents), res.IsTruncated)
	}
	for _, q := range []string{"list-type=2&max-keys=abc", "list-type=2&max-keys=-1", "list-type=2&encoding-type=base64"} {
		if _, code, s3 := get(q); code != 400 || s3 != "InvalidArgument" {
			t.Errorf("%s = %d %q, want 400 InvalidArgument", q, code, s3)
		}
	}
	for _, q := range []string{"list-type=2&delimiter=ab", "list-type=2&delimiter=" + url.QueryEscape("é")} {
		if _, code, s3 := get(q); code != 501 || s3 != "NotImplemented" {
			t.Errorf("%s = %d %q, want 501 NotImplemented", q, code, s3)
		}
	}
	res, _, _ := get("list-type=2&encoding-type=url")
	var keys []string
	for _, c := range res.Contents {
		keys = append(keys, c.Key)
	}
	if want := []string{"my%20file.txt", "plain", "%C3%A9/%C3%BC"}; !reflect.DeepEqual(keys, want) || res.EncodingType != "url" {
		t.Errorf("encoding-type=url keys = %v, want %v", keys, want)
	}
	res, _, _ = get("list-type=2&start-after=my%20file.txt")
	if len(res.Contents) != 2 || res.StartAfter != "my file.txt" || *res.KeyCount != 2 {
		t.Errorf("start-after = %+v", res)
	}
	// V1 shows its marker and V2 its count.
	res, _, _ = get("marker=plain")
	if res.Marker == nil || *res.Marker != "plain" || len(res.Contents) != 1 {
		t.Errorf("a V1 listing after marker=plain = %+v", res)
	}
	// An object's fields are S3's.
	res, _, _ = get("list-type=2&prefix=plain")
	if len(res.Contents) != 1 || res.Contents[0].Size != 1 || res.Contents[0].StorageClass != "STANDARD" || res.Contents[0].ETag == "" ||
		!strings.HasSuffix(res.Contents[0].LastModified, "Z") {
		t.Errorf("an entry = %+v", res.Contents)
	}
}

func TestListingsAreRateCappedAndTheirResponseBytesAreCountedAsEgress(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("rate", objectstorage.Oregon)
	p.putKeys(k, "rate", "a", "b", "c")
	p.gw.limits = newRequestLimits()
	p.Svc.SetUsageSink(p.PG)
	ctx, stop := context.WithCancel(context.Background())
	flushed := make(chan struct{})
	go func() { p.Svc.RunMeter(ctx, time.Hour); close(flushed) }()
	ok, refused := 0, 0
	for i := 0; i < 12; i++ {
		rec, doc := p.do(p.signed(k, "GET", "/rate?list-type=2", nil))
		switch {
		case rec.Code == 200:
			ok++
		case rec.Code == 503 && doc.Code == "SlowDown":
			refused++
		default:
			t.Fatalf("listing %d = %d %q", i, rec.Code, doc.Code)
		}
	}
	if ok < objectstorage.ListingsPerSecondPerWorkspace || ok > objectstorage.ListingsPerSecondPerWorkspace+2 || refused == 0 {
		t.Errorf("%d listings served and %d refused in a burst, want about %d served", ok, refused, objectstorage.ListingsPerSecondPerWorkspace)
	}
	// What the listings read is Wasabi's egress: the meter's final flush writes it.
	stop()
	<-flushed
	if got, err := p.PG.ObjectStorageWorkspaceEgressSince(context.Background(), p.ws, time.Now().Add(-time.Hour)); err != nil || got <= 0 {
		t.Errorf("the egress counted for %d listings = %d, %v, want a positive count", ok, got, err)
	}
}
