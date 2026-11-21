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
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const corsFile = `{
  "CORSRules": [
    {"ID": "site", "AllowedOrigins": ["https://example.com", "https://www.example.com"], "AllowedMethods": ["GET", "HEAD"],
     "AllowedHeaders": ["*"], "ExposeHeaders": ["ETag"], "MaxAgeSeconds": 600},
    {"AllowedOrigins": ["*"], "AllowedMethods": ["GET"]}
  ]
}`

const lifecycleFile = `{
  "Rules": [
    {"ID": "logs", "Status": "Enabled", "Filter": {"Prefix": "logs/"}, "Expiration": {"Days": 7},
     "AbortIncompleteMultipartUpload": {"DaysAfterInitiation": 3}},
    {"ID": "dated", "Status": "Disabled", "Filter": {"Prefix": ""}, "Expiration": {"Date": "2030-01-01T00:00:00Z"}},
    {"ID": "legacy", "Status": "Enabled", "Prefix": "old/", "Expiration": {"Days": 30}}
  ]
}`

// TestCORSAndLifecycleRulesReadBackWhatWasSet: `get -o json` prints a document
// `set` accepts, and the two are the same rules.
func TestCORSAndLifecycleRulesReadBackWhatWasSet(t *testing.T) {
	for _, tc := range []struct {
		group, file string
		table       []string
		none        string
	}{
		{"cors", corsFile, []string{"site", "https://example.com,https://www.example.com", "GET,HEAD", "ETag", "600s"}, "no CORS rules"},
		{"lifecycle", lifecycleFile, []string{"logs", "Enabled", "logs/", "after 7 days", "after 3 days", "on 2030-01-01", "legacy", "old/"}, "no lifecycle rules"},
	} {
		t.Run(tc.group, func(t *testing.T) {
			g := newRig(t)
			g.must("mb", "s3://photos")
			if out := g.must(tc.group, "get", "photos"); strings.TrimSpace(out) != tc.none {
				t.Errorf("get with no rules = %q, want %q", out, tc.none)
			}
			var empty map[string][]any
			if err := json.Unmarshal([]byte(g.must(tc.group, "get", "photos", "-o", "json")), &empty); err != nil {
				t.Fatal(err)
			}
			for _, rules := range empty {
				if rules == nil || len(rules) != 0 {
					t.Errorf("get -o json with no rules = %v, want an empty list", empty)
				}
			}

			path := filepath.Join(t.TempDir(), "rules.json")
			writeBytes(t, filepath.Dir(path), "rules.json", []byte(tc.file))
			if out := g.must(tc.group, "set", "photos", "--file", path); !strings.Contains(out, "put_"+tc.group+": photos") {
				t.Errorf("set printed %q", out)
			}
			got := g.must(tc.group, "get", "photos", "-o", "json")
			if !sameJSON(t, got, tc.file) {
				t.Errorf("get =\n%s\nwant the rules of\n%s", got, tc.file)
			}
			table := g.must(tc.group, "get", "s3://photos")
			for _, want := range tc.table {
				if !strings.Contains(table, want) {
					t.Errorf("the table lacks %q:\n%s", want, table)
				}
			}
			// What get printed is a document set accepts, and setting it again changes nothing.
			writeBytes(t, filepath.Dir(path), "again.json", []byte(got))
			g.must(tc.group, "set", "photos", "--file", filepath.Join(filepath.Dir(path), "again.json"))
			if again := g.must(tc.group, "get", "photos", "-o", "json"); !sameJSON(t, again, got) {
				t.Errorf("get after set of get =\n%s\nwant\n%s", again, got)
			}
			// Standard input is a file too.
			g.stdin.WriteString(tc.file)
			g.must(tc.group, "set", "photos", "--file", "-")

			if out := g.must(tc.group, "delete", "photos"); !strings.Contains(out, "delete_"+tc.group+": photos") {
				t.Errorf("delete printed %q", out)
			}
			if out := g.must(tc.group, "get", "photos"); strings.TrimSpace(out) != tc.none {
				t.Errorf("get after delete = %q", out)
			}
		})
	}
}

// sameJSON reports whether two JSON documents hold the same data.
func sameJSON(t *testing.T, a, b string) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal([]byte(a), &x); err != nil {
		t.Fatalf("%v in %s", err, a)
	}
	if err := json.Unmarshal([]byte(b), &y); err != nil {
		t.Fatalf("%v in %s", err, b)
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return string(xa) == string(ya)
}

