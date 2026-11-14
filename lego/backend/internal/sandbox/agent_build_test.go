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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
)

// ADR111: the sandbox seams of a repository environment build.

const testAgentDigest = "sha256:00000000000000000000000000000000000000000000000000000000000000aa"

// createCapture is an OpenSandbox that records the one create it is sent.
func createCapture(t *testing.T) (*Client, *createRequest) {
	t.Helper()
	var create createRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sandboxes" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
			t.Error(err)
		}
		_, _ = fmt.Fprint(w, `{"id":"sandbox-1","status":{"state":"Running"}}`)
	}))
	t.Cleanup(upstream.Close)
	return NewClient(upstream.URL), &create
}

// A session sandbox boots from a build's image in the workspace's own images
// repository, pinned to its digest, and keeps the template's entrypoint; a
// malformed image or a deployment without the registry is refused.
func TestAgentSessionSandboxBootsFromABuildImage(t *testing.T) {
	client, create := createCapture(t)
	svc := &Service{Base: &core.Base{}, Client: client, SessionEgress: &fakeSessionEgress{},
		Templates: map[string]Template{"agent": {Image: "bex/agent:1", Entrypoint: []string{"driver"}, SessionOnly: true}},
		Images:    &Images{Registry: RegistryConfig{BaseURL: "http://zot.internal:5000", Prefix: DefaultSnapshotRepoPrefix}}}
	lifecycle := NewAgentSessionLifecycle(svc)
	build := ids.New(ids.AgentRepoEnvironmentBuild)
	placeholder := agentsession.ModelKeyPlaceholder("ags-session")
	spec := func(image AgentImage) AgentSessionSandbox {
		return AgentSessionSandbox{WorkspaceID: "tea-a", SessionID: "ags-session", Repository: "bex-co/example", Branch: "bex-agent/session-test",
			ModelEndpoint: "https://api.openai.com/v1", ModelAPIKey: placeholder, Image: image}
	}
	if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), spec(AgentImage{BuildID: build, Digest: testAgentDigest})); err != nil {
		t.Fatal(err)
	}
	want := "zot.internal:5000/snapshots/tea-a-sandbox/images/" + build + "@" + testAgentDigest
	if create.Image.URI != want || len(create.Entrypoint) != 1 || create.Entrypoint[0] != "driver" {
		t.Fatalf("create image = %q entrypoint %v, want %q", create.Image.URI, create.Entrypoint, want)
	}
	if got := lifecycle.AgentTemplateImage(""); got != "bex/agent:1" {
		t.Fatalf("template image = %q", got)
	}
	for _, image := range []AgentImage{{BuildID: "sbb-0123456789abcdefghij", Digest: testAgentDigest}, {BuildID: build, Digest: "latest"}} {
		if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), spec(image)); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("image %+v = %v", image, err)
		}
	}
	svc.Images = nil
	if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), spec(AgentImage{BuildID: build, Digest: testAgentDigest})); !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("a build image without the registry = %v", err)
	}
}

// A builder is an agent template sandbox whose binding labels name its build,
// its repository and the build branch, under the build posture, with no
// model credential.
func TestAgentBuildSandboxIsBoundToItsBuild(t *testing.T) {
	client, create := createCapture(t)
	egress := &fakeSessionEgress{}
	svc := &Service{Base: &core.Base{}, Client: client, SessionEgress: egress,
		Templates: map[string]Template{"agent": {Image: "bex/agent:1", Entrypoint: []string{"driver"}, SessionOnly: true}}}
	lifecycle := NewAgentSessionLifecycle(svc)
	build := ids.New(ids.AgentRepoEnvironmentBuild)
	if _, err := lifecycle.CreateAgentBuildSandbox(context.Background(), "tea-a", build, "bex-co/example",
		sessionegress.Egress{Hosts: []string{"docs.example.com"}}, map[string]string{"BEX_AGENT_MODE": "build"}); err != nil {
		t.Fatal(err)
	}
	bindings, err := agentsession.BindingLabels(build, "bex-co/example", agentsession.BuildBranch(build))
	if err != nil {
		t.Fatal(err)
	}
	for label, value := range bindings {
		if create.Metadata[label] != value {
			t.Fatalf("metadata %s = %q, want %q", label, create.Metadata[label], value)
		}
	}
	if create.Image.URI != "bex/agent:1" || create.Env["BEX_AGENT_MODE"] != "build" || create.Env[ModelAPIKeyEnvVar] != "" {
		t.Fatalf("builder create = image %q env %v", create.Image.URI, create.Env)
	}
	if len(egress.calls) != 1 || egress.calls[0].op != "build" || egress.calls[0].session != build || egress.calls[0].extra[0] != "docs.example.com" {
		t.Fatalf("builder egress = %+v", egress.calls)
	}
	if _, err := lifecycle.CreateAgentBuildSandbox(context.Background(), "tea-a", "ags-0123456789abcdefghij", "bex-co/example", sessionegress.Egress{}, nil); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a builder for a session id = %v", err)
	}
}

