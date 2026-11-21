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

package s3

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestLsListsTheBucketsAndTheirTimes(t *testing.T) {
	g := newRig(t)
	g.s3.bucket("photos")
	g.s3.bucket("render-oregon")
	out := g.must("ls")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], " photos") || !strings.HasSuffix(lines[1], " render-oregon") {
		t.Errorf("ls =\n%s\nwant both buckets, system bucket included, one per line", out)
	}
	var doc struct {
		Buckets []lsBucket `json:"buckets"`
	}
	if err := json.Unmarshal([]byte(g.must("ls", "-o", "json")), &doc); err != nil || len(doc.Buckets) != 2 || doc.Buckets[0].Name != "photos" || doc.Buckets[0].CreatedAt == "" {
		t.Errorf("ls -o json = %+v, %v", doc, err)
	}
}

func TestLsListsOneLevelWithFoldersAndNamesBelowThePrefix(t *testing.T) {
	g := newRig(t)
	for key, size := range map[string]int{"top.txt": 3, "2024/jan/a.jpg": 10, "2024/feb/b.jpg": 20, "2024/readme.md": 1500, "2025/c.jpg": 5} {
		g.s3.put("photos", key, make([]byte, size), testNow)
	}
	out := g.must("ls", "s3://photos/")
	for _, want := range []string{"PRE 2024/", "PRE 2025/", " top.txt"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls of the root lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "a.jpg") {
		t.Errorf("a one-level listing shows a nested object:\n%s", out)
	}

	out = g.must("ls", "s3://photos/2024/", "--human-readable", "--summarize")
	for _, want := range []string{"PRE feb/", "PRE jan/", " 1.5 KiB readme.md", "Total Objects: 1", "Total Size: 1.5 KiB"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls of 2024/ lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "2024/readme.md") {
		t.Errorf("a listing below a folder shows the folder's name again:\n%s", out)
	}

	// A prefix is matched by its characters, as on S3.
	if out := g.must("ls", "s3://photos/20"); !strings.Contains(out, "PRE 2024/") || !strings.Contains(out, "PRE 2025/") {
		t.Errorf("ls of the prefix 20 =\n%s", out)
	}

	out = g.must("ls", "s3://photos/2024", "--recursive")
	if got := strings.Fields(out); !slices.Equal(keysColumn(out), []string{"2024/feb/b.jpg", "2024/jan/a.jpg", "2024/readme.md"}) {
		t.Errorf("ls --recursive keys = %v (%v)", keysColumn(out), got)
	}

	var doc lsOutput
	if err := json.Unmarshal([]byte(g.must("ls", "s3://photos/2024/", "-o", "json")), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Bucket != "photos" || doc.TotalObjects != 1 || doc.TotalSize != 1500 || len(doc.CommonPrefixes) != 2 || doc.Objects[0].Key != "2024/readme.md" {
		t.Errorf("ls -o json = %+v", doc)
	}
}

// keysColumn is the last field of every line of a listing.
func keysColumn(out string) []string {
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			keys = append(keys, f[len(f)-1])
		}
	}
	return keys
}

// TestListingAndDeletingTwoThousandFiveHundredKeysTakeThreeCallsEach is the
// page arithmetic: 1,000 keys to a page for a listing and 1,000 to a batch for
// a delete.
func TestListingAndDeletingTwoThousandFiveHundredKeysTakeThreeCallsEach(t *testing.T) {
	g := newRig(t)
	for _, key := range keysOf(2500, "logs/%05d.txt") {
		g.s3.put("photos", key, []byte("x"), testNow)
	}
	g.s3.put("photos", "keep/me.txt", []byte("x"), testNow)

	out := g.must("ls", "s3://photos/logs/", "--recursive", "--summarize")
	if !strings.Contains(out, "Total Objects: 2500") {
		t.Errorf("ls --recursive did not count 2500 objects:\n%s", out[max(0, len(out)-80):])
	}
	if got := g.s3.count("list-type=2"); got != 3 {
		t.Errorf("a listing of 2,500 keys made %d list calls, want 3", got)
	}

	g.s3.reset()
	out = g.must("rm", "s3://photos/logs", "--recursive")
	if got := strings.Count(out, "delete: s3://photos/logs/"); got != 2500 {
		t.Errorf("rm --recursive reported %d deletions, want 2500", got)
	}
	if lists, deletes := g.s3.count("list-type=2"), g.s3.count("POST /photos?delete"); lists != 3 || deletes != 3 {
		t.Errorf("rm --recursive of 2,500 keys made %d list and %d delete calls, want 3 and 3", lists, deletes)
	}
	if keys := g.s3.keys("photos"); !slices.Equal(keys, []string{"keep/me.txt"}) {
		t.Errorf("keys left = %v, want only the one outside the prefix", keys)
	}
}

