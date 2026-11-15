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
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Repository environment builds (ADR111): a builder is an agent template
// sandbox whose binding labels name its build, the gateway's build principal.
// It runs the setup steps; then its paused root filesystem is copied into the
// workspace's images repository, from which the repository's sessions boot.

// AgentImage names an image an agent sandbox boots from, in the workspace's
// own images repository: a repository environment build's (ADR111 D4) or a
// stopped turn's retained disk (ADR047 D12), by its id and the image's
// digest. The zero value boots the template image.
type AgentImage struct {
	ID     string
	Digest string
}

// AgentImageCommit is a committed image: its digest and compressed size.
type AgentImageCommit struct {
	Digest    string
	SizeBytes int64
}

// ErrAgentImageStorageLimit is a build image refused for its size: past the
// imported-image cap or the workspace's sandbox storage cap (ADR111 D9).
var ErrAgentImageStorageLimit = errors.New("the build image would pass the sandbox storage limit")

var imageDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Bounds on the builder's pause, the commit Job it runs (ADR042 D5). Vars so
// tests poll quickly.
var (
	agentBuildCommitTimeout = 30 * time.Minute
	agentBuildCommitPoll    = 5 * time.Second
	// agentBuildPauseStart is how long a builder may still read running after
	// its pause was accepted before the pause counts as failed.
	agentBuildPauseStart = 2 * time.Minute
)

func errAgentImagesUnavailable() error {
	return core.NewUnavailableError(CodeSandboxImagesUnavailable, "repository environment images need the sandbox registry", nil)
}

// agentImageRef is image's pull reference in the workspace's images
// repository, pinned to its digest.
func (s *Service) agentImageRef(workspaceID string, image AgentImage) (string, error) {
	if s.Images == nil {
		return "", errAgentImagesUnavailable()
	}
	if !agentsession.IsAgentImage(image.ID) || !imageDigestPattern.MatchString(image.Digest) {
		return "", fmt.Errorf("%w: invalid agent image %q@%q", core.ErrBadRequest, image.ID, image.Digest)
	}
	repo, err := s.Images.imageRepository(workspaceID, image.ID)
	if err != nil {
		return "", err
	}
	return repo.Digest(image.Digest).String(), nil
}

// AgentTemplateImage is the image an agent template boots ("" for an unknown
// template): the base a build ran on, which a session's template must still
// have for the build's image to carry its driver.
func (l *AgentSessionLifecycle) AgentTemplateImage(template string) string {
	if template == "" {
		template = "agent"
	}
	return l.service.Templates[template].Image
}

// CreateAgentBuildSandbox starts a repository environment build's builder
// from the agent template (ADR111 D1), bound to the build and its repository
// on the build branch, under the build posture: the workspace tier's
// destinations and no model. The caller (agentsessions) owns authorization,
// as for CreateAgentSessionSandbox.
func (l *AgentSessionLifecycle) CreateAgentBuildSandbox(ctx context.Context, workspaceID, buildID, repository string, egress sessionegress.Egress, driverEnv map[string]string) (Sandbox, error) {
	s := l.service
	if err := s.requireAgentSandboxes(workspaceID, buildID); err != nil {
		return Sandbox{}, err
	}
	if !agentsession.IsBuildPrincipal(buildID) {
		return Sandbox{}, fmt.Errorf("%w: invalid build id %q", core.ErrBadRequest, buildID)
	}
	return s.createAgentSandbox(ctx, agentSandbox{
		workspaceID: workspaceID, principal: buildID, repository: repository, branch: agentsession.BuildBranch(buildID), env: driverEnv,
		prepareEgress: func(namespace string) error {
			return s.SessionEgress.PrepareBuild(ctx, namespace, buildID, egress)
		},
	})
}

// ReadAgentBuildStatus reads a builder's status file, absent until its build
// ends, over the same exec boundary as a session's.
func (l *AgentSessionLifecycle) ReadAgentBuildStatus(ctx context.Context, workspaceID, buildID, sandboxID string) (string, error) {
	return l.service.readAgentStatus(ctx, workspaceID, buildID, sandboxID)
}

