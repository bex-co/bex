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
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"golang.org/x/sync/errgroup"
)

// Session secrets (ADR098). Values live only in OpenBao (s.ModelKeys, the
// same SecretKV as the BYO model key): one path per workspace agent secret so
// List yields names without reading values, and one document per session for
// its inline secrets. Postgres holds names only. No value is ever returned,
// logged or put in an error.
const (
	maxWorkspaceAgentSecrets = 50
	maxSessionSecrets        = 20
	maxSecretValueBytes      = 32 << 10
	maxSessionSecretBytes    = 128 << 10

	agentSecretValueField = "value"
	// AgentSecretAuditTarget is the audit target of a workspace agent secret
	// write, like ModelKeyAuditTarget for the model key.
	AgentSecretAuditTarget = "agent-secret"
	// secretNamesEnv names the injected secrets for the driver, which scrubs
	// their values from every published part (ADR098 D6).
	secretNamesEnv = "BEX_AGENT_SECRET_NAMES"
)

var secretNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

// The platform owns these: a secret with one of them would reroute the
// model, override the driver's run env or break the sandbox (ADR098 D3).
var (
	reservedSecretPrefixes = []string{"BEX_", "ANTHROPIC_", "OPENAI_", "CLAUDE_", "CODEX_", "GEMINI_", "OPENSANDBOX_", "GIT_"}
	reservedSecretNames    = []string{
		"GOOGLE_API_KEY", "GOOGLE_GEMINI_BASE_URL", "PATH", "HOME", "USER", "SHELL", "PWD", "TERM", "LANG", "HOSTNAME",
		"NODE_OPTIONS", "LD_PRELOAD", "LD_LIBRARY_PATH", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY", "SSL_CERT_FILE",
	}
)

// SecretInput is one inline session secret (Devin's session_secrets). The
// value is write-only: no response ever carries it.
type SecretInput struct {
	Name  string `json:"name" jsonschema:"the env var name the agent reads: [A-Za-z_][A-Za-z0-9_]* (at most 128), not a platform-reserved name such as BEX_* or PATH"`
	Value string `json:"value" jsonschema:"the secret value, 1 B to 32 KiB; write-only, redacted from the transcript"`
}

// AgentSecret is a workspace agent secret as every read shows it: its name
// and scope (w9/m385, ADR098 D9), never its value.
type AgentSecret struct {
	Name string `json:"name"`
	// Scope is which sessions receive the secret without naming it:
	// workspace (every session), repo (sessions on Repo) or explicit (only
	// sessions that name it in secretNames).
	Scope string `json:"scope"`
	Repo  string `json:"repo,omitempty"`
}

// AgentSecretInput is a create, rotate or rescope of one workspace agent
// secret. An empty Value keeps the stored value (a scope-only update of an
// existing secret); a nil Scope keeps the stored scope, or gives a new secret
// workspace scope (Devin's organization secrets).
type AgentSecretInput struct {
	Value string
	Scope *string
	Repo  string
}

// SecretScopeStore persists workspace agent secret scopes (ADR098 D9).
// *store.PGStore satisfies it; nil (store off) ⇒ every secret is explicit.
type SecretScopeStore interface {
	ListAgentSecretScopes(ctx context.Context, workspaceID string) ([]store.AgentSecretScope, error)
	UpsertAgentSecretScope(ctx context.Context, workspaceID string, in store.AgentSecretScope) error
	DeleteAgentSecretScope(ctx context.Context, workspaceID, name string) error
}

// appliedSourceInline marks an applied secret whose value came from the
// session's own inline document; the other sources are the scope constants.
const appliedSourceInline = "inline"

// AgentSecretList is the workspace's agent secrets: names and scopes.
type AgentSecretList struct {
	OwnerID string        `json:"ownerId"`
	Secrets []AgentSecret `json:"secrets"`
}

func workspaceSecretsPath(workspaceID string) string {
	return "agent-sessions/" + workspaceID + "/secrets"
}

func workspaceSecretPath(workspaceID, name string) string {
	return workspaceSecretsPath(workspaceID) + "/" + name
}

func sessionSecretsPath(workspaceID, sessionID string) string {
	return "agent-sessions/" + workspaceID + "/sessions/" + sessionID + "/secrets"
}