func TestRmFiltersDryrunAndDirectorySemantics(t *testing.T) {
	g := newRig(t)
	for _, key := range []string{"tmp/a.log", "tmp/b.txt", "tmp/sub/c.log", "tmp2/d.log", "other.log"} {
		g.s3.put("photos", key, []byte("x"), testNow)
	}
	// --dryrun lists and deletes nothing.
	out := g.must("rm", "s3://photos/tmp", "--recursive", "--dryrun")
	if !strings.Contains(out, "(dryrun) delete: s3://photos/tmp/a.log") || g.s3.count("POST /photos?delete") != 0 || len(g.s3.keys("photos")) != 5 {
		t.Errorf("--dryrun: out %q, keys %v", out, g.s3.keys("photos"))
	}
	// A prefix names a folder: tmp2/ is not under tmp.
	// The last matching filter decides: exclude everything, then include the logs.
	g.must("rm", "s3://photos/tmp/", "--recursive", "--exclude", "*", "--include", "*.log")
	if got := g.s3.keys("photos"); !slices.Equal(got, []string{"other.log", "tmp/b.txt", "tmp2/d.log"}) {
		t.Errorf("keys after the filtered delete = %v", got)
	}
	// With --include before --exclude the exclude wins.
	g.must("rm", "s3://photos/", "--recursive", "--include", "*.log", "--exclude", "*")
	if len(g.s3.keys("photos")) != 3 {
		t.Errorf("keys = %v, want nothing deleted when --exclude comes last", g.s3.keys("photos"))
	}
	// One object, and a missing one is not an error.
	if out := g.must("rm", "s3://photos/other.log"); !strings.Contains(out, "delete: s3://photos/other.log") {
		t.Errorf("rm = %q", out)
	}
	g.must("rm", "s3://photos/never-there")
	var doc struct{ Results []result }
	if err := json.Unmarshal([]byte(g.must("rm", "s3://photos/tmp2", "--recursive", "-o", "json")), &doc); err != nil || len(doc.Results) != 1 ||
		doc.Results[0].Action != "delete" || doc.Results[0].Target != "s3://photos/tmp2/d.log" {
		t.Errorf("rm -o json = %+v, %v", doc, err)
	}
}