// CommitAgentBuildSandbox turns a finished builder into the build's image
// (ADR111 D1 step 3): it removes the build's status file, so a session that
// boots the image never reads the build's verdict as its own; pauses the
// builder, which commits its root filesystem (refused at the workspace's
// storage cap, as a suspend is); waits for the commit; reserves the image's
// bytes against the cap; and copies the committed image to the build's
// repository. The caller terminates the builder afterwards whatever the
// outcome (TerminateAgentBuildSandbox).
func (l *AgentSessionLifecycle) CommitAgentBuildSandbox(ctx context.Context, workspaceID, buildID, sandboxID string) (AgentImageCommit, error) {
	s := l.service
	return s.commitAgentSandbox(ctx, agentCommit{
		workspaceID: workspaceID, principal: buildID, sandboxID: sandboxID, imageID: buildID, verb: "build",
		stale: []string{agentSessionStatusPath},
		reserve: func(ctx context.Context, size int64) error {
			return s.reserveAgentImage(ctx, workspaceID, buildID, size)
		},
	})
}

// agentCommit is one pause-commit of an agent sandbox into an image: whose
// sandbox it is, the id the image is stored under, the driver files that are
// removed first so a sandbox that boots the image never reads them as its
// own, and how the image's size is reserved against the workspace's cap.
type agentCommit struct {
	workspaceID, principal, sandboxID, imageID string
	// verb names the commit in a storage refusal: "build" or "retain".
	verb    string
	stale   []string
	reserve func(ctx context.Context, size int64) error
}

// ErrAgentSandboxPaused marks a commit that failed after the sandbox was
// paused: it can no longer run its turn, so a caller that wants to keep it
// has nothing to keep. A commit that fails before the pause leaves the
// sandbox running.
var ErrAgentSandboxPaused = errors.New("the sandbox was paused")

func (s *Service) commitAgentSandbox(ctx context.Context, c agentCommit) (AgentImageCommit, error) {
	if s.Images == nil {
		return AgentImageCommit{}, errAgentImagesUnavailable()
	}
	key, err := s.agentSessionKey(ctx, c.workspaceID)
	if err != nil {
		return AgentImageCommit{}, err
	}
	raw, err := s.agentSessionSandbox(ctx, key, c.workspaceID, c.principal, c.sandboxID)
	if err != nil {
		return AgentImageCommit{}, err
	}
	// rm -f succeeds on a missing file, so the test is what proves each is gone.
	command := "rm -f " + strings.Join(c.stale, " ")
	for _, path := range c.stale {
		command += " && test ! -e " + path
	}
	res, err := s.sessionExec(ctx, c.workspaceID, c.principal, c.sandboxID, command, maxExecOutputBytes)
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("exit %d", res.ExitCode)
	}
	if err != nil {
		return AgentImageCommit{}, fmt.Errorf("remove the driver's status files: %w", err)
	}
	if err := s.checkStorageCap(ctx, raw, c.verb); err != nil {
		return AgentImageCommit{}, fmt.Errorf("%w: %w", ErrAgentImageStorageLimit, err)
	}
	s.Meter.Observe(ctx, raw)
	if err := s.Client.Suspend(ctx, key, c.sandboxID); err != nil {
		return AgentImageCommit{}, fmt.Errorf("pause the sandbox: %w", err)
	}
	raw.Status.State = string(StatusSuspending)
	s.Meter.Observe(ctx, raw)
	paused := func(err error) error { return fmt.Errorf("%w: %w", ErrAgentSandboxPaused, err) }
	if err := s.awaitSuspended(ctx, key, c.sandboxID); err != nil {
		return AgentImageCommit{}, paused(err)
	}
	img, size, err := s.Images.snapshotImage(ctx, c.workspaceID, c.sandboxID)
	if err != nil {
		return AgentImageCommit{}, paused(err)
	}
	if size > s.Images.maxBytes() {
		return AgentImageCommit{}, paused(fmt.Errorf("%w: the image is %s, over the %s an image may hold", ErrAgentImageStorageLimit, formatGB(size), formatGB(s.Images.maxBytes())))
	}
	if err := c.reserve(ctx, size); err != nil {
		return AgentImageCommit{}, paused(err)
	}
	digest, err := s.Images.pushAgentImage(ctx, c.workspaceID, c.imageID, img)
	if err != nil {
		return AgentImageCommit{}, paused(err)
	}
	return AgentImageCommit{Digest: digest, SizeBytes: size}, nil
}

// reserveAgentImage counts a build image's bytes against the workspace's
// storage cap under the lock imports take, so two builds committing at once
// cannot both pass it.
func (s *Service) reserveAgentImage(ctx context.Context, workspaceID, buildID string, size int64) error {
	return s.reserveImageBytes(size, func(limit int64) error {
		return s.Storage.ReserveAgentRepoEnvironmentBuildImage(ctx, workspaceID, buildID, size, limit)
	})
}

