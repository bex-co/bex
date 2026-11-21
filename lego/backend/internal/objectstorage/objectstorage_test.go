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
	"strconv"
	"strings"
	"testing"
	"time"

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
	// Render's other regions, near misses and junk.
	for _, bad := range []string{"ohio", "virginia", "singapore", "wnam", "weur", "eu", "", "Oregon", "oregon ", "us-west-1"} {
		_, err := ParseRegion(bad)
		var named *RegionNotOfferedError
		if !errors.Is(err, ErrRegionNotOffered) || !errors.As(err, &named) || named.Value != bad {
			t.Errorf("ParseRegion(%q) error = %v, want a RegionNotOfferedError naming it", bad, err)
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

// TestValidateKey checks the cases of lego/cli/testdata/object-keys.txt, which
// `bex s3`'s own copy of the rule is tested against too (w32/063): the CLI
// imports no sibling module, so a change to the rule here must change that
// table, and the CLI with it.
func TestValidateKey(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "object-keys.txt"))
	if err != nil {
		t.Fatal(err)
	}
	limit := 0
	var valid, invalid []string
	for n, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kind, rest, _ := strings.Cut(line, " ")
		if kind == "max-key-bytes" {
			if limit, err = strconv.Atoi(rest); err != nil {
				t.Fatalf("object-keys.txt:%d: %v", n+1, err)
			}
			continue
		}
		key, err := strconv.Unquote(rest)
		if err != nil {
			t.Fatalf("object-keys.txt:%d: %s is not a Go string literal: %v", n+1, rest, err)
		}
		switch kind {
		case "valid":
			valid = append(valid, key)
		case "invalid":
			invalid = append(invalid, key)
		default:
			t.Fatalf("object-keys.txt:%d: %q is not max-key-bytes, valid or invalid", n+1, kind)
		}
	}
	if limit != MaxKeyBytes {
		t.Fatalf("object-keys.txt has max-key-bytes %d, and MaxKeyBytes is %d", limit, MaxKeyBytes)
	}
	if len(valid) == 0 || len(invalid) == 0 {
		t.Fatalf("object-keys.txt has %d valid and %d invalid keys", len(valid), len(invalid))
	}
	valid = append(valid, strings.Repeat("k", limit))
	invalid = append(invalid, strings.Repeat("k", limit+1), strings.Repeat("é", limit/2+1))
	for _, key := range valid {
		if err := ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) = %v, want nil", key, err)
		}
	}
	for _, key := range invalid {
		if err := ValidateKey(key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ValidateKey(%q) = %v, want ErrInvalidKey", key, err)
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
	if MaxObjectBytesPresigned != 5<<30 || MaxConcurrentStreamsPerWorkspace != 16 {
		t.Error("object and stream caps differ from ADR112 D8")
	}
	if ListDefaultLimit != 20 || ListMaxLimit != 1000 || MaxKeyBytes != 1024 {
		t.Error("list and key limits differ from ADR112 D2 and D7")
	}
}

// TestPacesLetAMaximumObjectFinishInsideTheStreamDeadline pins the arithmetic behind
// ADR112 D3's "up to 5 GiB for up to 30 minutes" (w21/114). The pace is the one thing
// bex itself puts between a client and StreamDeadline, so a pace under what a
// maximum-size object needs (5 GiB in 30 minutes is 2.84 MiB/s) would make the largest
// legal PutObject unfinishable by construction, which is what a pace of 1 MiB/s does
// (1.76 GiB). Twice the minimum leaves room for one more stream of the same workspace,
// which shares the pace.
func TestPacesLetAMaximumObjectFinishInsideTheStreamDeadline(t *testing.T) {
	need := MaxObjectBytesPresigned / int64(StreamDeadline/time.Second)
	for name, pace := range map[string]int64{"upload": S3WriteBytesPerSecond, "download": S3ReadBytesPerSecond} {
		if pace < 2*need {
			t.Errorf("the %s pace of %d B/s moves %d B in %v, under twice the largest object (%d B): a maximum-size object would not finish",
				name, pace, pace*int64(StreamDeadline/time.Second), StreamDeadline, MaxObjectBytesPresigned)
		}
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
