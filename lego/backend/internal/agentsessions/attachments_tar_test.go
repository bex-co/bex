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

package agentsessions

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The dispatch archive must be one the gateway's real extractor accepts: a
// fake copier never builds it, which is how a missing directory entry reached
// production (every session with an attachment failed ATTACHMENT_COPY_FAILED).
func TestAttachmentTarIsAcceptedByTheGatewayExtractor(t *testing.T) {
	objects := &fakeObjects{data: map[string][]byte{
		"tea-a/att-one":   []byte("first log\n"),
		"tea-a/att-two":   []byte("second turn\n"),
		"tea-a/att-three": []byte("first turn again\n"),
	}}
	s := &Service{Attachments: objects}
	now := time.Now()
	// Turns interleave and repeat: each turn directory is written once, before
	// its first file, wherever its files fall in the list.
	files := []store.AgentAttachment{
		{ID: "att-one", Turn: 1, Name: "build.log", ObjectKey: "tea-a/att-one", SizeBytes: 10, CreatedAt: now},
		{ID: "att-two", Turn: 2, Name: "next.txt", ObjectKey: "tea-a/att-two", SizeBytes: 12, CreatedAt: now},
		{ID: "att-three", Turn: 1, Name: "again.txt", ObjectKey: "tea-a/att-three", SizeBytes: 17, CreatedAt: now},
	}
	var buf bytes.Buffer
	if err := s.writeAttachmentTar(context.Background(), &buf, files); err != nil {
		t.Fatal(err)
	}
	if err := sandboxfiles.ValidateArchive(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("gateway rejects the dispatch archive: %v", err)
	}

	got := map[string]string{}
	dirs := 0
	r := tar.NewReader(&buf)
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeDir {
			dirs++
			continue
		}
		body, _ := io.ReadAll(r)
		got[h.Name] = string(body)
	}
	want := map[string]string{
		"1/att-one-build.log":   "first log\n",
		"2/att-two-next.txt":    "second turn\n",
		"1/att-three-again.txt": "first turn again\n",
	}
	if dirs != 2 || len(got) != len(want) {
		t.Fatalf("archive has %d dirs and files %v, want 2 dirs and %v", dirs, got, want)
	}
	for name, body := range want {
		if got[name] != body {
			t.Fatalf("%s = %q, want %q", name, got[name], body)
		}
	}
}

// The skills archive goes through the same gateway extractor (ADR099 D5).
func TestSkillTarIsAcceptedByTheGatewayExtractor(t *testing.T) {
	skills := []deliveredSkill{
		{manifest: skillManifestSkill{Name: "deploy"}, document: "---\nname: deploy\n---\nbody\n"},
		{manifest: skillManifestSkill{Name: "release"}, document: "---\nname: release\n---\nother\n"},
	}
	var buf bytes.Buffer
	if err := writeSkillTar(&buf, skills, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := sandboxfiles.ValidateArchive(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("gateway rejects the skills archive: %v", err)
	}
}