// commitFixture is a paused builder's world: OpenSandbox moving the builder
// from Running through Pausing to Paused once paused, the exec gateway that
// removes the build's status file, and the sandbox registry holding its
// committed snapshot.
type commitFixture struct {
	svc       *Service
	lifecycle *AgentSessionLifecycle
	build     string
	sink      *httptest.Server
	snapshot  string // the committed image's digest
	mu        sync.Mutex
	calls     []string
	rmExit    int
}

func newCommitFixture(t *testing.T) *commitFixture {
	t.Helper()
	f := &commitFixture{build: ids.New(ids.AgentRepoEnvironmentBuild)}
	prevPoll, prevStart := agentBuildCommitPoll, agentBuildPauseStart
	agentBuildCommitPoll, agentBuildPauseStart = time.Millisecond, time.Minute
	t.Cleanup(func() { agentBuildCommitPoll, agentBuildPauseStart = prevPoll, prevStart })
	f.sink = httptest.NewServer(registry.New())
	t.Cleanup(f.sink.Close)
	host := strings.TrimPrefix(f.sink.URL, "http://")
	img, err := random.Image(512, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"snap-gen1", "snap-gen2"} {
		ref, err := name.ParseReference(host + "/snapshots/tea-a-sandbox/builder-1-sandbox:" + tag)
		if err != nil {
			t.Fatal(err)
		}
		if tag == "snap-gen1" {
			older, _ := random.Image(64, 1)
			if err := remote.Write(ref, older); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := remote.Write(ref, img); err != nil {
			t.Fatal(err)
		}
	}
	digest, _ := img.Digest()
	f.snapshot = digest.String()

	state, reads := "Running", 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/builder-1":
			if state == "Pausing" {
				if reads++; reads > 1 {
					state = "Paused"
				}
			}
			sb := osSandbox{ID: "builder-1", Metadata: map[string]string{
				metadataOwner: "system", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
				metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: f.build,
			}}
			sb.Status.State = state
			_ = json.NewEncoder(w).Encode(sb)
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes/builder-1/pause":
			f.calls = append(f.calls, "pause")
			state = "Pausing"
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodDelete && r.URL.Path == "/sandboxes/builder-1":
			f.calls = append(f.calls, "terminate")
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	secret := []byte("exec-secret")
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
		if err != nil || claims.AgentSessionID != f.build || claims.SandboxID != "builder-1" {
			http.Error(w, "invalid ticket", http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.calls = append(f.calls, "exec "+strings.Join(claims.Command, " "))
		exit := f.rmExit
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "event: exit\ndata: {\"exit_code\":%d}\n\n", exit)
	}))
	t.Cleanup(gateway.Close)
	config := RegistryConfig{BaseURL: f.sink.URL, Prefix: DefaultSnapshotRepoPrefix}
	f.svc = &Service{
		Base:          &core.Base{},
		Client:        NewClient(upstream.URL),
		Exec:          &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client()},
		Images:        &Images{Registry: config, Reaper: &RegistryReaper{RegistryConfig: config}},
		SessionEgress: &fakeSessionEgress{},
	}
	f.lifecycle = NewAgentSessionLifecycle(f.svc)
	return f
}

// The builder's status file goes before its pause; the newest committed
// snapshot is copied to the build's repository under its one tag; the commit
// reports the image's digest and compressed size.
func TestCommitAgentBuildSandboxCopiesTheSnapshot(t *testing.T) {
	f := newCommitFixture(t)
	commit, err := f.lifecycle.CommitAgentBuildSandbox(context.Background(), "tea-a", f.build, "builder-1")
	if err != nil {
		t.Fatal(err)
	}
	if commit.Digest != f.snapshot || commit.SizeBytes <= 0 {
		t.Fatalf("commit = %+v, want the newest snapshot %s", commit, f.snapshot)
	}
	if len(f.calls) != 2 || !strings.Contains(f.calls[0], "rm -f "+agentSessionStatusPath) || f.calls[1] != "pause" {
		t.Fatalf("calls = %v, want the status file removed before the pause", f.calls)
	}
	host := strings.TrimPrefix(f.sink.URL, "http://")
	ref, _ := name.ParseReference(host + "/snapshots/tea-a-sandbox/images/" + f.build + ":" + imageTag)
	desc, err := remote.Head(ref)
	if err != nil || desc.Digest.String() != f.snapshot {
		t.Fatalf("the build's image = %+v / %v", desc, err)
	}
	// Terminating the builder drops its pause images, which no storage object
	// would ever reap, and keeps the build's copy.
	if err := f.lifecycle.TerminateAgentBuildSandbox(context.Background(), "tea-a", f.build, "builder-1"); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"snap-gen1", "snap-gen2"} {
		snap, _ := name.ParseReference(host + "/snapshots/tea-a-sandbox/builder-1-sandbox:" + tag)
		if _, err := remote.Head(snap); err == nil {
			t.Fatalf("the terminated builder's %s is still tagged", tag)
		}
	}
	if _, err := remote.Head(ref); err != nil {
		t.Fatalf("terminating the builder dropped the build's image: %v", err)
	}
}

