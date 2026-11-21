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
	"maps"
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
		// The decoder would read the Latin-1 byte as U+FFFD and send an origin the file does not name.
		{"cors", "latin1.json", "{\n  \"CORSRules\": [\n    {\"AllowedOrigins\": [\"https://caf\xe9.example\"], \"AllowedMethods\": [\"GET\"]}\n  ]\n}", []string{"latin1.json: line 3: the file is not UTF-8"}},
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

// TestALifecyclePrefixOrDateBexCannotSendAsWrittenIsAWrongCommandLine: the
// SDK's XML encoder sends a control character other than tab, newline or
// carriage return as U+FFFD, and the gateway stored "x\uFFFD/", a prefix real
// keys match, for a file that said "x\u0007/" (w32/067). A bare date failed
// with Go's layout string. Each is refused with exit 2, naming the rule, before
// a key is made or a request sent.
func TestALifecyclePrefixOrDateBexCannotSendAsWrittenIsAWrongCommandLine(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	minted := g.api.mintCount()
	g.s3.reset()
	dir := t.TempDir()
	rule := func(id, prefix string) string {
		return `{"ID":"` + id + `","Status":"Enabled","Filter":{"Prefix":"` + prefix + `"},"Expiration":{"Days":1}}`
	}
	for _, tc := range []struct {
		name, content, want string
	}{
		{"ctl.json", `{"Rules":[` + rule("c", `x\u0007/`) + `]}`, `lifecycle rule 1 ("c"): Filter.Prefix "x\a/" has a control character (` + prefixRule + `)`},
		{"tab.json", `{"Rules":[` + rule("t", `x\t/`) + `]}`, `lifecycle rule 1 ("t"): Filter.Prefix "x\t/" has a control character`},
		{"newline.json", `{"Rules":[` + rule("n", `x\n/`) + `]}`, `lifecycle rule 1 ("n"): Filter.Prefix "x\n/" has a control character`},
		{"del.json", `{"Rules":[` + rule("d", `x\u007f/`) + `]}`, `lifecycle rule 1 ("d"): Filter.Prefix "x\x7f/" has a control character`},
		{"c1.json", `{"Rules":[` + rule("c1", `x\u0085/`) + `]}`, `lifecycle rule 1 ("c1"): Filter.Prefix "x\u0085/" has a control character`},
		{"long.json", `{"Rules":[` + rule("l", strings.Repeat("p", maxKeyBytes+1)) + `]}`, `lifecycle rule 1 ("l"): Filter.Prefix "` + strings.Repeat("p", maxKeyBytes+1) + `" is 1025 bytes long`},
		{"legacy.json", `{"Rules":[` + rule("ok", "a/") + `,{"Status":"Enabled","Prefix":"x\u0001","Expiration":{"Days":1}}]}`, `lifecycle rule 2: Prefix "x\x01" has a control character`},
		{"date.json", `{"Rules":[{"ID":"d","Status":"Enabled","Filter":{"Prefix":"x/"},"Expiration":{"Date":"2027-1-1"}}]}`, `lifecycle rule 1 ("d"): Expiration.Date "2027-1-1" must be a date such as 2027-01-01 or 2027-01-01T00:00:00Z`},
		{"us-date.json", `{"Rules":[{"Status":"Enabled","Expiration":{"Date":"01/01/2027"}}]}`, `lifecycle rule 1: Expiration.Date "01/01/2027" must be a date such as 2027-01-01 or 2027-01-01T00:00:00Z`},
	} {
		writeBytes(t, dir, tc.name, []byte(tc.content))
		stdout, _, err := g.bex("lifecycle", "set", "photos", "--file", filepath.Join(dir, tc.name))
		if err == nil {
			t.Errorf("%s: set succeeded, printing %q", tc.name, stdout)
			continue
		}
		if code := exitCode(err); code != 2 {
			t.Errorf("%s: set exited %d, want 2: %v", tc.name, code, err)
		}
		if !strings.Contains(err.Error(), tc.name+": "+tc.want) {
			t.Errorf("%s: set said %q\nwant %q", tc.name, err, tc.want)
		}
	}
	if n := g.s3.count(""); n != 0 {
		t.Errorf("a refused rule still sent %d S3 requests: %v", n, g.s3.requests(""))
	}
	if n := g.api.mintCount(); n != minted {
		t.Errorf("a refused rule still made %d access keys", n-minted)
	}
}