func TestMalformedCommandLinesNeverCostAKey(t *testing.T) {
	g := newRig(t)
	for _, args := range [][]string{
		{"ls", "photos"}, {"ls", "s3://Photos"}, {"ls", "s3://"}, {"ls", "a", "b"},
		{"rm", "s3://photos"}, {"rm", "photos/key"}, {"rm", "s3://photos/k", "--exclude", "*"}, {"rm", "s3://photos/", "--recursive", "--concurrency", "11"},
		{"mb", "photos"}, {"mb", "s3://photos/key"}, {"rb", "s3://photos/key"},
		{"presign", "s3://photos"}, {"presign", "s3://photos/k", "--expires-in", "0"}, {"presign", "s3://photos/k", "--expires-in", "604801"},
		{"cp", "a", "b"}, {"cp", "a", "s3://photos/k", "--concurrency", "0"}, {"cp", "-", "s3://photos/"}, {"cp", "s3://photos/k", "-", "--recursive"},
		{"cp", "a", "s3://photos/k", "--metadata", "novalue"}, {"cp", "a", "s3://photos/k", "--exclude", "*"}, {"mv", "-", "s3://photos/k"},
		{"sync", "a", "b"}, {"sync", "-", "s3://photos/x"}, {"sync", "s3://photos/a", "s3://photos/a/b"},
		{"cors", "set", "photos"}, {"cors", "get"}, {"cors", "get", "../x"}, {"lifecycle", "set", "photos", "--file", "/no/such/file"},
		{"info"}, {"nonsense"}, {"cors", "nonsense"},
	} {
		if msg := g.fails(args...); msg == "" {
			t.Errorf("bex s3 %v: empty error", args)
		}
	}
	if g.api.mintCount() != 0 || len(g.s3.requests("")) != 0 {
		t.Errorf("a malformed command line cost %d keys and %d requests", g.api.mintCount(), len(g.s3.requests("")))
	}
	if msg := g.fails("s3api"); !strings.Contains(msg, "unknown command") {
		t.Errorf("bex s3 s3api: %q, want unknown command", msg)
	}
	msg := g.fails("nonsense")
	if !strings.Contains(msg, `unknown command "nonsense"`) || !strings.Contains(msg, "bex s3 env") {
		t.Errorf("an unknown command says %q, want it to name itself and point at bex s3 env", msg)
	}
}

func TestMbAndRb(t *testing.T) {
	g := newRig(t)
	if out := g.must("mb", "s3://photos"); strings.TrimSpace(out) != "make_bucket: photos" {
		t.Errorf("mb = %q", out)
	}
	if b := g.s3.bucket("photos"); b.location != "oregon" {
		t.Errorf("default location = %q, want oregon", b.location)
	}
	if msg := g.fails("mb", "s3://photos"); !strings.Contains(msg, "already exists in this workspace") {
		t.Errorf("a second mb says %q", msg)
	}

	g.s3.put("photos", "a/1", []byte("x"), testNow)
	g.s3.put("photos", "a/2", []byte("x"), testNow)
	if msg := g.fails("rb", "s3://photos"); !strings.Contains(msg, "--force") || !strings.Contains(msg, "seven days") {
		t.Errorf("rb of a bucket with objects says %q, want --force and the sweep's seven days", msg)
	}
	if len(g.s3.keys("photos")) != 2 {
		t.Error("rb without --force deleted objects")
	}
	out := g.must("rb", "s3://photos", "--force")
	if !strings.Contains(out, "delete: s3://photos/a/1") || !strings.Contains(out, "remove_bucket: photos") {
		t.Errorf("rb --force = %q", out)
	}
	if _, ok := g.s3.buckets["photos"]; ok {
		t.Error("the bucket is still there")
	}

	// The reserved namespace is refused before a request, with --force too.
	g.s3.put("render-oregon", "k", []byte("x"), testNow)
	g.s3.reset()
	if msg := g.fails("rb", "s3://render-oregon", "--force"); !strings.Contains(msg, "system bucket") {
		t.Errorf("rb of a system bucket says %q", msg)
	}
	if len(g.s3.requests("")) != 0 || len(g.s3.keys("render-oregon")) != 1 {
		t.Errorf("a refused rb --force of a system bucket made %d requests and left %v", len(g.s3.requests("")), g.s3.keys("render-oregon"))
	}

	// Unfinished multipart uploads keep a bucket from being removed.
	g.must("mb", "s3://busy")
	g.s3.uploads["u1"] = &fakeUpload{bucket: "busy", key: "k", parts: map[int][]byte{}}
	if msg := g.fails("rb", "s3://busy", "--force"); !strings.Contains(msg, "multipart") || !strings.Contains(msg, "seven days") {
		t.Errorf("rb of a bucket with an unfinished upload says %q", msg)
	}
}