// A build that cannot drop its status file, at the workspace's storage cap,
// with an image past the room left or past the image cap, is refused; the
// first two before any pause.
func TestCommitAgentBuildSandboxRefusals(t *testing.T) {
	t.Run("status file", func(t *testing.T) {
		f := newCommitFixture(t)
		f.rmExit = 1
		if _, err := f.lifecycle.CommitAgentBuildSandbox(context.Background(), "tea-a", f.build, "builder-1"); err == nil || errors.Is(err, ErrAgentImageStorageLimit) {
			t.Fatalf("commit = %v", err)
		}
		if len(f.calls) != 1 {
			t.Fatalf("calls = %v, want no pause", f.calls)
		}
	})
	t.Run("at the cap", func(t *testing.T) {
		f := newCommitFixture(t)
		f.svc.Storage, f.svc.MaxStorageBytes = &fakeStorageUsage{bytes: 100}, 100
		if _, err := f.lifecycle.CommitAgentBuildSandbox(context.Background(), "tea-a", f.build, "builder-1"); !errors.Is(err, ErrAgentImageStorageLimit) {
			t.Fatalf("commit at the cap = %v", err)
		}
		if len(f.calls) != 1 {
			t.Fatalf("calls = %v, want no pause", f.calls)
		}
	})
	t.Run("past the room", func(t *testing.T) {
		f := newCommitFixture(t)
		f.svc.Storage, f.svc.MaxStorageBytes = &fakeStorageUsage{bytes: 100}, 110
		if _, err := f.lifecycle.CommitAgentBuildSandbox(context.Background(), "tea-a", f.build, "builder-1"); !errors.Is(err, ErrAgentImageStorageLimit) {
			t.Fatalf("commit past the room = %v", err)
		}
	})
	t.Run("reserved within the room", func(t *testing.T) {
		f := newCommitFixture(t)
		usage := &fakeStorageUsage{bytes: 100}
		f.svc.Storage, f.svc.MaxStorageBytes = usage, 100<<20
		commit, err := f.lifecycle.CommitAgentBuildSandbox(context.Background(), "tea-a", f.build, "builder-1")
		if err != nil || usage.bytes != 100+commit.SizeBytes {
			t.Fatalf("commit = %+v / %v, usage %d: the image's bytes were not reserved", commit, err, usage.bytes)
		}
	})
	t.Run("past the image cap", func(t *testing.T) {
		f := newCommitFixture(t)
		f.svc.Images.MaxBytes = 10
		if _, err := f.lifecycle.CommitAgentBuildSandbox(context.Background(), "tea-a", f.build, "builder-1"); !errors.Is(err, ErrAgentImageStorageLimit) {
			t.Fatalf("commit past the image cap = %v", err)
		}
	})
}

// agentBuildIDs is an image store whose only use here is the workspace
// purge's list of imported builds.
type agentBuildIDs struct{ ImageStore }

func (agentBuildIDs) SandboxBuildIDs(context.Context, string, string) ([]string, error) {
	return nil, nil
}

type agentBuildImageList []string

func (l agentBuildImageList) AgentRepoEnvironmentBuildImageIDs(context.Context, string) ([]string, error) {
	return l, nil
}

// A build's image is deleted on retention or an environment delete, and with
// its workspace; an image already gone is no error.
func TestAgentBuildImagesAreDeleted(t *testing.T) {
	sink := httptest.NewServer(registry.New())
	t.Cleanup(sink.Close)
	host := strings.TrimPrefix(sink.URL, "http://")
	config := RegistryConfig{BaseURL: sink.URL, Prefix: DefaultSnapshotRepoPrefix}
	images := &Images{Store: agentBuildIDs{}, Registry: config, Reaper: &RegistryReaper{RegistryConfig: config}}
	push := func(build string) name.Reference {
		ref, _ := name.ParseReference(host + "/snapshots/tea-a-sandbox/images/" + build + ":" + imageTag)
		img, _ := random.Image(64, 1)
		if err := remote.Write(ref, img); err != nil {
			t.Fatal(err)
		}
		return ref
	}
	lifecycle := NewAgentSessionLifecycle(&Service{Base: &core.Base{}, Images: images})
	retired := ids.New(ids.AgentRepoEnvironmentBuild)
	ref := push(retired)
	if err := lifecycle.DeleteAgentBuildImage(context.Background(), "tea-a", retired); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.Head(ref); err == nil {
		t.Fatal("the retired image is still tagged")
	}
	if err := lifecycle.DeleteAgentBuildImage(context.Background(), "tea-a", retired); err != nil {
		t.Fatalf("deleting a gone image = %v", err)
	}
	if err := lifecycle.DeleteAgentBuildImage(context.Background(), "tea-a", "sbb-0123456789abcdefghij"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("deleting an import through the build seam = %v", err)
	}
	kept := ids.New(ids.AgentRepoEnvironmentBuild)
	ref = push(kept)
	purger := &WorkspacePurger{Images: images, AgentBuilds: agentBuildImageList{kept}}
	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.Head(ref); err == nil {
		t.Fatal("the workspace delete left a build image")
	}
}