// reserveImageBytes runs reserve, the store's reservation of an image's size
// under the workspace's storage lock, against the cap; a refusal past it is
// ErrAgentImageStorageLimit. Without a cap or a store nothing is reserved.
func (s *Service) reserveImageBytes(size int64, reserve func(limit int64) error) error {
	if s.MaxStorageBytes <= 0 || s.Storage == nil {
		return nil
	}
	err := reserve(s.MaxStorageBytes)
	var limit *store.SandboxStorageLimitError
	if errors.As(err, &limit) {
		return fmt.Errorf("%w: the %s image would take the workspace's sandbox images from %s past its %s limit",
			ErrAgentImageStorageLimit, formatGB(size), formatGB(limit.UsedBytes), formatGB(s.MaxStorageBytes))
	}
	return err
}

// awaitSuspended waits until a paused sandbox's commit has finished.
func (s *Service) awaitSuspended(ctx context.Context, key, sandboxID string) error {
	ctx, cancel := context.WithTimeout(ctx, agentBuildCommitTimeout)
	defer cancel()
	paused := s.Now()
	tick := time.NewTicker(agentBuildCommitPoll)
	defer tick.Stop()
	for {
		raw, err := s.Client.Get(ctx, key, sandboxID)
		if err != nil {
			return fmt.Errorf("read the pausing builder: %w", err)
		}
		switch status, _ := openSandboxStatus(raw.Status.State); status {
		case StatusSuspended:
			return nil
		case StatusErrored, StatusTerminated:
			return fmt.Errorf("the builder's pause ended %s", status)
		case StatusRunning:
			if s.Now().Sub(paused) > agentBuildPauseStart {
				return errors.New("the builder kept running: its pause did not commit")
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the builder's commit did not finish within %s", agentBuildCommitTimeout)
		case <-tick.C:
		}
	}
}

// TerminateAgentBuildSandbox ends a builder, its egress policy and its pause
// images, which the storage meter never reaps (an agent sandbox has no
// storage object); the build's own copy stays. OpenSandbox's terminate is
// idempotent.
func (l *AgentSessionLifecycle) TerminateAgentBuildSandbox(ctx context.Context, workspaceID, buildID, sandboxID string) error {
	return l.service.endPausedAgentSandbox(ctx, workspaceID, buildID, sandboxID, true)
}

// endPausedAgentSandbox ends a paused agent sandbox and deletes its pause
// images, which no storage object would ever reap: a builder's, or a stopped
// session turn's once its disk is saved. The egress policy goes with it only
// when withPolicy (endAgentSandbox).
func (s *Service) endPausedAgentSandbox(ctx context.Context, workspaceID, principal, sandboxID string, withPolicy bool) error {
	if err := s.endAgentSandbox(ctx, workspaceID, principal, sandboxID, withPolicy); err != nil {
		return err
	}
	if s.Images != nil && s.Images.Reaper != nil {
		if err := s.Images.Reaper.DeleteImages(ctx, workspaceID, sandboxID); err != nil {
			log.Printf("sandbox: delete paused sandbox %s's snapshot: %v", sandboxID, err)
		}
	}
	return nil
}

// DeleteAgentBuildImage deletes a build's image from the registry (ADR111
// D8). An image already gone is not an error; a deployment without the
// registry or its reaper credential has nothing it can delete.
func (l *AgentSessionLifecycle) DeleteAgentBuildImage(ctx context.Context, workspaceID, buildID string) error {
	if !agentsession.IsBuildPrincipal(buildID) {
		return fmt.Errorf("%w: invalid build id %q", core.ErrBadRequest, buildID)
	}
	if l.service.Images == nil {
		return nil
	}
	return l.service.Images.deleteImported(ctx, workspaceID, []string{buildID})
}

// RetainEndTimeout bounds the end of a paused sandbox after its disk is saved.
const RetainEndTimeout = 30 * time.Second

// RetainedCommit is a stopped turn's disk, saved as an image.
type RetainedCommit struct {
	AgentImageCommit
	// SandboxEnded says the paused sandbox and its pause images are gone. When
	// it is false the sandbox still stands, paused, and the idle reaper ends it.
	SandboxEnded bool
}

// RetainAgentSessionSandbox saves a stopped turn's disk (ADR047 D12): it
// removes the driver's status, interrupt and result files, so the sandbox
// that boots the image never reads the stopped turn's as its own; pauses the
// sandbox, which commits its root filesystem (refused at the workspace's
// storage cap); reserves the image's bytes against that cap; copies the
// committed image to the workspace's images repository under imageID; and
// ends the paused sandbox and its pause images, which no storage object would
// ever reap. The session's egress policy is left: it is shared by every
// sandbox of the session, so the caller deletes it (DeleteAgentSessionEgress)
// once it knows no newer turn is using it. The caller (agentsessions) owns
// authorization and has already read the transcript, which needs a running
// pod.
//
// A failure before the pause leaves the sandbox running and untouched. One
// after it ends the sandbox, since a paused sandbox can no longer be used, and
// is marked ErrAgentSandboxPaused.
func (l *AgentSessionLifecycle) RetainAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID, imageID string) (RetainedCommit, error) {
	s := l.service
	if !agentsession.IsRetainedImage(imageID) {
		return RetainedCommit{}, fmt.Errorf("%w: invalid retained image id %q", core.ErrBadRequest, imageID)
	}
	commit, err := s.commitAgentSandbox(ctx, agentCommit{
		workspaceID: workspaceID, principal: sessionID, sandboxID: sandboxID, imageID: imageID, verb: "retain",
		stale: []string{agentSessionStatusPath, agentSessionInterruptPath, agentSessionResultPath},
		reserve: func(ctx context.Context, size int64) error {
			return s.reserveRetainedImage(ctx, workspaceID, imageID, size)
		},
	})
	// The end of the sandbox is not bound by the commit's deadline: a sandbox
	// that was paused and then not ended would linger paused. The session's
	// egress policy stays: it is the session's, and a newer turn may be using
	// it (see DeleteAgentSessionEgress).
	settle, done := context.WithTimeout(context.WithoutCancel(ctx), RetainEndTimeout)
	defer done()
	if err != nil {
		if errors.Is(err, ErrAgentSandboxPaused) {
			if endErr := s.endPausedAgentSandbox(settle, workspaceID, sessionID, sandboxID, false); endErr != nil {
				log.Printf("sandbox: end the paused sandbox %s after a failed retention: %v", sandboxID, endErr)
			}
		}
		return RetainedCommit{}, err
	}
	out := RetainedCommit{AgentImageCommit: commit}
	if endErr := s.endPausedAgentSandbox(settle, workspaceID, sessionID, sandboxID, false); endErr != nil {
		log.Printf("sandbox: end the paused sandbox %s: %v", sandboxID, endErr)
		return out, nil
	}
	out.SandboxEnded = true
	return out, nil
}

