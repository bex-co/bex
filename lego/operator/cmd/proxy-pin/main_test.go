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

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/registry"
)

var (
	held    = "sha256:" + strings.Repeat("a", 64)
	built   = "sha256:" + strings.Repeat("b", 64)
	errRead = errors.New("image layer request: status 503")
)

// images is a registry read keyed by digest that counts its reads.
type images struct {
	files map[string]string
	fail  map[string]bool
	reads int
}

func (i *images) sha256Of(_ context.Context, digest string) (string, error) {
	i.reads++
	if i.fail[digest] {
		return "", errRead
	}
	return i.files[digest], nil
}

// w6/m470: the pin stays while the proxy binary is byte-identical, so an
// operator deploy that leaves its code alone rolls no SNI proxy; every other
// answer, a doubt included, moves it, as every deploy did before.
func TestPinKeepsOnlyAByteIdenticalBinary(t *testing.T) {
	for name, c := range map[string]struct {
		current string
		files   map[string]string
		fail    map[string]bool
		want    string
		reason  string
		reads   int
	}{
		"the same binary keeps the current pin": {
			files: map[string]string{held: "f00d", built: "f00d"},
			want:  held, reason: "kept: /kv-sni-proxy f00d unchanged", reads: 2,
		},
		"a changed binary moves the pin": {
			files: map[string]string{held: "f00d", built: "beef"},
			want:  built, reason: "moved: /kv-sni-proxy changed (f00d → beef)", reads: 2,
		},
		"an unreadable current image moves the pin": {
			files: map[string]string{built: "f00d"}, fail: map[string]bool{held: true},
			want: built, reason: "moved: /kv-sni-proxy in the current image is unknown: " + errRead.Error(), reads: 1,
		},
		"an unreadable candidate image moves the pin": {
			files: map[string]string{held: "f00d"}, fail: map[string]bool{built: true},
			want: built, reason: "moved: /kv-sni-proxy in the candidate image is unknown: " + errRead.Error(), reads: 2,
		},
		"a pin that is already the candidate reads nothing": {
			current: built,
			want:    built, reason: "kept: /kv-sni-proxy is already pinned to the candidate", reads: 0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			reg := &images{files: c.files, fail: c.fail}
			current := held
			if c.current != "" {
				current = c.current
			}
			digest, reason := pin(context.Background(), "/kv-sni-proxy", current, built, reg.sha256Of)
			if digest != c.want || reason != c.reason {
				t.Fatalf("pin = %s, %q; want %s, %q", digest, reason, c.want, c.reason)
			}
			if reg.reads != c.reads {
				t.Fatalf("%d registry reads, want %d", reg.reads, c.reads)
			}
		})
	}
}

// The command reads the file, build step and platform it was asked for, and
// prints exactly one digest, which deploy.yml writes into the pin.
func TestRunPrintsOneDigestForTheNamedFileAndPlatform(t *testing.T) {
	var asked struct {
		host, repo, step, file string
		platform               registry.Platform
	}
	reader := func(registryHost, repo string, platform registry.Platform, step, file string) fileSHA256 {
		asked.host, asked.repo, asked.platform, asked.step, asked.file = registryHost, repo, platform, step, file
		return (&images{files: map[string]string{held: "f00d", built: "f00d"}}).sha256Of
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-image", "ghcr.io/bex-co/bex-operator", "-current", held, "-candidate", built,
		"-file", "/pg-sni-proxy"}, &stdout, &stderr, reader)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.String() != held+"\n" {
		t.Fatalf("stdout %q, want exactly the held digest", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "kept: /pg-sni-proxy f00d unchanged") {
		t.Fatalf("stderr %q, want the kept reason", stderr.String())
	}
	want := registry.Platform{OS: "linux", Architecture: "amd64"}
	if asked.host != "ghcr.io" || asked.repo != "bex-co/bex-operator" || asked.platform != want ||
		asked.step != "COPY /workspace/pg-sni-proxy /pg-sni-proxy # buildkit" || asked.file != "pg-sni-proxy" {
		t.Fatalf("read %+v; want ghcr.io bex-co/bex-operator, linux/amd64 (production's, whatever the runner's), "+
			"pg-sni-proxy's COPY step", asked)
	}

	stdout.Reset()
	if code := run([]string{"-image", "ghcr.io/bex-co/bex-operator", "-current", held, "-candidate", built,
		"-file", "/kv-sni-proxy", "-platform", "linux/arm64"}, &stdout, &stderr, reader); code != 0 {
		t.Fatalf("linux/arm64: exit %d", code)
	}
	if asked.platform.Architecture != "arm64" {
		t.Fatalf("-platform linux/arm64 read %+v", asked.platform)
	}
}

// Only bad arguments fail the command, and they read nothing.
func TestRunRefusesBadArguments(t *testing.T) {
	valid := map[string]string{
		"-image": "ghcr.io/bex-co/bex-operator", "-current": held, "-candidate": built, "-file": "/kv-sni-proxy",
	}
	for name, override := range map[string]map[string]string{
		"a tag as the current pin":  {"-current": "latest"},
		"a short candidate digest":  {"-candidate": "sha256:abc"},
		"an image with a tag":       {"-image": "ghcr.io/bex-co/bex-operator:latest"},
		"an image with no registry": {"-image": "bex-co/bex-operator"},
		"a nested file":             {"-file": "/bin/kv-sni-proxy"},
		"a relative file":           {"-file": "kv-sni-proxy"},
		"an unknown platform":       {"-platform": "linux/s390x"},
	} {
		t.Run(name, func(t *testing.T) {
			var args []string
			for _, flagName := range []string{"-image", "-current", "-candidate", "-file", "-platform"} {
				value, set := override[flagName]
				if !set {
					value, set = valid[flagName]
				}
				if set {
					args = append(args, flagName, value)
				}
			}
			reads := 0
			reader := func(string, string, registry.Platform, string, string) fileSHA256 {
				return func(context.Context, string) (string, error) { reads++; return "", nil }
			}
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr, reader); code != 2 {
				t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr.String())
			}
			if stdout.Len() != 0 || reads != 0 {
				t.Fatalf("stdout %q after %d reads; a refusal must print no digest and read nothing", stdout.String(), reads)
			}
		})
	}
}

// The build step the command derives is the one lego/Dockerfile's COPY of
// each SNI proxy leaves. If the COPY changes, every read fails as "no build
// step", and every deploy moves the pins again. This test fails first.
func TestCopyStepMatchesTheDockerfile(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "..", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kv-sni-proxy", "pg-sni-proxy"} {
		line := "COPY --from=builder /workspace/" + name + " /" + name + "\n"
		if !bytes.Contains(dockerfile, []byte(line)) {
			t.Errorf("lego/Dockerfile has no %q; copyStep(%q) = %q would match no build step",
				strings.TrimSpace(line), name, copyStep(name))
		}
	}
}