func validateSecretName(field, name string) error {
	if !secretNamePattern.MatchString(name) {
		return core.NewBadRequestError("AGENT_SECRET_NAME_INVALID",
			"a secret name must be an env var name: a letter or _ then letters, digits or _ (at most 128)", map[string]any{"field": field})
	}
	upper := strings.ToUpper(name)
	if slices.Contains(reservedSecretNames, upper) || slices.ContainsFunc(reservedSecretPrefixes, func(p string) bool { return strings.HasPrefix(upper, p) }) {
		return core.NewBadRequestError("AGENT_SECRET_NAME_RESERVED",
			"the platform sets this env var, so a secret cannot use the name", map[string]any{"field": field, "name": name})
	}
	return nil
}

func validateSecretValue(field, value string) error {
	if value == "" || len(value) > maxSecretValueBytes || !core.ValidEnvValue(value) {
		return core.NewBadRequestError("AGENT_SECRET_VALUE_INVALID",
			fmt.Sprintf("a secret value must be 1 B to %d KiB with no NUL byte", maxSecretValueBytes>>10), map[string]any{"field": field, "maxBytes": maxSecretValueBytes})
	}
	return nil
}

func errSecretScopeInvalid(field, msg string) error {
	return core.NewBadRequestError("AGENT_SECRET_SCOPE_INVALID", msg, map[string]any{"field": field})
}

// requestedSecretScope validates the scope a write asks for; nil when it asks
// for none. A repo scope names its repository as owner/name, lowercased like
// a session's repo so the two compare equal; no other scope names one.
func requestedSecretScope(name string, scope *string, repo string) (*store.AgentSecretScope, error) {
	if scope == nil {
		if strings.TrimSpace(repo) != "" {
			return nil, errSecretScopeInvalid("repo", "only a repo scope names a repository")
		}
		return nil, nil
	}
	switch *scope {
	case store.AgentSecretScopeWorkspace, store.AgentSecretScopeExplicit:
		if strings.TrimSpace(repo) != "" {
			return nil, errSecretScopeInvalid("repo", "only a repo scope names a repository")
		}
		return &store.AgentSecretScope{Name: name, Scope: *scope}, nil
	case store.AgentSecretScopeRepo:
		normalized, err := agentsession.NormalizeRepository(repo)
		if err != nil {
			return nil, errSecretScopeInvalid("repo", "a repo scope names its repository as owner/name")
		}
		return &store.AgentSecretScope{Name: name, Scope: *scope, Repo: normalized}, nil
	default:
		return nil, errSecretScopeInvalid("scope", `a secret's scope is "workspace", "repo" or "explicit"`)
	}
}

func secretLimitError(msg string, limit int) error {
	return core.NewBadRequestError("AGENT_SECRET_LIMIT_EXCEEDED", msg, map[string]any{"limit": limit})
}

func errSecretNotFound(name string) error {
	return core.NewNotFoundError("AGENT_SECRET_NOT_FOUND", "no workspace agent secret has this name", map[string]any{"name": name})
}

func errSecretsUnavailable() error {
	return core.NewUnavailableError("AGENT_SECRETS_UNAVAILABLE", "agent secrets need the secret store, which this deployment does not configure", nil)
}

// secretStoreFailure hides the backend's error (it can name a path) behind
// the dependency-unavailable shape; the log line carries the cause only.
func secretStoreFailure(op, workspaceID string, err error) error {
	return core.HideCause(core.NewAgentSessionDependencyUnavailableError(), fmt.Errorf("agent secrets %s (workspace=%s): %w", op, workspaceID, err))
}

// AgentSecrets lists the workspace's agent secrets by name. Developers may
// list (they choose what to attach); values are never read.
func (s *Service) AgentSecrets(ctx context.Context, ownerID string) (AgentSecretList, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanCreate, ownerID)
	if err != nil {
		return AgentSecretList{}, err
	}
	if s.ModelKeys == nil {
		return AgentSecretList{}, errSecretsUnavailable()
	}
	names, err := s.workspaceSecretNames(ctx, workspaceID)
	if err != nil {
		return AgentSecretList{}, err
	}
	scopes, err := s.secretScopes(ctx, workspaceID)
	if err != nil {
		return AgentSecretList{}, err
	}
	out := AgentSecretList{OwnerID: workspaceID, Secrets: make([]AgentSecret, 0, len(names))}
	for _, name := range names {
		out.Secrets = append(out.Secrets, secretView(name, scopes))
	}
	return out, nil
}

