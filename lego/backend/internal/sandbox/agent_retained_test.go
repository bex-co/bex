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

package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// ADR047 D12: the sandbox seams of a stopped turn's retained disk. They reuse
// the commit fixture of a repository environment build; the principal is a
// session here.

type retainFixture struct {
	*commitFixture
	session string
	image   string
}

func newRetainFixture(t *testing.T) *retainFixture {
	t.Helper()
	f := newCommitFixture(t)
	f.build = ids.New(ids.AgentSession) // the principal the fixture's sandbox is bound to
	return &retainFixture{commitFixture: f, session: f.build, image: ids.New(ids.AgentRetainedImage)}
}

func (f *retainFixture) retain() (RetainedCommit, error) {
	return f.lifecycle.RetainAgentSessionSandbox(context.Background(), "tea-a", f.session, "builder-1", f.image)
}

func (f *retainFixture) ref(path string) name.Reference {
	host := strings.TrimPrefix(f.sink.URL, "http://")
	ref, _ := name.ParseReference(host + "/snapshots/tea-a-sandbox/" + path)
	return ref
}

func (f *retainFixture) snapshotTagged(tag string) bool {
	_, err := remote.Head(f.ref("builder-1-sandbox:" + tag))
	return err == nil
}

// The driver's status, interrupt and result files go before the pause, so the
// sandbox that boots the image cannot read the stopped turn's verdict as its
// own; the newest committed snapshot is copied to the retained image's
// repository; and the paused sandbox and its pause images are ended while the
// copy stays. The session's egress policy is not touched: a newer turn may be
// using it.
func TestRetainAgentSessionSandboxSavesTheDisk(t *testing.T) {
	f := newRetainFixture(t)
	commit, err := f.retain()
	if err != nil {
		t.Fatal(err)
	}
	if commit.Digest != f.snapshot || commit.SizeBytes <= 0 || !commit.SandboxEnded {
		t.Fatalf("commit = %+v, want the newest snapshot %s and an ended sandbox", commit, f.snapshot)
	}
	if len(f.calls) != 3 || f.calls[1] != "pause" || f.calls[2] != "terminate" {
		t.Fatalf("calls = %v, want the files removed, then the pause, then the end", f.calls)
	}
	for _, path := range []string{agentSessionStatusPath, agentSessionInterruptPath, agentSessionResultPath} {
		if !strings.Contains(f.calls[0], "rm -f ") || !strings.Contains(f.calls[0], path) || !strings.Contains(f.calls[0], "test ! -e "+path) {
			t.Fatalf("exec %q does not remove and check %s", f.calls[0], path)
		}
	}
	desc, err := remote.Head(f.ref("images/" + f.image + ":" + imageTag))
	if err != nil || desc.Digest.String() != f.snapshot {
		t.Fatalf("the retained image = %+v / %v", desc, err)
	}
	for _, tag := range []string{"snap-gen1", "snap-gen2"} {
		if f.snapshotTagged(tag) {
			t.Fatalf("the ended sandbox's %s is still tagged", tag)
		}
	}
	egress := f.svc.SessionEgress.(*fakeSessionEgress)
	if len(egress.calls) != 0 {
		t.Fatalf("egress = %+v, want the session's policy left to the caller", egress.calls)
	}
	// The caller deletes it once no newer turn uses it.
	if err := f.lifecycle.DeleteAgentSessionEgress(context.Background(), "tea-a", f.session); err != nil {
		t.Fatal(err)
	}
	if len(egress.calls) != 1 || egress.calls[0].op != "delete" || egress.calls[0].session != f.session || egress.calls[0].namespace != "tea-a-sandbox" {
		t.Fatalf("egress = %+v, want the session's policy deleted", egress.calls)
	}
}

// A failure before the pause leaves the sandbox running and untouched: the
// status files cannot be removed, the workspace is at its storage cap.
func TestRetainAgentSessionSandboxRefusalsBeforeThePauseLeaveItRunning(t *testing.T) {
	t.Run("a file that cannot be removed", func(t *testing.T) {
		f := newRetainFixture(t)
		f.rmExit = 1
		_, err := f.retain()
		if err == nil || errors.Is(err, ErrAgentImageStorageLimit) || errors.Is(err, ErrAgentSandboxPaused) {
			t.Fatalf("retain = %v", err)
		}
		if len(f.calls) != 1 {
			t.Fatalf("calls = %v, want no pause and no end", f.calls)
		}
	})
	t.Run("at the cap", func(t *testing.T) {
		f := newRetainFixture(t)
		f.svc.Storage, f.svc.MaxStorageBytes = &fakeStorageUsage{bytes: 100}, 100
		if _, err := f.retain(); !errors.Is(err, ErrAgentImageStorageLimit) || errors.Is(err, ErrAgentSandboxPaused) {
			t.Fatalf("retain at the cap = %v", err)
		}
		if len(f.calls) != 1 {
			t.Fatalf("calls = %v, want no pause and no end", f.calls)
		}
	})
	t.Run("without the registry", func(t *testing.T) {
		f := newRetainFixture(t)
		f.svc.Images = nil
		if _, err := f.retain(); !errors.Is(err, core.ErrUnavailable) {
			t.Fatalf("retain without the registry = %v", err)
		}
		if len(f.calls) != 0 {
			t.Fatalf("calls = %v", f.calls)
		}
	})
	t.Run("an id that is not a retained image's", func(t *testing.T) {
		f := newRetainFixture(t)
		f.image = ids.New(ids.AgentRepoEnvironmentBuild)
		if _, err := f.retain(); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("retain under a build id = %v", err)
		}
		if len(f.calls) != 0 {
			t.Fatalf("calls = %v", f.calls)
		}
	})
}

