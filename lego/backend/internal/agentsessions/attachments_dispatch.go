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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// AttachmentCopier extracts one tar archive at path (sandboxfiles.
// AgentAttachmentsPath or AgentSkillsPath) in a session's own sandbox. open
// returns the archive for one attempt; the copier may call it again to retry.
// *sandbox.AgentSessionLifecycle implements it over the gateway's
// /sandbox-files path.
type AttachmentCopier interface {
	UploadAgentSessionFiles(ctx context.Context, workspaceID, sessionID, sandboxID, path string, open func() io.ReadCloser) error
}

// attachmentsRoot is where the agent finds the session's input files: the
// image links it to sandboxfiles.AgentAttachmentsPath, outside /workspace, so
// an agent's `git add -A` can never commit them (ADR095 D4).
const attachmentsRoot = "/attachments"

// attachmentWait is how long the driver waits for the copy: the gateway's
// transfer bound plus the copier's retries and the provisioning in between.
const attachmentWait = sandboxfiles.DefaultTransferTimeout + 2*time.Minute

var (
	attachmentCopyFailure = store.AgentSessionFailure{
		Reason: "the session's attached files could not be copied into its sandbox",
		Code:   "ATTACHMENT_COPY_FAILED",
	}
	attachmentListFailure = store.AgentSessionFailure{Reason: "the session's attached files could not be read"}
)

// attachmentManifest is BEX_AGENT_ATTACHMENTS: what the driver waits for and
// presents to the agent.
type attachmentManifest struct {
	WaitMs int64                    `json:"waitMs"`
	Files  []attachmentManifestFile `json:"files"`
}

type attachmentManifestFile struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	// Turn is the turn that first attached the file; Current marks the files
	// this dispatch's turn attached, including ones it attached again.
	Turn    int  `json:"turn"`
	Current bool `json:"current"`
}

func attachmentEntry(a store.AgentAttachment) string {
	return strconv.Itoa(a.Turn) + "/" + a.ID + "-" + a.Name
}

func withAttachmentManifest(env map[string]string, current []string, inputs []store.AgentAttachment) map[string]string {
	manifest := attachmentManifest{WaitMs: attachmentWait.Milliseconds(), Files: make([]attachmentManifestFile, len(inputs))}
	for i, a := range inputs {
		manifest.Files[i] = attachmentManifestFile{
			Path: attachmentsRoot + "/" + attachmentEntry(a), Name: a.Name,
			MediaType: a.MediaType, Size: a.SizeBytes, Turn: a.Turn, Current: slices.Contains(current, a.ID),
		}
	}
	encoded, _ := json.Marshal(manifest)
	return withEnv(env, "BEX_AGENT_ATTACHMENTS", string(encoded))
}

// withEnv returns a copy of a dispatch env with key set; the caller's map is
// never mutated.
func withEnv(env map[string]string, key, value string) map[string]string {
	out := maps.Clone(env)
	if out == nil {
		out = map[string]string{}
	}
	out[key] = value
	return out
}

// turnLoader reads a dispatch's turn row at most once, and only when a
// dispatch helper asks for it.
func (s *Service) turnLoader(ctx context.Context, sessionID string, turn int) func() (store.AgentSessionTurn, error) {
	var (
		loaded bool
		row    store.AgentSessionTurn
		err    error
	)
	return func() (store.AgentSessionTurn, error) {
		if loaded {
			return row, err
		}
		loaded = true
		var turns []store.AgentSessionTurn
		if turns, err = s.Store.AgentSessionTurns(ctx, sessionID); err != nil {
			return row, err
		}
		for _, t := range turns {
			if t.Turn == turn {
				row = t
			}
		}
		return row, nil
	}
}

// dispatchAttachments lists the inputs a dispatch must deliver and the ids its
// turn attached; none when the feature is off.
func (s *Service) dispatchAttachments(ctx context.Context, sessionID string, currentTurn func() (store.AgentSessionTurn, error)) ([]store.AgentAttachment, []string, error) {
	if !s.attachmentsEnabled() {
		return nil, nil, nil
	}
	inputs, err := s.AttachmentRows.ListAgentSessionInputs(ctx, sessionID)
	if err != nil || len(inputs) == 0 {
		return nil, nil, err
	}
	turn, err := currentTurn()
	if err != nil {
		return nil, nil, err
	}
	current := make([]string, 0, len(turn.Attachments))
	for _, a := range turn.Attachments {
		current = append(current, a.ID)
	}
	return inputs, current, nil
}

