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

package github

import (
	"bytes"
	"strings"
	"testing"
)

// publicGitFuzzSeeds are the advertisement bodies the package's protocol tests
// exercise — protocolv0, the version-1 preamble, an empty repository, a symref
// fallback — plus the framing edges the hand-rolled pkt-line parser must
// survive: flush-only, a truncated header, a length shorter than the header, a
// length past the buffer, non-hex digits, and a zero-length packet mid-stream.
// The bytes are fetched from a third-party host a tenant names, so the host
// controls every one of them. Seeds live as source so a reviewer sees exactly
// what the untrusted surface is probed with; the on-disk testdata/fuzz cache is
// never committed.
func publicGitFuzzSeeds(f *testing.F) {
	f.Helper()
	for _, seed := range []string{
		"",
		"0000",
		"000",
		"00000",
		"0001",
		"0003",
		"0004",
		"fff0",
		"zzzz",
		gitAdvertisement(testCommitA+" refs/heads/main\x00symref=HEAD:refs/heads/main\n", testCommitA+" HEAD\n"),
		gitAdvertisement(strings.Repeat("0", 40) + " capabilities^{}\x00\n"),
		gitPacket("# service=git-upload-pack") + "0000" + gitPacket("version 1\n") + gitPacket(testCommitA+" refs/heads/main\x00symref=HEAD:refs/heads/main") + "0000",
		gitPacket("# service=git-upload-pack\n") + "0000" + "0004" + "0000",
	} {
		f.Add(seed)
	}
}

// FuzzPublicGitPacket drives Git's four-hex-digit, length-includes-header
// pkt-line framing over bytes an untrusted host returns for a tenant-named
// public repository. The parser indexes and re-slices the buffer by a length
// it reads from the buffer itself, so it must never panic and must never lose
// or duplicate a byte: a parsed packet plus its remainder always accounts for
// the whole input.
func FuzzPublicGitPacket(f *testing.F) {
	publicGitFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, data string) {
		raw := []byte(data)
		payload, rest, flush, ok := publicGitPacket(raw)
		if !ok {
			return
		}
		if flush && data[:4] != "0000" {
			t.Fatalf("publicGitPacket(%q) reported flush for %q", data, data[:4])
		}
		if len(payload)+len(rest)+4 != len(raw) {
			t.Fatalf("publicGitPacket(%q) = payload %d + rest %d + header, want %d", data, len(payload), len(rest), len(raw))
		}
		if !bytes.Equal(append(append([]byte(data[:4]), payload...), rest...), raw) {
			t.Fatalf("publicGitPacket(%q) did not account for every byte", data)
		}
	})
}

// FuzzParsePublicGitAdvertisement drives the full advertisement parser with
// arbitrary host-supplied bytes. It walks packets in a bounded loop and must
// reject any shape it does not understand with ok=false rather than panicking;
// any accepted advertisement must name only well-formed 40/64-hex commit ids.
func FuzzParsePublicGitAdvertisement(f *testing.F) {
	publicGitFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, data string) {
		refs, head, ok := parsePublicGitAdvertisement([]byte(data))
		if !ok {
			return
		}
		for name, hash := range refs {
			if len(hash) != 40 && len(hash) != 64 {
				t.Fatalf("accepted %q -> %q, a commit id is 40 or 64 hex digits", name, hash)
			}
			if name == "" {
				t.Fatalf("accepted an empty ref name for %q", hash)
			}
		}
		if head != "" && !validPublicGitRef(head) {
			t.Fatalf("accepted HEAD symref %q", head)
		}
	})
}
