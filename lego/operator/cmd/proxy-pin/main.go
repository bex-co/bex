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

// proxy-pin tells deploy.yml which operator digest to pin for one SNI proxy
// DaemonSet (w6/m470, docs/ADR021-keyvalue-management.md § 10). The answer is
// the current pin while the proxy's binary is byte-identical in the freshly
// built image, so a deploy that leaves the proxy's code alone rolls none of
// its pods and closes none of its connections. Otherwise it is the new
// digest. Any doubt (a registry error, a missing build step) also answers
// the new digest, which every deploy pinned before. It prints exactly one
// digest on stdout and its reason on stderr, and exits non-zero only on bad
// arguments. It is not built into the image.
//
//	go run ./cmd/proxy-pin -image ghcr.io/bex-co/bex-operator \
//	  -current sha256:… -candidate sha256:… -file /kv-sni-proxy
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/registry"
)

// readTimeout bounds both image reads: the pin must never hold a deploy.
const readTimeout = 3 * time.Minute

// fileSHA256 reads the proxy binary's sha256 out of the image at digest.
type fileSHA256 func(ctx context.Context, digest string) (string, error)

// fileReader is the registry read for one image file; tests replace it.
type fileReader func(registryHost, repo string, platform registry.Platform, step, file string) fileSHA256

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, registryFileReader))
}

func registryFileReader(registryHost, repo string, platform registry.Platform, step, file string) fileSHA256 {
	return func(ctx context.Context, digest string) (string, error) {
		return registry.ImageFileSHA256(ctx, nil, registryHost, repo, digest, platform, step, file)
	}
}

// binaryPath is a top-level binary as lego/Dockerfile copies one in.
var binaryPath = regexp.MustCompile(`^/[a-z0-9][a-z0-9-]*$`)

func run(args []string, stdout, stderr io.Writer, reader fileReader) int {
	flags := flag.NewFlagSet("proxy-pin", flag.ContinueOnError)
	flags.SetOutput(stderr)
	image := flags.String("image", "", "image repository with its registry host, e.g. ghcr.io/bex-co/bex-operator")
	current := flags.String("current", "", "the digest pinned now (sha256:…)")
	candidate := flags.String("candidate", "", "the freshly built digest (sha256:…)")
	file := flags.String("file", "", "the proxy binary's path in the image, e.g. /kv-sni-proxy")
	platformFlag := flags.String("platform", "linux/amd64",
		"the platform to compare: production's linux/amd64, or linux/arm64")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	registryHost, repo, imageOK := splitImage(*image)
	platform, platformErr := registry.ParsePlatform(*platformFlag)
	switch {
	case flags.NArg() != 0:
		return usage(stderr, "unexpected arguments %q", flags.Args())
	case !imageOK:
		return usage(stderr, "-image %q is not a registry host and repository without a tag or digest", *image)
	case !registry.IsDigest(*current):
		return usage(stderr, "-current %q is not a digest", *current)
	case !registry.IsDigest(*candidate):
		return usage(stderr, "-candidate %q is not a digest", *candidate)
	case !binaryPath.MatchString(*file):
		return usage(stderr, "-file %q is not a top-level binary path such as /kv-sni-proxy", *file)
	case platformErr != nil:
		return usage(stderr, "-platform %q is not linux/amd64 or linux/arm64", *platformFlag)
	}
	name := strings.TrimPrefix(*file, "/")
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	digest, reason := pin(ctx, *file, *current, *candidate, reader(registryHost, repo, platform, copyStep(name), name))
	_, _ = fmt.Fprintln(stderr, reason)
	_, _ = fmt.Fprintln(stdout, digest)
	return 0
}

func usage(stderr io.Writer, format string, args ...any) int {
	_, _ = fmt.Fprintf(stderr, "proxy-pin: "+format+"\n", args...)
	return 2
}

// splitImage splits ghcr.io/bex-co/bex-operator into its registry host and
// repository. A tag or digest is refused: the digests come separately.
func splitImage(image string) (registryHost, repo string, ok bool) {
	registryHost, repo, found := strings.Cut(image, "/")
	if !found || repo == "" || !strings.ContainsAny(registryHost, ".:") || strings.ContainsAny(repo, "@:") {
		return "", "", false
	}
	return registryHost, repo, true
}

// copyStep is the image config history entry lego/Dockerfile's
// `COPY --from=builder /workspace/<name> /<name>` leaves.
func copyStep(name string) string {
	return "COPY /workspace/" + name + " /" + name + " # buildkit"
}

// pin decides the digest: current while both images carry the same file,
// candidate otherwise. A failed read is a doubt, and a doubt moves the pin.
func pin(ctx context.Context, file, current, candidate string, sha256Of fileSHA256) (digest, reason string) {
	if current == candidate {
		return candidate, "kept: " + file + " is already pinned to the candidate"
	}
	held, err := sha256Of(ctx, current)
	if err != nil {
		return candidate, fmt.Sprintf("moved: %s in the current image is unknown: %v", file, err)
	}
	built, err := sha256Of(ctx, candidate)
	if err != nil {
		return candidate, fmt.Sprintf("moved: %s in the candidate image is unknown: %v", file, err)
	}
	if held != built {
		return candidate, fmt.Sprintf("moved: %s changed (%s → %s)", file, held, built)
	}
	return current, fmt.Sprintf("kept: %s %s unchanged", file, held)
}
