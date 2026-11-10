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

package build

import (
	"regexp"
	"strings"
)

// platformfetch.go names a native build that failed while fetching or
// preparing a toolchain bex supplies (w8/m148). A native build's Dockerfile is
// bex's own — the tenant supplies only its build command — so every image it
// pulls, every checksum-pinned ADD and every `bex-<tool>…` helper stage is a
// platform fetch, never the tenant's. On 2026-11-08 GitHub's release CDN
// answered 504 for the Hugo tarball and every static build ended "build failed
// in the docker build step: … digest mismatch sha256:…", read as the tenant's
// failure. The raw BuildKit lines stay in the build log; the deploy's reason
// says which download failed and that a retry is the remedy.

var (
	// buildkitStepHeader is BuildKit's progress header for step N:
	// `#15 [bex-hugo-amd64 1/1] ADD --checksum=… https://…`.
	buildkitStepHeader = regexp.MustCompile(`^#([0-9]+) \[([^\]]+)\] (.*)$`)
	buildkitStepError  = regexp.MustCompile(`^#([0-9]+) ERROR: (.*)$`)
	// buildkitSummaryHeader is the failed step's header in BuildKit's closing
	// `------` block, the one a capped capture keeps when the progress header
	// scrolled off: ` > [bex-hugo-amd64 1/1] ADD …:`.
	buildkitSummaryHeader = regexp.MustCompile(`^ > \[([^\]]+)\] (.*?):?$`)
	platformStage         = regexp.MustCompile(`^bex-([a-z0-9]+)`)
	fetchImageStep        = regexp.MustCompile(`^(?:load metadata for |FROM |COPY (?:--\S+ )*--from=)(\S+/\S+)`)
	fetchURL              = regexp.MustCompile(`https?://([^/\s]+)/(?:\S*/)?([^/\s]+)`)
	// The HTTP status of a failed fetch, as BuildKit's HTTP source
	// (`invalid response status 504`) or a registry client (`… 503 Service
	// Unavailable`) reports it.
	fetchStatus = regexp.MustCompile(`invalid response status ([0-9]{3})|: ([45][0-9]{2}) [A-Z][A-Za-z]+(?: [A-Z][A-Za-z]+)*`)
)

// platformToolNames names what a fetched stage, file or image supplies, where
// the default "<name> toolchain" would not read right.
var platformToolNames = map[string]string{
	"hugo":          "Hugo toolchain",
	"cnb-run":       "native runtime image",
	"static-ffmpeg": "FFmpeg toolchain",
	"busybox":       "build helper image",
}

// PlatformFetch is a native build step that failed fetching something bex
// supplies: what (Tool), from where (Host, when known) and how (Status, the
// HTTP status when one was reported; ChecksumMismatch when the download did
// not match its pinned digest).
type PlatformFetch struct {
	Tool             string
	Host             string
	Status           string
	ChecksumMismatch bool
}

// platformFetchFailure reads a native build's captured BuildKit transcript and
// returns the failed step's fetch when it was one bex makes — an ADD, an image
// pull, or a COPY from an image — or nil for every other failure, including
// the tenant's build command and a compile step inside a helper stage.
func platformFetchFailure(transcript string) *PlatformFetch {
	headers := map[string]string{}
	var failed, header string
	for l := range strings.SplitSeq(transcript, "\n") {
		if m := buildkitStepHeader.FindStringSubmatch(l); m != nil {
			if _, seen := headers[m[1]]; !seen {
				headers[m[1]] = "[" + m[2] + "] " + m[3]
			}
		} else if m := buildkitStepError.FindStringSubmatch(l); m != nil && failed == "" {
			failed = m[1]
		} else if m := buildkitSummaryHeader.FindStringSubmatch(l); m != nil && header == "" {
			header = "[" + m[1] + "] " + m[2]
		}
	}
	if h, known := headers[failed]; known {
		header = h
	}
	stage, step, _ := strings.Cut(strings.TrimPrefix(header, "["), "] ")
	var fetch PlatformFetch
	var tool string
	if m := fetchImageStep.FindStringSubmatch(step); m != nil {
		fetch.Host, tool = imageSource(m[1])
	} else if m := fetchURL.FindStringSubmatch(step); strings.HasPrefix(step, "ADD ") && m != nil {
		fetch.Host, tool = m[1], m[2]
		if i := strings.IndexAny(tool, "-_."); i > 0 {
			tool = tool[:i] // the file's name before its version: pnpm-10.34.7.tgz
		}
	} else {
		return nil
	}
	if m := platformStage.FindStringSubmatch(stage); m != nil {
		tool = m[1] // a helper stage names its tool: bex-hugo-amd64
	}
	fetch.Tool = platformToolNames[tool]
	if fetch.Tool == "" {
		fetch.Tool = tool + " toolchain"
	}
	// The verdict is the failed step's own lines and BuildKit's closing
	// error, so a parallel step's status cannot be read as this one's.
	var verdict strings.Builder
	for l := range strings.SplitSeq(transcript, "\n") {
		if failed == "" || strings.HasPrefix(l, "#"+failed+" ") || strings.HasPrefix(l, "error:") {
			verdict.WriteString(l + "\n")
		}
	}
	if m := fetchStatus.FindStringSubmatch(verdict.String()); m != nil {
		fetch.Status = m[1] + m[2]
	}
	fetch.ChecksumMismatch = strings.Contains(verdict.String(), "digest mismatch")
	return &fetch
}

// imageSource is the registry host an image reference pulls from and the name
// its repository goes by, without a `bex-` prefix: ghcr.io/bex-co/bex-hugo:…
// is ("ghcr.io", "hugo"), docker.io/library/busybox:… is ("docker.io", "busybox").
func imageSource(ref string) (host, name string) {
	ref, _, _ = strings.Cut(ref, "@")
	if slash := strings.LastIndexByte(ref, '/'); strings.LastIndexByte(ref, ':') > slash {
		ref = ref[:strings.LastIndexByte(ref, ':')]
	}
	host = "docker.io"
	if first, rest, found := strings.Cut(ref, "/"); found && strings.ContainsAny(first, ".:") {
		host, ref = first, rest
	}
	return host, strings.TrimPrefix(ref[strings.LastIndexByte(ref, '/')+1:], "bex-")
}