// A failure after the pause ends the sandbox, which cannot run its turn
// again, writes no image, and says so.
func TestRetainAgentSessionSandboxRefusalsAfterThePauseEndTheSandbox(t *testing.T) {
	t.Run("past the room left under the cap", func(t *testing.T) {
		f := newRetainFixture(t)
		f.svc.Storage, f.svc.MaxStorageBytes = &fakeStorageUsage{bytes: 100}, 110
		_, err := f.retain()
		if !errors.Is(err, ErrAgentImageStorageLimit) || !errors.Is(err, ErrAgentSandboxPaused) {
			t.Fatalf("retain past the room = %v", err)
		}
		if got := f.calls[len(f.calls)-1]; got != "terminate" {
			t.Fatalf("calls = %v, want the sandbox ended", f.calls)
		}
		if _, err := remote.Head(f.ref("images/" + f.image + ":" + imageTag)); err == nil {
			t.Fatalf("an image was copied that the cap refused")
		}
	})
	t.Run("past the image cap", func(t *testing.T) {
		f := newRetainFixture(t)
		f.svc.Images.MaxBytes = 10
		if _, err := f.retain(); !errors.Is(err, ErrAgentImageStorageLimit) || !errors.Is(err, ErrAgentSandboxPaused) {
			t.Fatalf("retain past the image cap = %v", err)
		}
	})
	t.Run("reserved within the room", func(t *testing.T) {
		f := newRetainFixture(t)
		usage := &fakeStorageUsage{bytes: 100}
		f.svc.Storage, f.svc.MaxStorageBytes = usage, 100<<20
		commit, err := f.retain()
		if err != nil || usage.bytes != 100+commit.SizeBytes {
			t.Fatalf("retain = %+v / %v, usage %d: the image's bytes were not reserved", commit, err, usage.bytes)
		}
	})
}

// A retained image is deleted by its id, and only a retained image's id; the
// workspace purge removes the ones the store still names.
func TestRetainedImagesAreDeleted(t *testing.T) {
	f := newRetainFixture(t)
	if _, err := f.retain(); err != nil {
		t.Fatal(err)
	}
	ref := f.ref("images/" + f.image + ":" + imageTag)
	if err := f.lifecycle.DeleteAgentRetainedImage(context.Background(), "tea-a", f.image); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.Head(ref); err == nil {
		t.Fatal("the deleted retained image is still tagged")
	}
	if err := f.lifecycle.DeleteAgentRetainedImage(context.Background(), "tea-a", f.image); err != nil {
		t.Fatalf("deleting a gone image = %v", err)
	}
	for _, bad := range []string{ids.New(ids.AgentRepoEnvironmentBuild), "sbb-0123456789abcdefghij", ids.New(ids.AgentSession)} {
		if err := f.lifecycle.DeleteAgentRetainedImage(context.Background(), "tea-a", bad); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("deleting %s through the retained seam = %v", bad, err)
		}
	}

	left := ids.New(ids.AgentRetainedImage)
	leftRef := f.ref("images/" + left + ":" + imageTag)
	img, _ := random.Image(64, 1)
	if err := remote.Write(leftRef, img); err != nil {
		t.Fatal(err)
	}
	purger := &WorkspacePurger{Images: &Images{Store: agentBuildIDs{}, Registry: f.svc.Images.Registry, Reaper: f.svc.Images.Reaper}, AgentRetained: retainedImageList{left}}
	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.Head(leftRef); err == nil {
		t.Fatal("the workspace purge left a retained image behind")
	}
}

type retainedImageList []string

func (l retainedImageList) AgentRetainedImageIDs(context.Context, string) ([]string, error) {
	return l, nil
}

// A session sandbox boots from a retained image in the workspace's images
// repository, pinned to its digest; an id of another kind than a build's or a
// retained image's is refused.
func TestAgentSessionSandboxBootsFromARetainedImage(t *testing.T) {
	client, create := createCapture(t)
	svc := &Service{Base: &core.Base{}, Client: client, SessionEgress: &fakeSessionEgress{},
		Templates: map[string]Template{"agent": {Image: "bex/agent:1", Entrypoint: []string{"driver"}, SessionOnly: true}},
		Images:    &Images{Registry: RegistryConfig{BaseURL: "http://zot.internal:5000", Prefix: DefaultSnapshotRepoPrefix}}}
	lifecycle := NewAgentSessionLifecycle(svc)
	retained := ids.New(ids.AgentRetainedImage)
	spec := func(image AgentImage) AgentSessionSandbox {
		return AgentSessionSandbox{WorkspaceID: "tea-a", SessionID: "ags-session", Repository: "bex-co/example", Branch: "bex-agent/session-test",
			ModelEndpoint: "https://api.openai.com/v1", ModelAPIKey: agentsession.ModelKeyPlaceholder("ags-session"), Image: image}
	}
	if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), spec(AgentImage{ID: retained, Digest: testAgentDigest})); err != nil {
		t.Fatal(err)
	}
	want := "zot.internal:5000/snapshots/tea-a-sandbox/images/" + retained + "@" + testAgentDigest
	if create.Image.URI != want || len(create.Entrypoint) != 1 || create.Entrypoint[0] != "driver" {
		t.Fatalf("create image = %q entrypoint %v, want %q", create.Image.URI, create.Entrypoint, want)
	}
	if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), spec(AgentImage{ID: ids.New(ids.AgentSession), Digest: testAgentDigest})); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a session id as an image = %v", err)
	}
}
