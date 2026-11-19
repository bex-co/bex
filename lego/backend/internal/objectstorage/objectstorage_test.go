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
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestRegionConstantsMatchWasabi(t *testing.T) {
	cases := []struct {
		region                 Region
		wasabi, bucket, server string
	}{
		{Oregon, "us-west-1", "bex-objects-oregon", "https://s3.us-west-1.wasabisys.com"},
		{Frankfurt, "eu-central-2", "bex-objects-frankfurt", "https://s3.eu-central-2.wasabisys.com"},
	}
	for _, c := range cases {
		if c.region.WasabiRegion() != c.wasabi || c.region.Bucket() != c.bucket || c.region.Endpoint() != c.server {
			t.Errorf("%s = %s, %s, %s; want %s, %s, %s", c.region, c.region.WasabiRegion(), c.region.Bucket(), c.region.Endpoint(), c.wasabi, c.bucket, c.server)
		}
	}
	if DefaultRegion != Oregon {
		t.Errorf("DefaultRegion = %s, want oregon", DefaultRegion)
	}
}

func TestParseRegionServesOnlyOregonAndFrankfurt(t *testing.T) {
	for _, ok := range []string{"oregon", "frankfurt"} {
		if r, err := ParseRegion(ok); err != nil || string(r) != ok {
			t.Errorf("ParseRegion(%q) = %q, %v", ok, r, err)
		}
	}
	// Render's other regions, Cloudflare's hints, near misses and junk.
	for _, bad := range []string{"ohio", "virginia", "singapore", "wnam", "weur", "eu", "", "Oregon", "oregon ", "us-west-1"} {
		_, err := ParseRegion(bad)
		var named *RegionNotOfferedError
		if !errors.Is(err, ErrRegionNotOffered) || !errors.As(err, &named) || named.Value != bad {
			t.Errorf("ParseRegion(%q) error = %v, want a RegionNotOfferedError naming it", bad, err)
		}
	}
}

func TestRegionFromHint(t *testing.T) {
	cases := []struct {
		hint, jurisdiction string
		want               Region
		offered            bool
	}{
		{"", "", Oregon, true},
		{"wnam", "", Oregon, true},
		{"WNAM", "", Oregon, true},
		{"", "default", Oregon, true},
		{"weur", "", Frankfurt, true},
		{"", "eu", Frankfurt, true},
		{"weur", "eu", Frankfurt, true},
		{"", " EU ", Frankfurt, true},
		{"enam", "", "", false},
		{"eeur", "", "", false},
		{"apac", "", "", false},
		{"oc", "", "", false},
		{"wnam", "eu", "", false},
		{"", "fedramp", "", false},
		{"nonsense", "", "", false},
	}
	for _, c := range cases {
		got, err := RegionFromHint(c.hint, c.jurisdiction)
		if c.offered && (err != nil || got != c.want) {
			t.Errorf("RegionFromHint(%q, %q) = %q, %v; want %q", c.hint, c.jurisdiction, got, err, c.want)
		}
		if !c.offered && !errors.Is(err, ErrRegionNotOffered) {
			t.Errorf("RegionFromHint(%q, %q) = %q, %v; want ErrRegionNotOffered", c.hint, c.jurisdiction, got, err)
		}
	}
}

func TestValidateBucketName(t *testing.T) {
	valid := []string{"abc", "photos", "my-bucket-1", "0a0", strings.Repeat("a", 63)}
	for _, name := range valid {
		if err := ValidateBucketName(name); err != nil {
			t.Errorf("ValidateBucketName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", "a", "ab", "-abc", "abc-", "Abc", "a_b", "a.b", "a b", "ab/c", "é-bucket", strings.Repeat("a", 64)}
	for _, name := range invalid {
		if err := ValidateBucketName(name); !errors.Is(err, ErrInvalidBucketName) {
			t.Errorf("ValidateBucketName(%q) = %v, want ErrInvalidBucketName", name, err)
		}
	}
	for _, name := range []string{"render-oregon", "render-frankfurt", "render-ohio", "render-x"} {
		if err := ValidateBucketName(name); !errors.Is(err, ErrReservedBucketName) {
			t.Errorf("ValidateBucketName(%q) = %v, want ErrReservedBucketName", name, err)
		}
	}
	// Only the prefix is reserved, not the word.
	for _, name := range []string{"render", "renderer-assets", "my-render-bucket"} {
		if err := ValidateBucketName(name); err != nil {
			t.Errorf("ValidateBucketName(%q) = %v, want nil", name, err)
		}
	}
	for _, r := range Regions {
		if !IsSystemBucketName(SystemBucketName(r)) || ValidateBucketName(SystemBucketName(r)) == nil {
			t.Errorf("system bucket name %q must be reserved and so refused to a user", SystemBucketName(r))
		}
	}
}

func TestValidateKey(t *testing.T) {
	valid := []string{
		"a", "cat.jpg", "dir/cat.jpg", "a/b/c/d", "with space", "ünï/çödé", "日本語/キー",
		"trailing/", "dots.in.name", ".hidden", "..hidden", "name..", "a/.../b", "100%", "%2e%2e", "%252e%252e/x",
		"a?b#c", "a;b", strings.Repeat("k", MaxKeyBytes),
		// Look-alikes of ".." are ordinary names: S3 keys are opaque.
		"．．/x", "‥/x",
	}
	for _, key := range valid {
		if err := ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) = %v, want nil", key, err)
		}
	}
	invalid := map[string]string{
		"empty":                 "",
		"too long":              strings.Repeat("k", MaxKeyBytes+1),
		"too long in bytes":     strings.Repeat("é", MaxKeyBytes/2+1),
		"leading slash":         "/a",
		"only a slash":          "/",
		"dot":                   ".",
		"dot dot":               "..",
		"leading dot dot":       "../a",
		"inner dot dot":         "a/../b",
		"trailing dot dot":      "a/..",
		"dot segment":           "a/./b",
		"leading dot segment":   "./a",
		"trailing dot segment":  "a/.",
		"doubled slash then ..": "a//../b",
		"doubled slash":         "a//b",
		"doubled slash at end":  "a//",
		"doubled slash inside":  "dir//file.txt",
		"backslash":             `a\b`,
		"backslash dot dot":     `..\a`,
		"nul":                   "a\x00b",
		"newline":               "a\nb",
		"tab":                   "a\tb",
		"escape":                "a\x1bb",
		"delete":                "a\x7fb",
		"invalid utf-8":         "a\xffb",
		"truncated utf-8":       "a\xe2\x82",
	}
	for name, key := range invalid {
		if err := ValidateKey(key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%s: ValidateKey(%q) = %v, want ErrInvalidKey", name, key, err)
		}
	}
}

