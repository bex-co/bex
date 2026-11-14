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

// AgentImage names a repository environment build's image an agent sandbox
// boots from: the build's id and the image's digest, which place it in the
// workspace's own images repository. The zero value boots the template image.
type AgentImage struct {
	BuildID string
	Digest  string
}

// AgentImageCommit is a committed build image: its digest and compressed size.
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
	if !agentsession.IsBuildPrincipal(image.BuildID) || !imageDigestPattern.MatchString(image.Digest) {
		return "", fmt.Errorf("%w: invalid agent image %q@%q", core.ErrBadRequest, image.BuildID, image.Digest)
	}
	repo, err := s.Images.imageRepository(workspaceID, image.BuildID)
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
	if s.Images == nil {
		return AgentImageCommit{}, errAgentImagesUnavailable()
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return AgentImageCommit{}, err
	}
	raw, err := s.agentSessionSandbox(ctx, key, workspaceID, buildID, sandboxID)
	if err != nil {
		return AgentImageCommit{}, err
	}
	res, err := s.sessionExec(ctx, workspaceID, buildID, sandboxID,
		"rm -f "+agentSessionStatusPath+" && test ! -e "+agentSessionStatusPath, maxExecOutputBytes)
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("exit %d", res.ExitCode)
	}
	if err != nil {
		return AgentImageCommit{}, fmt.Errorf("remove the build's status file: %w", err)
	}
	if err := s.checkStorageCap(ctx, raw, "build"); err != nil {
		return AgentImageCommit{}, fmt.Errorf("%w: %w", ErrAgentImageStorageLimit, err)
	}
	s.Meter.Observe(ctx, raw)
	if err := s.Client.Suspend(ctx, key, sandboxID); err != nil {
		return AgentImageCommit{}, fmt.Errorf("pause the builder: %w", err)
	}
	raw.Status.State = string(StatusSuspending)
	s.Meter.Observe(ctx, raw)
	if err := s.awaitSuspended(ctx, key, sandboxID); err != nil {
		return AgentImageCommit{}, err
	}
	img, size, err := s.Images.snapshotImage(ctx, workspaceID, sandboxID)
	if err != nil {
		return AgentImageCommit{}, err
	}
	if size > s.Images.maxBytes() {
		return AgentImageCommit{}, fmt.Errorf("%w: the image is %s, over the %s an image may hold", ErrAgentImageStorageLimit, formatGB(size), formatGB(s.Images.maxBytes()))
	}
	if err := s.reserveAgentImage(ctx, workspaceID, buildID, size); err != nil {
		return AgentImageCommit{}, err
	}
	digest, err := s.Images.pushAgentImage(ctx, workspaceID, buildID, img)
	if err != nil {
		return AgentImageCommit{}, err
	}
	return AgentImageCommit{Digest: digest, SizeBytes: size}, nil
}

// reserveAgentImage counts a build image's bytes against the workspace's
// storage cap under the lock imports take, so two builds committing at once
// cannot both pass it.
func (s *Service) reserveAgentImage(ctx context.Context, workspaceID, buildID string, size int64) error {
	if s.MaxStorageBytes <= 0 || s.Storage == nil {
		return nil
	}
	err := s.Storage.ReserveAgentRepoEnvironmentBuildImage(ctx, workspaceID, buildID, size, s.MaxStorageBytes)
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
	s := l.service
	if err := s.terminateAgentSandbox(ctx, workspaceID, buildID, sandboxID); err != nil {
		return err
	}
	if s.Images != nil && s.Images.Reaper != nil {
		if err := s.Images.Reaper.DeleteImages(ctx, workspaceID, sandboxID); err != nil {
			log.Printf("sandbox: delete builder %s's snapshot: %v", sandboxID, err)
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