// reserveRetainedImage counts a retained image's bytes against the
// workspace's storage cap under the lock imports and builds take, so two
// stopped turns saving at once cannot both pass it.
func (s *Service) reserveRetainedImage(ctx context.Context, workspaceID, imageID string, size int64) error {
	return s.reserveImageBytes(size, func(limit int64) error {
		return s.Storage.ReserveAgentSessionRetainedImage(ctx, workspaceID, imageID, size, limit)
	})
}

// DeleteAgentSessionEgress deletes the session's egress policy, which a save
// leaves when it ends the paused sandbox: the policy is the session's, shared
// by every sandbox of its turns, so only a caller that knows no newer turn is
// using it may delete it (agentsessions checks the session's turn first).
func (l *AgentSessionLifecycle) DeleteAgentSessionEgress(ctx context.Context, workspaceID, sessionID string) error {
	s := l.service
	if s.SessionEgress == nil {
		return nil
	}
	return s.SessionEgress.Delete(ctx, store.SandboxNamespace(workspaceID), sessionID)
}

// DeleteAgentRetainedImage deletes a retained image from the registry (ADR047
// D12). An image already gone is not an error; a deployment without the
// registry or its reaper credential has nothing it can delete.
func (l *AgentSessionLifecycle) DeleteAgentRetainedImage(ctx context.Context, workspaceID, imageID string) error {
	if !agentsession.IsRetainedImage(imageID) {
		return fmt.Errorf("%w: invalid retained image id %q", core.ErrBadRequest, imageID)
	}
	if l.service.Images == nil {
		return nil
	}
	return l.service.Images.deleteImported(ctx, workspaceID, []string{imageID})
}
