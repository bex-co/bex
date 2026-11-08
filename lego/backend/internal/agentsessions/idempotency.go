package agentsessions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const maxIdempotencyKeyLen = 128

// idempotencyOf validates the client's key and digests the normalized request
// it guards. The digest covers everything that shapes the session the client
// asked for, so retrying the same intent matches and changing any of it does
// not. No key means no idempotency (both results empty).
func idempotencyOf(workspaceID string, req CreateRequest, output store.AgentTurnOutput) (key, hash string, err error) {
	key = req.IdempotencyKey
	if key == "" {
		return "", "", nil
	}
	if len(key) > maxIdempotencyKeyLen {
		return "", "", core.NewBadRequestError("AGENT_SESSION_IDEMPOTENCY_KEY_INVALID", "idempotency key must be 1-128 printable ASCII characters", nil)
	}
	for _, r := range key {
		if r < 0x21 || r > 0x7e {
			return "", "", core.NewBadRequestError("AGENT_SESSION_IDEMPOTENCY_KEY_INVALID", "idempotency key must be 1-128 printable ASCII characters", nil)
		}
	}
	egress := slices.Clone(req.EgressAllowlist)
	slices.Sort(egress)
	digest, err := json.Marshal(struct {
		Workspace string      `json:"w"`
		Repo      string      `json:"r"`
		Branch    string      `json:"b"`
		Config    AgentConfig `json:"c"`
		Egress    []string    `json:"e"`
		Title     string      `json:"t"`
		Tags      []string    `json:"g"`
		// omitempty keeps every digest made before attachments existed (ADR095).
		Attachments []string `json:"a,omitempty"`
		// The normalized output contract (ADR094 D5), likewise omitted when unset.
		OutputSchema   json.RawMessage `json:"o,omitempty"`
		OutputRequired bool            `json:"q,omitempty"`
		// The per-session spend ceiling (ADR094 D8), omitted when unset.
		MaxSpendUSD *float64 `json:"m,omitempty"`
		// Attached secret names (ADR098), never their values; omitted when none.
		Secrets []string `json:"s,omitempty"`
		// Invoked skills and their arguments (ADR099 D4); omitted when none.
		Skills []SkillInvocation `json:"k,omitempty"`
		// The parent (ADR102) and whether the child inherits all its parent's
		// secrets (no secretNames given); omitted for a top-level session.
		Parent  string `json:"p,omitempty"`
		Inherit bool   `json:"i,omitempty"`
	}{workspaceID, req.Repo, req.Branch, req.AgentConfig, egress, req.Title, req.Tags, req.AttachmentIDs, output.Schema, output.Required, req.MaxSpendUSD, secretNamesOf(req.SessionSecretsInput), req.Skills, req.ParentSessionID, req.ParentSessionID != "" && req.SecretNames == nil && len(req.Secrets) == 0})
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(digest)
	return key, hex.EncodeToString(sum[:]), nil
}

// replayCreate returns the session an idempotency key already names. found is
// false when no live session holds the key; the same key with a different
// request digest is AGENT_SESSION_IDEMPOTENCY_KEY_REUSED.
func (s *Service) replayCreate(ctx context.Context, workspaceID, key, hash string) (View, bool, error) {
	existing, storedHash, err := s.Store.FindAgentSessionByIdempotencyKey(ctx, workspaceID, key)
	if errors.Is(err, store.ErrNotFound) {
		return View{}, false, nil
	}
	if err != nil {
		return View{}, false, err
	}
	if storedHash != hash {
		return View{}, false, core.NewConflictError("AGENT_SESSION_IDEMPOTENCY_KEY_REUSED",
			"this idempotency key was already used for a different agent session request", map[string]any{"id": existing.ID})
	}
	view, err := s.toView(existing)
	if err != nil {
		return View{}, false, err
	}
	created := false
	view.Created = &created
	return view, true, nil
}

// secretNamesOf is the sorted, distinct names a request attaches.
func secretNamesOf(in SessionSecretsInput) []string {
	names := slices.Clone(in.SecretNames)
	for _, secret := range in.Secrets {
		names = append(names, secret.Name)
	}
	slices.Sort(names)
	return slices.Compact(names)
}