// TestALifecycleDateMayBeABareDate: a bare date is midnight UTC, the only time
// the gateway takes, and is sent in RFC 3339 (w32/067). A prefix that is not
// ASCII is an ordinary prefix, and get counts one day as a day.
func TestALifecycleDateMayBeABareDate(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeBytes(t, dir, "date.json", []byte(`{"Rules":[
	  {"ID":"d","Status":"Enabled","Filter":{"Prefix":"x/"},"Expiration":{"Date":"2027-01-01"}},
	  {"ID":"one","Status":"Enabled","Filter":{"Prefix":"фото/\uFFFD"},"Expiration":{"Days":1},"AbortIncompleteMultipartUpload":{"DaysAfterInitiation":1}},
	  {"ID":"week","Status":"Enabled","Filter":{"Prefix":"tmp/"},"Expiration":{"Days":7}}]}`))
	g.must("lifecycle", "set", "photos", "--file", filepath.Join(dir, "date.json"))
	g.s3.mu.Lock()
	sent := string(g.s3.buckets["photos"].lifecycle)
	g.s3.mu.Unlock()
	if !strings.Contains(sent, "<Date>2027-01-01T00:00:00Z</Date>") || !strings.Contains(sent, "<Prefix>фото/\uFFFD</Prefix>") {
		t.Errorf("set sent %s\nwant the Date 2027-01-01T00:00:00Z and the prefix as written", sent)
	}
	var doc struct {
		Rules []struct{ Expiration struct{ Date string } }
	}
	if err := json.Unmarshal([]byte(g.must("lifecycle", "get", "photos", "-o", "json")), &doc); err != nil {
		t.Fatal(err)
	}
	if date := doc.Rules[0].Expiration.Date; date != "2027-01-01T00:00:00Z" {
		t.Errorf("get -o json read the date back as %q, want 2027-01-01T00:00:00Z", date)
	}
	table := g.must("lifecycle", "get", "photos")
	for _, want := range []string{"on 2027-01-01", "after 1 day", "after 7 days"} {
		if !strings.Contains(table, want) {
			t.Errorf("the table lacks %q:\n%s", want, table)
		}
	}
	if strings.Contains(table, "1 days") {
		t.Errorf("the table counts one day as days:\n%s", table)
	}
}

// TestSetCountsOneRuleAsARule: `set` reports how many rules it stored, and a
// one-rule document is "(1 rule)", not "(1 rules)" (w32/072). With -o json the
// count is in the same result's target.
func TestSetCountsOneRuleAsARule(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	cors := `{"AllowedOrigins": ["*"], "AllowedMethods": ["GET"]}`
	lifecycle := `{"Status": "Enabled", "Filter": {"Prefix": "tmp/"}, "Expiration": {"Days": 1}}`
	for _, tc := range []struct {
		group, one, two string
	}{
		{"cors", `{"CORSRules": [` + cors + `]}`, `{"CORSRules": [` + cors + `, ` + cors + `]}`},
		{"lifecycle", `{"Rules": [` + lifecycle + `]}`, `{"Rules": [` + lifecycle + `, ` + lifecycle + `]}`},
	} {
		for _, doc := range []struct{ name, content, target string }{
			{"one.json", tc.one, "photos (1 rule)"},
			{"two.json", tc.two, "photos (2 rules)"},
		} {
			path := filepath.Join(dir, tc.group+"-"+doc.name)
			writeBytes(t, dir, tc.group+"-"+doc.name, []byte(doc.content))
			if out, want := strings.TrimSpace(g.must(tc.group, "set", "photos", "--file", path)), "put_"+tc.group+": "+doc.target; out != want {
				t.Errorf("%s set %s printed %q, want %q", tc.group, doc.name, out, want)
			}
			var report struct {
				Results []map[string]any `json:"results"`
			}
			if err := json.Unmarshal([]byte(g.must(tc.group, "set", "photos", "--file", path, "-o", "json")), &report); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"action": "put_" + tc.group, "target": doc.target}
			if len(report.Results) != 1 || !maps.Equal(report.Results[0], want) {
				t.Errorf("%s set %s -o json = %v, want one result %v", tc.group, doc.name, report.Results, want)
			}
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