// copyAttachments streams every input from the bucket into the sandbox as one
// tar archive of `<turn>/<att-id>-<name>` entries: one gateway transfer and
// one extraction per dispatch.
func (s *Service) copyAttachments(ctx context.Context, workspaceID, sessionID, sandboxID string, inputs []store.AgentAttachment) error {
	if len(inputs) == 0 {
		return nil
	}
	return s.AttachmentCopier.UploadAgentSessionFiles(ctx, workspaceID, sessionID, sandboxID, sandboxfiles.AgentAttachmentsPath, func() io.ReadCloser {
		reader, writer := io.Pipe()
		go func() { writer.CloseWithError(s.writeAttachmentTar(ctx, writer, inputs)) }()
		return reader
	})
}

// writeAttachmentTar writes read-only regular files of exactly the recorded
// size. The gateway extracts only archives whose parent directories precede
// their children and that never repeat a name, so each `<turn>/` directory is
// written once, before its first file.
func (s *Service) writeAttachmentTar(ctx context.Context, w io.Writer, files []store.AgentAttachment) error {
	archive := tar.NewWriter(w)
	dirs := map[string]bool{}
	for _, a := range files {
		if dir := strconv.Itoa(a.Turn); !dirs[dir] {
			dirs[dir] = true
			if err := tarDir(archive, dir, a.CreatedAt); err != nil {
				return err
			}
		}
		if err := tarFile(archive, attachmentEntry(a), a.SizeBytes, a.CreatedAt); err != nil {
			return err
		}
		body, err := s.Attachments.Get(ctx, a.ObjectKey)
		if err != nil {
			return err
		}
		n, err := io.Copy(archive, io.LimitReader(body, a.SizeBytes))
		_ = body.Close()
		if err != nil {
			return err
		}
		if n != a.SizeBytes {
			return fmt.Errorf("attachment %s: object has %d bytes, recorded %d", a.ID, n, a.SizeBytes)
		}
	}
	return archive.Close()
}

// tarDir and tarFile write the read-only entries the gateway's extractor
// accepts (sandboxfiles): a directory before its children, files of an exact
// size. Attachments and skills share them so the archive format lives here.
func tarDir(archive *tar.Writer, name string, at time.Time) error {
	return archive.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name + "/", Mode: 0o555, ModTime: at})
}

func tarFile(archive *tar.Writer, name string, size int64, at time.Time) error {
	return archive.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o444, Size: size, ModTime: at})
}

// sweepAttachments expires unconfirmed and unattached uploads, then purges the
// objects of retired rows batch by batch until a short batch or the pass
// budget. A row whose object fails to delete keeps its row for a later pass.
func sweepAttachments(ctx context.Context, s *Service, now time.Time) error {
	if !s.attachmentsEnabled() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, attachmentSweepBudget)
	defer cancel()
	if _, err := s.AttachmentRows.ExpireAgentAttachments(ctx, now); err != nil {
		return err
	}
	for {
		batch, err := s.AttachmentRows.ClaimRetiredAgentAttachments(ctx, attachmentSweepBatch, now)
		if err != nil || len(batch) == 0 {
			return err
		}
		keys := make([]string, len(batch))
		byKey := make(map[string]string, len(batch))
		for i, a := range batch {
			keys[i], byKey[a.ObjectKey] = a.ObjectKey, a.ID
		}
		gone, err := s.Attachments.DeleteMany(ctx, keys)
		if err != nil {
			log.Printf("agent attachment sweep: delete %d objects failed: %v", len(keys), err)
		}
		purged := make([]string, 0, len(gone))
		for _, key := range gone {
			purged = append(purged, byKey[key])
		}
		if err := s.AttachmentRows.PurgeAgentAttachments(ctx, purged); err != nil {
			return err
		}
		if len(batch) < attachmentSweepBatch || err != nil {
			return nil
		}
	}
}

// RunAttachmentSweep runs the attachment sweep every interval until ctx ends.
func RunAttachmentSweep(ctx context.Context, s *Service, interval time.Duration) {
	if s == nil {
		return
	}
	core.Poll(ctx, "agent attachment sweep", interval, func(ctx context.Context) error {
		return sweepAttachments(ctx, s, time.Now())
	})
}