func TestPresignSignsAURLThatDiesWithTheKey(t *testing.T) {
	g := newRig(t)
	out := g.must("presign", "s3://photos/dir/cat one.jpg", "--expires-in", "300")
	raw := strings.TrimSpace(out)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("presign printed %q: %v", out, err)
	}
	q := u.Query()
	if u.Path != "/photos/dir/cat%20one.jpg" && u.Path != "/photos/dir/cat one.jpg" {
		t.Errorf("path = %q, want the object path-style", u.Path)
	}
	if u.Host != strings.TrimPrefix(g.s3.srv.URL, "https://") {
		t.Errorf("host = %q, want the endpoint", u.Host)
	}
	if !strings.HasPrefix(q.Get("X-Amz-Credential"), testKeyID+"/") || q.Get("X-Amz-Expires") != "300" || q.Get("X-Amz-Signature") == "" {
		t.Errorf("query = %v, want the key, 300 seconds and a signature", q)
	}
	if len(g.s3.requests("")) != 0 {
		t.Error("presign made a request to the endpoint")
	}
	// 300 seconds fits in the key's twelve hours: no warning.
	_, stderr, _ := g.bex("presign", "s3://photos/k", "--expires-in", "300")
	if stderr != "" {
		t.Errorf("a short URL warned: %q", stderr)
	}
	// Seven days outlive a twelve-hour key, and the command says when the URL really dies.
	stdout, stderr, err := g.bex("presign", "s3://photos/k", "--expires-in", "604800", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "stops working at 2026-11-20T22:00:00Z") || !strings.Contains(stderr, "keys create") {
		t.Errorf("stderr = %q, want the key's expiry and the persistent-key advice", stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil || doc["expiresIn"] != float64(12*3600) || doc["expiresAt"] != "2026-11-20T22:00:00Z" {
		t.Errorf("presign -o json = %v, %v", doc, err)
	}
	// A persistent key has no such limit.
	g.env[accessKeyEnv], g.env[secretKeyEnv] = "s3k-persistentkey0000000", "secret"
	if _, stderr, _ := g.bex("presign", "s3://photos/k", "--expires-in", "604800"); stderr != "" {
		t.Errorf("a URL signed by a persistent key warned: %q", stderr)
	}
}

// TestPresignedURLIsServedWithNoKey: the URL it prints works with nothing but
// the URL, as the gateway reads a presigned request.
func TestPresignedURLIsServedWithNoKey(t *testing.T) {
	g := newRig(t)
	g.s3.put("photos", "cat.jpg", []byte("meow"), testNow)
	raw := strings.TrimSpace(g.must("presign", "s3://photos/cat.jpg"))
	resp, err := g.s3.srv.Client().Get(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "meow" {
		t.Errorf("GET %s = %d %q", "<presigned>", resp.StatusCode, body)
	}
}

func TestListingAnEmptyOrMissingBucket(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://empty")
	if out := g.must("ls", "s3://empty"); out != "" {
		t.Errorf("ls of an empty bucket = %q", out)
	}
	if msg := g.fails("ls", "s3://nope"); !strings.Contains(msg, "NoSuchBucket") {
		t.Errorf("ls of a missing bucket says %q", msg)
	}
	var doc lsOutput
	if err := json.Unmarshal([]byte(g.must("ls", "s3://empty", "-o", "json")), &doc); err != nil || doc.Objects == nil || doc.CommonPrefixes == nil {
		t.Errorf("an empty listing as data = %+v, %v; want empty lists and not null", doc, err)
	}
}

func TestResultsAsYAMLAndJSONAgreeWithText(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	g.s3.put("photos", "a", []byte("x"), testNow)
	text := g.must("rm", "s3://photos/a")
	g.s3.put("photos", "a", []byte("x"), testNow)
	var doc struct{ Results []result }
	if err := json.Unmarshal([]byte(g.must("rm", "s3://photos/a", "-o", "json")), &doc); err != nil || len(doc.Results) != 1 {
		t.Fatalf("json: %+v, %v", doc, err)
	}
	if want := fmt.Sprintf("%s: %s", doc.Results[0].Action, doc.Results[0].Target); strings.TrimSpace(text) != want {
		t.Errorf("text %q and json %q disagree", strings.TrimSpace(text), want)
	}
	g.s3.put("photos", "a", []byte("x"), testNow)
	if out := g.must("rm", "s3://photos/a", "-o", "yaml"); !strings.Contains(out, "action: delete") {
		t.Errorf("yaml = %q", out)
	}
}