// secretScopes reads the workspace's scope rows by name; with no scope store
// every secret is explicit.
func (s *Service) secretScopes(ctx context.Context, workspaceID string) (map[string]store.AgentSecretScope, error) {
	out := map[string]store.AgentSecretScope{}
	if s.SecretScopes == nil {
		return out, nil
	}
	rows, err := s.SecretScopes.ListAgentSecretScopes(ctx, workspaceID)
	if err != nil {
		return nil, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	for _, row := range rows {
		out[row.Name] = row
	}
	return out, nil
}

// storedScope is a secret's scope row; no row means explicit, the behavior
// every secret had before scopes.
func storedScope(name string, scopes map[string]store.AgentSecretScope) store.AgentSecretScope {
	if row, ok := scopes[name]; ok {
		return row
	}
	return store.AgentSecretScope{Name: name, Scope: store.AgentSecretScopeExplicit}
}

func secretView(name string, scopes map[string]store.AgentSecretScope) AgentSecret {
	row := storedScope(name, scopes)
	return AgentSecret{Name: name, Scope: row.Scope, Repo: row.Repo}
}

// SetAgentSecret creates, rotates or rescopes a workspace agent secret
// (admins only). Sessions read the new value and scope from their next
// dispatch.
func (s *Service) SetAgentSecret(ctx context.Context, ownerID, name string, in AgentSecretInput) (AgentSecret, error) {
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, ownerID, AgentSecretAuditTarget)
	if err != nil {
		return AgentSecret{}, err
	}
	if err := validateSecretName("name", name); err != nil {
		return AgentSecret{}, err
	}
	if in.Value != "" {
		if err := validateSecretValue("value", in.Value); err != nil {
			return AgentSecret{}, err
		}
	}
	requested, err := requestedSecretScope(name, in.Scope, in.Repo)
	if err != nil {
		return AgentSecret{}, err
	}
	if s.ModelKeys == nil || (requested != nil && requested.Scope != store.AgentSecretScopeExplicit && s.SecretScopes == nil) {
		return AgentSecret{}, errSecretsUnavailable()
	}
	names, err := s.workspaceSecretNames(ctx, workspaceID)
	if err != nil {
		return AgentSecret{}, err
	}
	exists := slices.Contains(names, name)
	if !exists && len(names) >= maxWorkspaceAgentSecrets {
		return AgentSecret{}, secretLimitError(fmt.Sprintf("a workspace holds at most %d agent secrets", maxWorkspaceAgentSecrets), maxWorkspaceAgentSecrets)
	}
	if !exists && in.Value == "" {
		return AgentSecret{}, validateSecretValue("value", "")
	}
	scopes, err := s.secretScopes(ctx, workspaceID)
	if err != nil {
		return AgentSecret{}, err
	}
	// A row left by a deleted secret of the same name is not the new
	// secret's: only scopes of secrets that still exist count.
	for row := range scopes {
		if !slices.Contains(names, row) {
			delete(scopes, row)
		}
	}
	stored := storedScope(name, scopes)
	next := stored
	switch {
	case s.SecretScopes == nil:
		next = store.AgentSecretScope{Name: name, Scope: store.AgentSecretScopeExplicit}
	case requested != nil:
		next = *requested
	case !exists:
		next = store.AgentSecretScope{Name: name, Scope: store.AgentSecretScopeWorkspace}
	}
	if in.Value != "" || !exists || widens(stored, next) {
		if err := s.checkScopedTotal(ctx, workspaceID, scopes, next, in.Value); err != nil {
			return AgentSecret{}, err
		}
	}
	if in.Value != "" {
		if err := s.ModelKeys.Put(ctx, workspaceSecretPath(workspaceID, name), map[string]string{agentSecretValueField: in.Value}); err != nil {
			return AgentSecret{}, secretStoreFailure("write", workspaceID, err)
		}
	}
	// The value is written first: if the scope write then fails, a new
	// secret reads explicit (no row), the narrower behavior.
	if s.SecretScopes != nil && (!exists || next != stored) {
		if err := s.SecretScopes.UpsertAgentSecretScope(ctx, workspaceID, next); err != nil {
			return AgentSecret{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
		}
	}
	return AgentSecret{Name: name, Scope: next.Scope, Repo: next.Repo}, nil
}

// widens reports whether moving a secret from one scope to another can add
// sessions that receive it: explicit → anything scoped, or a repo scope →
// workspace or another repository.
func widens(from, to store.AgentSecretScope) bool {
	switch from.Scope {
	case store.AgentSecretScopeExplicit:
		return to.Scope != store.AgentSecretScopeExplicit
	case store.AgentSecretScopeRepo:
		return to.Scope == store.AgentSecretScopeWorkspace || (to.Scope == store.AgentSecretScopeRepo && to.Repo != from.Repo)
	default:
		return false
	}
}

// checkScopedTotal refuses a scope that would push the secrets some session
// receives without naming them past the session total (ADR098 D3): the
// workspace-scoped values plus one repository's repo-scoped values, with
// next's value replacing its stored one. Only the totals next can change are
// read: a repo scope bounds its own repository; a workspace scope bounds the
// largest one.
func (s *Service) checkScopedTotal(ctx context.Context, workspaceID string, scopes map[string]store.AgentSecretScope, next store.AgentSecretScope, value string) error {
	if next.Scope == store.AgentSecretScopeExplicit {
		return nil
	}
	rows := maps.Clone(scopes)
	rows[next.Name] = next
	counts := func(row store.AgentSecretScope) bool {
		return row.Scope == store.AgentSecretScopeWorkspace ||
			(row.Scope == store.AgentSecretScopeRepo && (next.Scope == store.AgentSecretScopeWorkspace || row.Repo == next.Repo))
	}
	var names []string
	for name, row := range rows {
		if counts(row) && (name != next.Name || value == "") {
			names = append(names, name)
		}
	}
	values, err := s.workspaceSecretValues(ctx, workspaceID, names)
	if err != nil {
		return err
	}
	if value != "" {
		values[next.Name] = value
	}
	workspaceTotal, repoTotals := 0, map[string]int{}
	for name, row := range rows {
		switch {
		case !counts(row):
		case row.Scope == store.AgentSecretScopeWorkspace:
			workspaceTotal += len(values[name])
		default:
			repoTotals[row.Repo] += len(values[name])
		}
	}
	largest := 0
	for _, total := range repoTotals {
		largest = max(largest, total)
	}
	if workspaceTotal+largest > maxSessionSecretBytes {
		return secretLimitError(fmt.Sprintf("the secrets a session receives by scope total at most %d KiB", maxSessionSecretBytes>>10), maxSessionSecretBytes)
	}
	return nil
}

// DeleteAgentSecret removes a workspace agent secret (admins only). A session
// still referencing it fails its next dispatch AGENT_SESSION_SECRET_UNAVAILABLE.
func (s *Service) DeleteAgentSecret(ctx context.Context, ownerID, name string) error {
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, ownerID, AgentSecretAuditTarget)
	if err != nil {
		return err
	}
	if err := validateSecretName("name", name); err != nil {
		return err
	}
	if s.ModelKeys == nil {
		return errSecretsUnavailable()
	}
	names, err := s.workspaceSecretNames(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !slices.Contains(names, name) {
		return errSecretNotFound(name)
	}
	if err := s.ModelKeys.Delete(ctx, workspaceSecretPath(workspaceID, name)); err != nil {
		return secretStoreFailure("delete", workspaceID, err)
	}
	// A row left behind is harmless: no value backs it, so no dispatch
	// applies it, and a new secret of the same name starts from the default.
	if s.SecretScopes != nil {
		if err := s.SecretScopes.DeleteAgentSecretScope(ctx, workspaceID, name); err != nil {
			log.Printf("agent secrets: scope row delete failed (workspace=%s name=%s): %v", workspaceID, name, err)
		}
	}
	return nil
}

func (s *Service) workspaceSecretNames(ctx context.Context, workspaceID string) ([]string, error) {
	names, err := s.ModelKeys.List(ctx, workspaceSecretsPath(workspaceID))
	if err != nil {
		return nil, secretStoreFailure("list", workspaceID, err)
	}
	slices.Sort(names)
	return names, nil
}

// sessionSecrets is a validated attach request: the union of names the
// session ends up with and the inline values to store for it.
type sessionSecrets struct {
	names  []string
	inline map[string]string
}

func (a sessionSecrets) changed(prior []string) bool { return !slices.Equal(a.names, prior) }

// validateSecretInput checks one request's shape before anything is read:
// names, values and duplicates. Cheap, so it runs before authorization-heavy
// work and before any secret-store call.
func validateSecretInput(refs []string, inline []SecretInput) error {
	if len(refs)+len(inline) > maxSessionSecrets {
		return secretLimitError(fmt.Sprintf("a session takes at most %d secrets", maxSessionSecrets), maxSessionSecrets)
	}
	seen := map[string]bool{}
	for _, name := range refs {
		if err := validateSecretName("secretNames", name); err != nil {
			return err
		}
	}
	for _, in := range inline {
		if err := validateSecretName("secrets.name", in.Name); err != nil {
			return err
		}
		if err := validateSecretValue("secrets.value", in.Value); err != nil {
			return err
		}
		if seen[in.Name] {
			return core.NewBadRequestError("AGENT_SECRET_DUPLICATE", "an inline secret name appears twice", map[string]any{"name": in.Name})
		}
		seen[in.Name] = true
	}
	return nil
}

// attachSecrets resolves an attach request against the session's prior names
// and inline document: every reference must exist, and the union must stay
// within the session limits (ADR098 D3). It reads workspace values only to
// size them.
func (s *Service) attachSecrets(ctx context.Context, workspaceID string, prior []string, priorInline map[string]string, in SessionSecretsInput) (sessionSecrets, error) {
	out := sessionSecrets{names: slices.Clone(prior), inline: maps.Clone(priorInline)}
	if len(in.SecretNames) == 0 && len(in.Secrets) == 0 {
		return out, nil
	}
	if s.ModelKeys == nil {
		return sessionSecrets{}, errSecretsUnavailable()
	}
	if out.inline == nil {
		out.inline = map[string]string{}
	}
	for _, secret := range in.Secrets {
		out.inline[secret.Name] = secret.Value
	}
	// The same union the idempotency digest records, so the two cannot drift.
	out.names = secretNamesOf(SessionSecretsInput{SecretNames: append(slices.Clone(prior), in.SecretNames...), Secrets: in.Secrets})
	if len(out.names) > maxSessionSecrets {
		return sessionSecrets{}, secretLimitError(fmt.Sprintf("a session takes at most %d secrets", maxSessionSecrets), maxSessionSecrets)
	}
	var stored []string
	if len(in.SecretNames) > 0 || len(prior) > 0 {
		var err error
		if stored, err = s.workspaceSecretNames(ctx, workspaceID); err != nil {
			return sessionSecrets{}, err
		}
	}
	for _, name := range in.SecretNames {
		if _, ok := out.inline[name]; !ok && !slices.Contains(stored, name) {
			return sessionSecrets{}, errSecretNotFound(name)
		}
	}
	total := 0
	var referenced []string
	for _, name := range out.names {
		if value, ok := out.inline[name]; ok {
			total += len(value)
		} else if slices.Contains(stored, name) {
			referenced = append(referenced, name) // a name deleted since attach is the dispatch's to report
		}
	}
	values, err := s.workspaceSecretValues(ctx, workspaceID, referenced)
	if err != nil {
		return sessionSecrets{}, err
	}
	for _, value := range values {
		total += len(value)
	}
	if total > maxSessionSecretBytes {
		return sessionSecrets{}, secretLimitError(fmt.Sprintf("a session's secret values total at most %d KiB", maxSessionSecretBytes>>10), maxSessionSecretBytes)
	}
	return out, nil
}

// secretReadConcurrency bounds the parallel OpenBao reads of one attach,
// dispatch or scope check: a dispatch reads its named secrets plus every
// scoped one, up to maxWorkspaceAgentSecrets.
const secretReadConcurrency = 10

// workspaceSecretValues reads the named workspace secrets concurrently. A
// missing one reads as "".
func (s *Service) workspaceSecretValues(ctx context.Context, workspaceID string, names []string) (map[string]string, error) {
	values := make([]string, len(names))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(secretReadConcurrency)
	for i, name := range names {
		g.Go(func() error {
			data, err := s.ModelKeys.Get(gctx, workspaceSecretPath(workspaceID, name))
			if err != nil {
				return secretStoreFailure("read", workspaceID, err)
			}
			values[i] = data[agentSecretValueField]
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(names))
	for i, name := range names {
		out[name] = values[i]
	}
	return out, nil
}

// sessionInline reads the session's inline secret document and, when the
// store supports compare-and-set, its version (0 when absent).
func (s *Service) sessionInline(ctx context.Context, record store.AgentSession) (map[string]string, uint64, error) {
	path := sessionSecretsPath(record.WorkspaceID, record.ID)
	if versioned, ok := s.ModelKeys.(core.VersionedSecretKV); ok {
		snapshot, err := versioned.GetVersioned(ctx, path)
		if err != nil {
			return nil, 0, secretStoreFailure("read", record.WorkspaceID, err)
		}
		return snapshot.Data, snapshot.Version, nil
	}
	data, err := s.ModelKeys.Get(ctx, path)
	if err != nil {
		return nil, 0, secretStoreFailure("read", record.WorkspaceID, err)
	}
	return data, 0, nil
}

// putSessionInline writes the session's inline document. On a versioned store
// it is a compare-and-set against the version read, so a concurrent steer
// cannot drop the values another one wrote; Create passes 0 (create-only).
func (s *Service) putSessionInline(ctx context.Context, record store.AgentSession, inline map[string]string, version uint64) error {
	path := sessionSecretsPath(record.WorkspaceID, record.ID)
	var err error
	if versioned, ok := s.ModelKeys.(core.VersionedSecretKV); ok {
		_, err = versioned.PutCAS(ctx, path, inline, version)
		if errors.Is(err, core.ErrConflict) {
			return core.NewConflictError("AGENT_SESSION_CONFLICT", "agent session conflicts with existing state", nil)
		}
	} else {
		err = s.ModelKeys.Put(ctx, path, inline)
	}
	if err != nil {
		return secretStoreFailure("write", record.WorkspaceID, err)
	}
	return nil
}

// deleteSessionSecrets removes the session's inline document (ADR098 D7).
// Best effort after the row is gone: a failure is logged without the value.
func (s *Service) deleteSessionSecrets(ctx context.Context, record store.AgentSession) {
	if s.ModelKeys == nil {
		return
	}
	if err := s.ModelKeys.Delete(ctx, sessionSecretsPath(record.WorkspaceID, record.ID)); err != nil {
		log.Printf("agent secrets: session document delete failed (session=%s): %v", record.ID, err)
	}
}

var errSessionSecretUnavailable = errors.New("agent session secret unavailable")

var secretUnavailableFailure = store.AgentSessionFailure{
	Reason: "a secret attached to the session is no longer available",
	Code:   "AGENT_SESSION_SECRET_UNAVAILABLE",
}

// withSessionSecrets resolves the secrets a dispatch applies into its env
// (ADR098 D5, D9): the names the session attached (the inline value wins,
// else the workspace secret's), plus every workspace-scoped secret and every
// secret scoped to the session's repository. An attached name that resolves
// to nothing fails the dispatch rather than running the agent without it; a
// scoped secret deleted since its row was read is simply not applied. Every
// applied name joins the driver's scrub list. It returns what applied, with
// each name's source; the error never carries a value.
func (s *Service) withSessionSecrets(ctx context.Context, record store.AgentSession, env map[string]string) (map[string]string, []store.AppliedSecret, error) {
	scoped, err := s.scopedSecretsFor(ctx, record)
	if err != nil {
		return nil, nil, err
	}
	if len(record.SecretNames) == 0 && len(scoped) == 0 {
		return env, nil, nil
	}
	if s.ModelKeys == nil {
		return nil, nil, errSessionSecretUnavailable
	}
	var inline map[string]string
	if len(record.SecretNames) > 0 {
		if inline, _, err = s.sessionInline(ctx, record); err != nil {
			return nil, nil, err
		}
	}
	var referenced []string
	for _, name := range record.SecretNames {
		if _, ok := inline[name]; !ok {
			referenced = append(referenced, name)
		}
	}
	for _, row := range scoped {
		referenced = append(referenced, row.Name)
	}
	workspace, err := s.workspaceSecretValues(ctx, record.WorkspaceID, referenced)
	if err != nil {
		return nil, nil, err
	}
	out := withEnv(env, secretNamesEnv, "")
	applied := make([]store.AppliedSecret, 0, len(record.SecretNames)+len(scoped))
	names := make([]string, 0, cap(applied))
	add := func(name, value, source string) {
		out[name] = value
		names = append(names, name)
		applied = append(applied, store.AppliedSecret{Name: name, Source: source})
	}
	for _, name := range record.SecretNames {
		value, ok := inline[name]
		source := appliedSourceInline
		if !ok {
			value, source = workspace[name], store.AgentSecretScopeExplicit
		}
		if value == "" {
			log.Printf("agent-session dispatch: attached secret missing (session=%s name=%s)", record.ID, name)
			return nil, nil, errSessionSecretUnavailable
		}
		add(name, value, source)
	}
	for _, row := range scoped {
		if value := workspace[row.Name]; value != "" {
			add(row.Name, value, row.Scope)
		}
	}
	out[secretNamesEnv] = strings.Join(names, ",")
	return out, applied, nil
}

// scopedSecretsFor lists the scoped secrets that reach the session without
// being named: every workspace-scoped one and every one scoped to its
// repository, leaving out names it attached itself (their own source wins).
func (s *Service) scopedSecretsFor(ctx context.Context, record store.AgentSession) ([]store.AgentSecretScope, error) {
	if s.SecretScopes == nil || s.ModelKeys == nil {
		return nil, nil
	}
	rows, err := s.SecretScopes.ListAgentSecretScopes(ctx, record.WorkspaceID)
	if err != nil {
		return nil, err
	}
	var out []store.AgentSecretScope
	for _, row := range rows {
		if slices.Contains(record.SecretNames, row.Name) {
			continue
		}
		if row.Scope == store.AgentSecretScopeWorkspace || (row.Scope == store.AgentSecretScopeRepo && record.Repo != "" && row.Repo == record.Repo) {
			out = append(out, row)
		}
	}
	return out, nil
}

// recordAppliedSecrets keeps the session view's appliedSecrets current. It is
// informational, so a failed write is logged and the dispatch goes on.
func (s *Service) recordAppliedSecrets(ctx context.Context, record store.AgentSession, applied []store.AppliedSecret) {
	if slices.Equal(record.AppliedSecrets, applied) {
		return
	}
	if err := s.Store.SetAgentSessionAppliedSecrets(ctx, record.ID, applied); err != nil {
		log.Printf("agent-session dispatch: applied secrets not recorded (session=%s): %v", record.ID, err)
	}
}

// attachSteerSecrets stores a steer's inline secrets and returns the session's
// new name union for BeginAgentSessionTurn to record with the turn (ADR098
// D1), or nil when the names do not change. Names are only ever added, and a
// refused turn attaches none: values a refused steer wrote stay outside the
// recorded names, so no dispatch reads them and the next steer drops them.
func (s *Service) attachSteerSecrets(ctx context.Context, record store.AgentSession, in SessionSecretsInput) ([]string, error) {
	if len(in.SecretNames) == 0 && len(in.Secrets) == 0 {
		return nil, nil
	}
	if s.ModelKeys == nil {
		return nil, errSecretsUnavailable()
	}
	// The prior inline document is read even for a references-only steer:
	// its values count toward the session total and resolve its names.
	doc, version, err := s.sessionInline(ctx, record)
	if err != nil {
		return nil, err
	}
	prior := map[string]string{}
	for name, value := range doc {
		if slices.Contains(record.SecretNames, name) {
			prior[name] = value
		}
	}
	attached, err := s.attachSecrets(ctx, record.WorkspaceID, record.SecretNames, prior, in)
	if err != nil {
		return nil, err
	}
	if len(in.Secrets) > 0 {
		if err := s.putSessionInline(ctx, record, attached.inline, version); err != nil {
			return nil, err
		}
	}
	if !attached.changed(record.SecretNames) {
		return nil, nil
	}
	return attached.names, nil
}