func TestARulesFileThatIsWrongNamesTheLine(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	minted := g.api.mintCount()
	dir := t.TempDir()
	for _, tc := range []struct {
		group, name, content string
		want                 []string
	}{
		{"cors", "syntax.json", "{\n  \"CORSRules\": [\n    {\"AllowedOrigins\": [\"*\"],,\n  ]\n}", []string{"syntax.json: line 3"}},
		{"cors", "type.json", "{\n  \"CORSRules\": [\n    {\"AllowedOrigins\": \"*\", \"AllowedMethods\": [\"GET\"]}\n  ]\n}", []string{"type.json: line 3"}},
		{"cors", "unknown.json", "{\n  \"CORSRules\": [\n    {\"AllowedOrigin\": [\"*\"]}\n  ]\n}", []string{"unknown.json: line 3", "AllowedOrigin"}},
		{"cors", "empty.json", `{"CORSRules": []}`, []string{"no CORSRules", "cors delete"}},
		{"cors", "bare.json", `[{"AllowedOrigins": ["*"]}]`, []string{"bare.json: line 1"}},
		{"lifecycle", "transition.json", "{\n \"Rules\": [\n  {\"Status\": \"Enabled\", \"Transitions\": [{\"Days\": 1}]}\n ]\n}", []string{"transition.json: line 3", "Transitions"}},
		{"lifecycle", "two.json", `{"Rules":[{"Status":"Enabled"}]} {"Rules":[]}`, []string{"more than one"}},
	} {
		path := filepath.Join(dir, tc.name)
		writeBytes(t, dir, tc.name, []byte(tc.content))
		g.s3.reset()
		msg := g.fails(tc.group, "set", "photos", "--file", path)
		for _, want := range append(tc.want, "") {
			if !strings.Contains(msg, want) {
				t.Errorf("%s: error %q does not say %q", tc.name, msg, want)
			}
		}
		if len(g.s3.requests("")) != 0 || g.api.mintCount() != minted {
			t.Errorf("%s: a bad file reached the server or cost a key", tc.name)
		}
	}
}

func TestRulesTheServerDoesNotStoreAnswerWithItsOwnMessage(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	// A rule set the server cannot store is sent as it is, and the server's answer is printed.
	path := filepath.Join(dir, "r.json")
	writeBytes(t, dir, "r.json", []byte(`{"Rules":[{"ID":"x","Status":"Enabled","Filter":{"Prefix":"a/"},"Expiration":{"Days":1}}]}`))
	g.s3.intercept = func(r *http.Request, _ int) (int, string, string, bool) {
		return 501, "NotImplemented", "Transition rules are not implemented", r.Method == "PUT"
	}
	msg := g.fails("lifecycle", "set", "photos", "--file", path)
	if !strings.Contains(msg, "does not serve this operation") || !strings.Contains(msg, "Transition rules are not implemented") {
		t.Errorf("a 501 says %q, want the server's own words", msg)
	}
	g.s3.intercept = nil
	if msg := g.fails("cors", "get", "missing-bucket"); !strings.Contains(msg, "there is no bucket missing-bucket") {
		t.Errorf("a missing bucket says %q", msg)
	}
}

func TestInfoOfAnObject(t *testing.T) {
	g := newRig(t)
	g.s3.put("photos", "dir/cat.jpg", make([]byte, 1536), testNow)
	g.s3.mu.Lock()
	o := g.s3.buckets["photos"].objects["dir/cat.jpg"]
	o.contentType, o.metadata = "image/jpeg", map[string]string{"mtime": "1700000000", "owner": "ada"}
	g.s3.mu.Unlock()
	out := g.must("info", "s3://photos/dir/cat.jpg")
	for _, want := range []string{"s3://photos/dir/cat.jpg", "1.5 KiB (1536 bytes)", "image/jpeg", "Metadata mtime", "1700000000", "Metadata owner", "ada", "2026-11-20 10:00:00 UTC"} {
		if !strings.Contains(out, want) {
			t.Errorf("info lacks %q:\n%s", want, out)
		}
	}
	var doc objectInfo
	if err := json.Unmarshal([]byte(g.must("info", "s3://photos/dir/cat.jpg", "-o", "json")), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Size != 1536 || doc.Key != "dir/cat.jpg" || doc.Metadata["owner"] != "ada" || doc.ContentType != "image/jpeg" || doc.ETag == "" {
		t.Errorf("info -o json = %+v", doc)
	}
	if msg := g.fails("info", "s3://photos/nope"); !strings.Contains(msg, "there is no object s3://photos/nope") {
		t.Errorf("a missing object says %q", msg)
	}
	// A bucket, with or without the scheme, is bex's own route, as before.
	if out := g.must("info", "s3://photos/"); !strings.Contains(out, "frankfurt") {
		t.Errorf("info of a bucket via s3:// = %q", out)
	}
}