func TestObjectKeyLayoutRoundTrips(t *testing.T) {
	ws, bucket := ids.New(ids.Workspace), ids.New(ids.ObjectBucket)
	got, err := ObjectKey(ws, bucket, "dir/cat.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if want := ws + "/" + bucket + "/dir/cat.jpg"; got != want {
		t.Fatalf("ObjectKey = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, Prefix(ws, bucket)) || !strings.HasPrefix(Prefix(ws, bucket), WorkspacePrefix(ws)) {
		t.Fatalf("prefixes do not nest: %q, %q, %q", got, Prefix(ws, bucket), WorkspacePrefix(ws))
	}
	gws, gbucket, gkey, ok := ParseObjectKey(got)
	if !ok || gws != ws || gbucket != bucket || gkey != "dir/cat.jpg" {
		t.Fatalf("ParseObjectKey(%q) = %q, %q, %q, %v", got, gws, gbucket, gkey, ok)
	}
}

func TestObjectKeyRefusesWhatCouldLeaveItsPrefix(t *testing.T) {
	ws, bucket := ids.New(ids.Workspace), ids.New(ids.ObjectBucket)
	if _, err := ObjectKey(ws, bucket, "../"+ids.New(ids.ObjectBucket)+"/stolen"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("a .. key = %v, want ErrInvalidKey", err)
	}
	// The ids are checked too: a name or a path in either slot is not an id.
	for _, c := range []struct{ ws, bucket string }{
		{ws, "photos"},
		{ws, ids.New(ids.Service)},
		{ws, ws},
		{ids.New(ids.Service), bucket},
		{"", bucket},
		{ws, ""},
		{ws + "/x", bucket},
		{ws, bucket + "/.."},
	} {
		if _, err := ObjectKey(c.ws, c.bucket, "k"); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ObjectKey(%q, %q) = %v, want ErrInvalidKey", c.ws, c.bucket, err)
		}
	}
}

func TestParseObjectKeyRejectsJunk(t *testing.T) {
	ws, bucket := ids.New(ids.Workspace), ids.New(ids.ObjectBucket)
	for _, junk := range []string{
		"", "k", ws, ws + "/", ws + "/" + bucket, ws + "/" + bucket + "/",
		ws + "/photos/k", "tea/obk/k", ids.New(ids.Service) + "/" + bucket + "/k",
	} {
		if _, _, _, ok := ParseObjectKey(junk); ok {
			t.Errorf("ParseObjectKey(%q) accepted junk", junk)
		}
	}
}

// TestCapsAreCodeConstantsNotConfiguration pins what ADR112 D1, D8 and D9
// fixed, so a change to one is a visible, reviewed edit of this test too.
func TestCapsAreCodeConstantsNotConfiguration(t *testing.T) {
	if MaxWorkspaceBytes != 100_000_000_000 || MaxWorkspaceObjects != 1_000_000 || MaxUserBuckets != 1000 {
		t.Error("workspace caps differ from ADR112 D8")
	}
	if MaxObjectBytesPresigned != 5<<30 || MaxObjectBytesStreamed != 300<<20 || MaxConcurrentStreamsPerWorkspace != 16 {
		t.Error("object and stream caps differ from ADR112 D8")
	}
	if ListDefaultLimit != 20 || ListMaxLimit != 1000 || MaxKeyBytes != 1024 {
		t.Error("list and key limits differ from ADR112 D2 and D7")
	}
}

// TestPackageReadsNoEnvironmentVariable enforces ADR112 D9: an environment
// variable carries a credential and nothing else, and the credentials are read
// by cmd/api, not here. A change that reads one in this package is configuration
// by the back door.
func TestPackageReadsNoEnvironmentVariable(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no sources found: %v", err)
	}
	reads := map[string]bool{"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true, "Expand": true}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && reads[sel.Sel.Name] {
				t.Errorf("%s reads the environment with os.%s; ADR112 D9 allows credentials only, read by cmd/api", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}
