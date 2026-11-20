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

package store

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sandbox group credentials (w2/m532, docs/ADR122-sandbox-credential-brokering.md).
// Postgres stores credential metadata, monotonic version history (retaining
// current + 3 most recently active distinct prior configurations), group-salted
// SHA-256 fingerprints, and serialized byte counts. Secret values are NEVER
// stored in Postgres; they live write-only in OpenBao.

const (
	// SandboxCredentialMaxPerGroup is Render's per-group credential count cap.
	SandboxCredentialMaxPerGroup = 25
	// SandboxCredentialMaxCurrentBytes is Render's combined serialized size cap
	// for all current credential settings and secret values in a group (200 KiB).
	SandboxCredentialMaxCurrentBytes = 200 * 1024
	// SandboxCredentialMaxTotalBytes is Render's total storage cap per group
	// including retained version history and version metadata (1 MiB).
	SandboxCredentialMaxTotalBytes = 1024 * 1024
	// SandboxCredentialMaxPriorConfigs is how many distinct prior configurations
	// Render retains alongside the current configuration.
	SandboxCredentialMaxPriorConfigs = 3

	sandboxCredentialDefaultLockTimeout = 2 * time.Second
	sandboxCredentialSaltBytes          = 32
)

var (
	// ErrSandboxCredentialExists is returned when creating a credential with an
	// ID that already exists in the sandbox group.
	ErrSandboxCredentialExists = errors.New("sandbox group credential already exists")
	// ErrSandboxCredentialLimit is returned when creating a credential would
	// exceed SandboxCredentialMaxPerGroup.
	ErrSandboxCredentialLimit = errors.New("sandbox group credential limit reached")
	// ErrSandboxCredentialCurrentSizeLimit is returned when a write would push
	// the group's current combined serialized size past 200 KiB.
	ErrSandboxCredentialCurrentSizeLimit = errors.New("sandbox group credential current serialized size limit exceeded")
	// ErrSandboxCredentialTotalSizeLimit is returned when a write would push
	// the group's total versioned storage past 1 MiB.
	ErrSandboxCredentialTotalSizeLimit = errors.New("sandbox group credential total storage limit exceeded")
	// ErrSandboxCredentialBusy is returned when a concurrent credential write
	// holds the sandbox group lock past the lock timeout.
	ErrSandboxCredentialBusy = errors.New("sandbox group credential temporarily contended")
	// ErrSandboxCredentialVersionNotFound is returned when restoring a version
	// ID that does not exist or has been evicted from retention.
	ErrSandboxCredentialVersionNotFound = errors.New("sandbox group credential version not found")
)

// SandboxCredentialInjection is the stored non-secret injection descriptor.
type SandboxCredentialInjection struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Template string `json:"template,omitempty"`
}

// SandboxCredentialConfig is the stored non-secret routing & injection config.
type SandboxCredentialConfig struct {
	Domain    string                     `json:"domain"`
	Enabled   bool                       `json:"enabled"`
	Methods   []string                   `json:"methods"`
	Paths     []string                   `json:"paths"`
	Injection SandboxCredentialInjection `json:"injection"`
}

// SandboxCredentialVersion is one retained row of sandbox_group_credential_versions.
type SandboxCredentialVersion struct {
	ID                    int64
	UpdatedAt             time.Time
	UpdatedBy             string
	Fingerprint           string
	RestoredFromVersionID *int64
	SecretRef             string
	Config                SandboxCredentialConfig
	SecretBytes           int
}

// SandboxCredential is one row of sandbox_group_credentials joined with its
// current version metadata.
type SandboxCredential struct {
	WorkspaceID            string
	SandboxGroupID         string
	ID                     string
	Config                 SandboxCredentialConfig
	CurrentVersion         SandboxCredentialVersion
	CurrentSecretRef       string
	CurrentSerializedBytes int
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// SandboxCredentialSecretRefEntry identifies one OpenBao secret reference owned
// by a workspace/group/credential, returned for teardown cleanup.
type SandboxCredentialSecretRefEntry struct {
	SandboxGroupID string
	CredentialID   string
	SecretRef      string
}

// SandboxCredentialCommitSecretFn writes the secret value to OpenBao inside the
// serialized group write transaction immediately before Postgres commits.
// alreadyRetained is true when an existing retained version of this credential
// already references secretRef.
type SandboxCredentialCommitSecretFn func(ctx context.Context, secretRef string, alreadyRetained bool) error

// SandboxCredentialPreparedWrite carries the computed metadata and OpenBao
// pre-commit callback produced once the group salt (and current row, on update)
// are loaded under the group lock.
type SandboxCredentialPreparedWrite struct {
	Config         SandboxCredentialConfig
	UpdatedBy      string
	Fingerprint    string
	SecretRef      string
	SecretBytes    int
	OnBeforeCommit SandboxCredentialCommitSecretFn
}

// SandboxCredentialPrepareCreateFn prepares a credential create using the
// group's fingerprint salt.
type SandboxCredentialPrepareCreateFn func(ctx context.Context, salt []byte) (SandboxCredentialPreparedWrite, error)

// SandboxCredentialPrepareUpdateFn prepares a credential update using the
// group's fingerprint salt and the current locked credential row.
type SandboxCredentialPrepareUpdateFn func(ctx context.Context, salt []byte, current SandboxCredential) (SandboxCredentialPreparedWrite, error)

// CanonicalSandboxCredentialConfig returns a copy of cfg with non-nil sorted
// slices so JSON marshaling and fingerprinting are deterministic.
func CanonicalSandboxCredentialConfig(cfg SandboxCredentialConfig) SandboxCredentialConfig {
	out := cfg
	if len(cfg.Methods) == 0 {
		out.Methods = []string{}
	} else {
		out.Methods = slices.Clone(cfg.Methods)
		slices.Sort(out.Methods)
		out.Methods = slices.Compact(out.Methods)
	}
	if len(cfg.Paths) == 0 {
		out.Paths = []string{}
	} else {
		out.Paths = slices.Clone(cfg.Paths)
		slices.Sort(out.Paths)
		out.Paths = slices.Compact(out.Paths)
	}
	return out
}

// SandboxCredentialFingerprint computes the 64-hex group-salted HMAC-SHA256
// fingerprint over routing settings, injection settings, enabled state, and
// secret value (ADR122 D2). Omitting template and passing "${SECRET}" produce
// identical fingerprints.
func SandboxCredentialFingerprint(salt []byte, cfg SandboxCredentialConfig, secretValue string) string {
	norm := CanonicalSandboxCredentialConfig(cfg)
	tpl := norm.Injection.Template
	if tpl == "" {
		tpl = "${SECRET}"
	}
	payload := struct {
		Domain  string   `json:"domain"`
		Enabled bool     `json:"enabled"`
		Methods []string `json:"methods"`
		Paths   []string `json:"paths"`
		InjType string   `json:"injectionType"`
		InjName string   `json:"injectionName"`
		InjTpl  string   `json:"injectionTemplate"`
		Secret  string   `json:"secret"`
	}{
		Domain:  norm.Domain,
		Enabled: norm.Enabled,
		Methods: norm.Methods,
		Paths:   norm.Paths,
		InjType: norm.Injection.Type,
		InjName: norm.Injection.Name,
		InjTpl:  tpl,
		Secret:  secretValue,
	}
	raw, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte("bex-sandbox-cred-fp-v1\x00"))
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

// SandboxCredentialSecretRef computes the 64-hex group-salted reference key for
// one secret value in OpenBao. Domain-separated from the config fingerprint.
func SandboxCredentialSecretRef(salt []byte, secretValue string) string {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte("bex-sandbox-cred-secret-v1\x00"))
	mac.Write([]byte(secretValue))
	return hex.EncodeToString(mac.Sum(nil))
}

func sandboxCredentialConfigJSON(cfg SandboxCredentialConfig) ([]byte, error) {
	return json.Marshal(CanonicalSandboxCredentialConfig(cfg))
}

func sandboxCredentialSerializedBytes(credID string, configJSON []byte, secretBytes int) int {
	// Include credential ID + canonical JSON config + raw UTF-8 secret byte size.
	return len(credID) + len(configJSON) + secretBytes
}

func sandboxCredentialVersionMetaBytes(versionID int64, updatedAt time.Time, updatedBy, fingerprint string, restoredFrom *int64) int {
	raw, _ := json.Marshal(struct {
		ID                    int64     `json:"id"`
		UpdatedAt             time.Time `json:"updatedAt"`
		UpdatedBy             string    `json:"updatedBy"`
		Fingerprint           string    `json:"fingerprint"`
		RestoredFromVersionID *int64    `json:"restoredFromVersionId,omitempty"`
	}{
		ID:                    versionID,
		UpdatedAt:             updatedAt.UTC(),
		UpdatedBy:             updatedBy,
		Fingerprint:           fingerprint,
		RestoredFromVersionID: restoredFrom,
	})
	return len(raw)
}

func (s *PGStore) credentialLockTimeout() time.Duration {
	if s.SandboxCredentialLockTimeout > 0 {
		return s.SandboxCredentialLockTimeout
	}
	return sandboxCredentialDefaultLockTimeout
}

func isCredentialContentionErr(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	// 55P03 = lock_not_available (lock_timeout exceeded); 40001 = serialization_failure; 40P01 = deadlock_detected.
	return pgErr.Code == "55P03" || pgErr.Code == "40001" || pgErr.Code == "40P01"
}

// lockSandboxGroupForCredentials locks the live sandbox_groups row FOR UPDATE
// with a bounded lock_timeout and lazily generates its 32-byte fingerprint salt
// if unset.
func (s *PGStore) lockSandboxGroupForCredentials(ctx context.Context, tx pgx.Tx, workspaceID, groupID string) ([]byte, error) {
	ms := max(s.credentialLockTimeout().Milliseconds(), 1)
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", ms)); err != nil {
		return nil, err
	}
	var salt []byte
	err := tx.QueryRow(ctx, `
		SELECT credential_fingerprint_salt
		FROM sandbox_groups
		WHERE workspace_id = $1 AND id = $2 AND deleting_at IS NULL
		FOR UPDATE`, workspaceID, groupID).Scan(&salt)
	if err != nil {
		if isCredentialContentionErr(err) {
			return nil, ErrSandboxCredentialBusy
		}
		return nil, classify("sandbox group", err)
	}
	if len(salt) != sandboxCredentialSaltBytes {
		salt = make([]byte, sandboxCredentialSaltBytes)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("generate credential fingerprint salt: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE sandbox_groups SET credential_fingerprint_salt = $3
			WHERE workspace_id = $1 AND id = $2`, workspaceID, groupID, salt); err != nil {
			return nil, err
		}
	}
	return salt, nil
}

func checkSandboxGroupCredentialStorageCaps(ctx context.Context, tx pgx.Tx, workspaceID, groupID string) error {
	var currentBytes int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(current_serialized_bytes), 0)
		FROM sandbox_group_credentials
		WHERE workspace_id = $1 AND sandbox_group_id = $2`, workspaceID, groupID).Scan(&currentBytes); err != nil {
		return err
	}
	if currentBytes > SandboxCredentialMaxCurrentBytes {
		return ErrSandboxCredentialCurrentSizeLimit
	}
	// Total storage = sum of distinct retained config+secret bytes across the
	// group + sum of version metadata bytes across all retained version rows.
	var totalBytes int64
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT SUM(cfg_bytes) FROM (
					SELECT DISTINCT ON (credential_id, fingerprint)
						config_serialized_bytes AS cfg_bytes
					FROM sandbox_group_credential_versions
					WHERE workspace_id = $1 AND sandbox_group_id = $2
					ORDER BY credential_id, fingerprint, version_id DESC
				) distinct_cfgs
			), 0)
			+
			COALESCE((
				SELECT SUM(version_meta_bytes)
				FROM sandbox_group_credential_versions
				WHERE workspace_id = $1 AND sandbox_group_id = $2
			), 0)`, workspaceID, groupID).Scan(&totalBytes); err != nil {
		return err
	}
	if totalBytes > SandboxCredentialMaxTotalBytes {
		return ErrSandboxCredentialTotalSizeLimit
	}
	return nil
}

// pruneSandboxCredentialVersions keeps the current fingerprint plus the 3 most
// recently active distinct prior fingerprints (ordered by max version_id DESC),
// deletes all version rows referencing any evicted fingerprint, and returns the
// secret_refs that are no longer referenced by any surviving version row of
// this credential.
func pruneSandboxCredentialVersions(ctx context.Context, tx pgx.Tx, workspaceID, groupID, credID, currentFingerprint string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		WITH kept_fingerprints AS (
			SELECT $4::text AS fingerprint
			UNION
			SELECT fingerprint FROM (
				SELECT fingerprint, MAX(version_id) AS last_vid
				FROM sandbox_group_credential_versions
				WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3
				  AND fingerprint <> $4
				GROUP BY fingerprint
				ORDER BY last_vid DESC
				LIMIT $5
			) prior
		),
		deleted AS (
			DELETE FROM sandbox_group_credential_versions
			WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3
			  AND fingerprint NOT IN (SELECT fingerprint FROM kept_fingerprints)
			RETURNING secret_ref
		)
		SELECT DISTINCT d.secret_ref
		FROM deleted d
		WHERE NOT EXISTS (
			SELECT 1 FROM sandbox_group_credential_versions v
			WHERE v.workspace_id = $1 AND v.sandbox_group_id = $2 AND v.credential_id = $3
			  AND v.secret_ref = d.secret_ref
			  AND v.fingerprint IN (SELECT fingerprint FROM kept_fingerprints)
		)
		ORDER BY d.secret_ref`,
		workspaceID, groupID, credID, currentFingerprint, SandboxCredentialMaxPriorConfigs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var evicted []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, err
		}
		evicted = append(evicted, ref)
	}
	return evicted, rows.Err()
}

type sandboxCredentialVersionRow struct {
	versionID       int64
	updatedAt       time.Time
	updatedBy       string
	fingerprint     string
	restoredFrom    *int64
	secretRef       string
	configJSON      []byte
	secretBytes     int
	serializedBytes int
	metaBytes       int
}

func insertSandboxCredentialVersionTx(ctx context.Context, tx pgx.Tx, workspaceID, groupID, credID string, row sandboxCredentialVersionRow) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO sandbox_group_credential_versions (
			workspace_id, sandbox_group_id, credential_id,
			version_id, updated_at, updated_by,
			fingerprint, restored_from_version_id,
			secret_ref, config_json, secret_bytes,
			config_serialized_bytes, version_meta_bytes
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8,
			$9, $10, $11,
			$12, $13
		)`,
		workspaceID, groupID, credID,
		row.versionID, row.updatedAt, row.updatedBy,
		row.fingerprint, row.restoredFrom,
		row.secretRef, row.configJSON, row.secretBytes,
		row.serializedBytes, row.metaBytes)
	return err
}

func updateSandboxCredentialCurrentTx(
	ctx context.Context, tx pgx.Tx, workspaceID, groupID, credID string,
	versionID int64, cfg SandboxCredentialConfig, fingerprint, secretRef string, serializedBytes int, now time.Time,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE sandbox_group_credentials
		SET current_version_id = $4,
		    next_version_id = $5,
		    domain = $6,
		    enabled = $7,
		    methods = $8,
		    paths = $9,
		    injection_type = $10,
		    injection_name = $11,
		    injection_template = $12,
		    current_fingerprint = $13,
		    current_secret_ref = $14,
		    current_serialized_bytes = $15,
		    updated_at = $16
		WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3`,
		workspaceID, groupID, credID,
		versionID, versionID+1,
		cfg.Domain, cfg.Enabled, cfg.Methods, cfg.Paths,
		cfg.Injection.Type, cfg.Injection.Name, cfg.Injection.Template,
		fingerprint, secretRef, serializedBytes, now)
	return err
}

func scanSandboxCredential(row pgx.Row) (SandboxCredential, error) {
	var c SandboxCredential
	var methods, paths []string
	err := row.Scan(
		&c.WorkspaceID, &c.SandboxGroupID, &c.ID,
		&c.Config.Domain, &c.Config.Enabled, &methods, &paths,
		&c.Config.Injection.Type, &c.Config.Injection.Name, &c.Config.Injection.Template,
		&c.CurrentSecretRef, &c.CurrentSerializedBytes, &c.CreatedAt, &c.UpdatedAt,
		&c.CurrentVersion.ID, &c.CurrentVersion.UpdatedAt, &c.CurrentVersion.UpdatedBy,
		&c.CurrentVersion.Fingerprint, &c.CurrentVersion.RestoredFromVersionID,
		&c.CurrentVersion.SecretRef, &c.CurrentVersion.SecretBytes,
	)
	if err != nil {
		return SandboxCredential{}, err
	}
	if methods == nil {
		methods = []string{}
	}
	if paths == nil {
		paths = []string{}
	}
	c.Config.Methods = methods
	c.Config.Paths = paths
	c.CurrentVersion.Config = c.Config
	return c, nil
}

const selectSandboxCredentialColumns = `
	c.workspace_id, c.sandbox_group_id, c.credential_id,
	c.domain, c.enabled, c.methods, c.paths,
	c.injection_type, c.injection_name, c.injection_template,
	c.current_secret_ref, c.current_serialized_bytes, c.created_at, c.updated_at,
	v.version_id, v.updated_at, v.updated_by,
	v.fingerprint, v.restored_from_version_id,
	v.secret_ref, v.secret_bytes`

// ListSandboxGroupCredentials returns all credentials of a sandbox group sorted
// lexicographically by credential_id (C collation).
func (s *PGStore) ListSandboxGroupCredentials(ctx context.Context, workspaceID, groupID string) ([]SandboxCredential, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+selectSandboxCredentialColumns+`
		FROM sandbox_group_credentials c
		JOIN sandbox_group_credential_versions v
		  ON v.sandbox_group_id = c.sandbox_group_id
		 AND v.credential_id = c.credential_id
		 AND v.version_id = c.current_version_id
		WHERE c.workspace_id = $1 AND c.sandbox_group_id = $2
		ORDER BY c.credential_id COLLATE "C" ASC`, workspaceID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SandboxCredential, 0)
	for rows.Next() {
		c, err := scanSandboxCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetSandboxGroupCredential returns one credential in a sandbox group or
// ErrNotFound.
func (s *PGStore) GetSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string) (SandboxCredential, error) {
	c, err := scanSandboxCredential(s.Pool.QueryRow(ctx, `
		SELECT `+selectSandboxCredentialColumns+`
		FROM sandbox_group_credentials c
		JOIN sandbox_group_credential_versions v
		  ON v.sandbox_group_id = c.sandbox_group_id
		 AND v.credential_id = c.credential_id
		 AND v.version_id = c.current_version_id
		WHERE c.workspace_id = $1 AND c.sandbox_group_id = $2 AND c.credential_id = $3`,
		workspaceID, groupID, credID))
	if err != nil {
		return SandboxCredential{}, classify("sandbox group credential", err)
	}
	return c, nil
}

// ListSandboxGroupCredentialVersions returns retained versions of one
// credential, newest version ID first. Returns ErrNotFound if the credential
// does not exist.
func (s *PGStore) ListSandboxGroupCredentialVersions(ctx context.Context, workspaceID, groupID, credID string) ([]SandboxCredentialVersion, error) {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM sandbox_group_credentials
			WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3
		)`, workspaceID, groupID, credID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("sandbox group credential: %w", ErrNotFound)
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT version_id, updated_at, updated_by, fingerprint, restored_from_version_id,
		       secret_ref, config_json, secret_bytes
		FROM sandbox_group_credential_versions
		WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3
		ORDER BY version_id DESC`, workspaceID, groupID, credID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SandboxCredentialVersion, 0)
	for rows.Next() {
		var v SandboxCredentialVersion
		var rawCfg []byte
		if err := rows.Scan(&v.ID, &v.UpdatedAt, &v.UpdatedBy, &v.Fingerprint, &v.RestoredFromVersionID,
			&v.SecretRef, &rawCfg, &v.SecretBytes); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rawCfg, &v.Config); err != nil {
			return nil, fmt.Errorf("decode credential version config: %w", err)
		}
		v.Config = CanonicalSandboxCredentialConfig(v.Config)
		out = append(out, v)
	}
	return out, rows.Err()
}

// CreateSandboxGroupCredential creates a new credential in a sandbox group
// under the group row lock, enforcing uniqueness, the 25-credential cap, and
// the 200 KiB / 1 MiB storage caps atomically. It invokes prepared.OnBeforeCommit
// only after all DB checks pass and immediately before commit.
// wroteSecretRef is non-empty when OnBeforeCommit succeeded so the caller can
// clean up the newly written OpenBao entry if the commit itself fails.
func (s *PGStore) CreateSandboxGroupCredential(
	ctx context.Context, workspaceID, groupID, credID string, prepare SandboxCredentialPrepareCreateFn,
) (created SandboxCredential, wroteSecretRef string, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		salt, err := s.lockSandboxGroupForCredentials(ctx, tx, workspaceID, groupID)
		if err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM sandbox_group_credentials
				WHERE sandbox_group_id = $1 AND credential_id = $2
			)`, groupID, credID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrSandboxCredentialExists
		}
		var count int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM sandbox_group_credentials
			WHERE workspace_id = $1 AND sandbox_group_id = $2`, workspaceID, groupID).Scan(&count); err != nil {
			return err
		}
		if count >= SandboxCredentialMaxPerGroup {
			return ErrSandboxCredentialLimit
		}
		prepared, err := prepare(ctx, salt)
		if err != nil {
			return err
		}
		cfg := CanonicalSandboxCredentialConfig(prepared.Config)
		cfgJSON, err := sandboxCredentialConfigJSON(cfg)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		const versionID int64 = 1
		serializedBytes := sandboxCredentialSerializedBytes(credID, cfgJSON, prepared.SecretBytes)
		metaBytes := sandboxCredentialVersionMetaBytes(versionID, now, prepared.UpdatedBy, prepared.Fingerprint, nil)

		if _, err := tx.Exec(ctx, `
			INSERT INTO sandbox_group_credentials (
				workspace_id, sandbox_group_id, credential_id,
				current_version_id, next_version_id,
				domain, enabled, methods, paths,
				injection_type, injection_name, injection_template,
				current_fingerprint, current_secret_ref, current_serialized_bytes,
				created_at, updated_at
			) VALUES (
				$1, $2, $3,
				1, 2,
				$4, $5, $6, $7,
				$8, $9, $10,
				$11, $12, $13,
				$14, $14
			)`,
			workspaceID, groupID, credID,
			cfg.Domain, cfg.Enabled, cfg.Methods, cfg.Paths,
			cfg.Injection.Type, cfg.Injection.Name, cfg.Injection.Template,
			prepared.Fingerprint, prepared.SecretRef, serializedBytes, now); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrSandboxCredentialExists
			}
			return err
		}
		if err := insertSandboxCredentialVersionTx(ctx, tx, workspaceID, groupID, credID, sandboxCredentialVersionRow{
			versionID:       versionID,
			updatedAt:       now,
			updatedBy:       prepared.UpdatedBy,
			fingerprint:     prepared.Fingerprint,
			secretRef:       prepared.SecretRef,
			configJSON:      cfgJSON,
			secretBytes:     prepared.SecretBytes,
			serializedBytes: serializedBytes,
			metaBytes:       metaBytes,
		}); err != nil {
			return err
		}
		if err := checkSandboxGroupCredentialStorageCaps(ctx, tx, workspaceID, groupID); err != nil {
			return err
		}
		if prepared.OnBeforeCommit != nil {
			if err := prepared.OnBeforeCommit(ctx, prepared.SecretRef, false); err != nil {
				return err
			}
			wroteSecretRef = prepared.SecretRef
		}
		created = SandboxCredential{
			WorkspaceID:            workspaceID,
			SandboxGroupID:         groupID,
			ID:                     credID,
			Config:                 cfg,
			CurrentSecretRef:       prepared.SecretRef,
			CurrentSerializedBytes: serializedBytes,
			CreatedAt:              now,
			UpdatedAt:              now,
			CurrentVersion: SandboxCredentialVersion{
				ID:          versionID,
				UpdatedAt:   now,
				UpdatedBy:   prepared.UpdatedBy,
				Fingerprint: prepared.Fingerprint,
				SecretRef:   prepared.SecretRef,
				Config:      cfg,
				SecretBytes: prepared.SecretBytes,
			},
		}
		return nil
	})
	if err != nil {
		if isCredentialContentionErr(err) {
			return SandboxCredential{}, wroteSecretRef, ErrSandboxCredentialBusy
		}
		return SandboxCredential{}, wroteSecretRef, err
	}
	return created, wroteSecretRef, nil
}

// UpdateSandboxGroupCredential updates an existing credential under the group
// row lock, mints a higher monotonic version ID, prunes versions outside
// current + 3 most recently active distinct prior configurations, enforces the
// 200 KiB / 1 MiB storage caps, and calls prepared.OnBeforeCommit immediately
// before commit. Returns evicted OpenBao secret_refs to purge post-commit, and
// wroteNewSecretRef if OnBeforeCommit wrote a brand-new (previously unretained)
// secret_ref so the caller can clean it up if Commit fails.
func (s *PGStore) UpdateSandboxGroupCredential(
	ctx context.Context, workspaceID, groupID, credID string, prepare SandboxCredentialPrepareUpdateFn,
) (updated SandboxCredential, evictedSecretRefs []string, wroteNewSecretRef string, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		salt, err := s.lockSandboxGroupForCredentials(ctx, tx, workspaceID, groupID)
		if err != nil {
			return err
		}
		var nextVersionID int64
		var cur SandboxCredential
		var methods, paths []string
		err = tx.QueryRow(ctx, `
			SELECT `+selectSandboxCredentialColumns+`, c.next_version_id
			FROM sandbox_group_credentials c
			JOIN sandbox_group_credential_versions v
			  ON v.sandbox_group_id = c.sandbox_group_id
			 AND v.credential_id = c.credential_id
			 AND v.version_id = c.current_version_id
			WHERE c.workspace_id = $1 AND c.sandbox_group_id = $2 AND c.credential_id = $3`,
			workspaceID, groupID, credID).Scan(
			&cur.WorkspaceID, &cur.SandboxGroupID, &cur.ID,
			&cur.Config.Domain, &cur.Config.Enabled, &methods, &paths,
			&cur.Config.Injection.Type, &cur.Config.Injection.Name, &cur.Config.Injection.Template,
			&cur.CurrentSecretRef, &cur.CurrentSerializedBytes, &cur.CreatedAt, &cur.UpdatedAt,
			&cur.CurrentVersion.ID, &cur.CurrentVersion.UpdatedAt, &cur.CurrentVersion.UpdatedBy,
			&cur.CurrentVersion.Fingerprint, &cur.CurrentVersion.RestoredFromVersionID,
			&cur.CurrentVersion.SecretRef, &cur.CurrentVersion.SecretBytes,
			&nextVersionID,
		)
		if err != nil {
			return classify("sandbox group credential", err)
		}
		if methods == nil {
			methods = []string{}
		}
		if paths == nil {
			paths = []string{}
		}
		cur.Config.Methods = methods
		cur.Config.Paths = paths
		cur.CurrentVersion.Config = cur.Config

		prepared, err := prepare(ctx, salt, cur)
		if err != nil {
			return err
		}
		var alreadyRetained bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM sandbox_group_credential_versions
				WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3 AND secret_ref = $4
			)`, workspaceID, groupID, credID, prepared.SecretRef).Scan(&alreadyRetained); err != nil {
			return err
		}
		cfg := CanonicalSandboxCredentialConfig(prepared.Config)
		cfgJSON, err := sandboxCredentialConfigJSON(cfg)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		versionID := nextVersionID
		serializedBytes := sandboxCredentialSerializedBytes(credID, cfgJSON, prepared.SecretBytes)
		metaBytes := sandboxCredentialVersionMetaBytes(versionID, now, prepared.UpdatedBy, prepared.Fingerprint, nil)

		if err := insertSandboxCredentialVersionTx(ctx, tx, workspaceID, groupID, credID, sandboxCredentialVersionRow{
			versionID:       versionID,
			updatedAt:       now,
			updatedBy:       prepared.UpdatedBy,
			fingerprint:     prepared.Fingerprint,
			secretRef:       prepared.SecretRef,
			configJSON:      cfgJSON,
			secretBytes:     prepared.SecretBytes,
			serializedBytes: serializedBytes,
			metaBytes:       metaBytes,
		}); err != nil {
			return err
		}
		if err := updateSandboxCredentialCurrentTx(ctx, tx, workspaceID, groupID, credID,
			versionID, cfg, prepared.Fingerprint, prepared.SecretRef, serializedBytes, now); err != nil {
			return err
		}
		evicted, err := pruneSandboxCredentialVersions(ctx, tx, workspaceID, groupID, credID, prepared.Fingerprint)
		if err != nil {
			return err
		}
		if err := checkSandboxGroupCredentialStorageCaps(ctx, tx, workspaceID, groupID); err != nil {
			return err
		}
		if prepared.OnBeforeCommit != nil {
			if err := prepared.OnBeforeCommit(ctx, prepared.SecretRef, alreadyRetained); err != nil {
				return err
			}
			if !alreadyRetained {
				wroteNewSecretRef = prepared.SecretRef
			}
		}
		evictedSecretRefs = evicted
		updated = SandboxCredential{
			WorkspaceID:            workspaceID,
			SandboxGroupID:         groupID,
			ID:                     credID,
			Config:                 cfg,
			CurrentSecretRef:       prepared.SecretRef,
			CurrentSerializedBytes: serializedBytes,
			CreatedAt:              cur.CreatedAt,
			UpdatedAt:              now,
			CurrentVersion: SandboxCredentialVersion{
				ID:          versionID,
				UpdatedAt:   now,
				UpdatedBy:   prepared.UpdatedBy,
				Fingerprint: prepared.Fingerprint,
				SecretRef:   prepared.SecretRef,
				Config:      cfg,
				SecretBytes: prepared.SecretBytes,
			},
		}
		return nil
	})
	if err != nil {
		if isCredentialContentionErr(err) {
			return SandboxCredential{}, nil, wroteNewSecretRef, ErrSandboxCredentialBusy
		}
		return SandboxCredential{}, nil, wroteNewSecretRef, err
	}
	return updated, evictedSecretRefs, wroteNewSecretRef, nil
}

// RestoreSandboxGroupCredential restores a retained version as a new current
// version with a higher monotonic ID and restoredFromVersionId set. Returns any
// evicted OpenBao secret_refs to purge post-commit.
func (s *PGStore) RestoreSandboxGroupCredential(
	ctx context.Context, workspaceID, groupID, credID string, targetVersionID int64, updatedBy string,
) (restored SandboxCredential, evictedSecretRefs []string, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := s.lockSandboxGroupForCredentials(ctx, tx, workspaceID, groupID); err != nil {
			return err
		}
		var nextVersionID int64
		var createdAt time.Time
		if err := tx.QueryRow(ctx, `
			SELECT next_version_id, created_at
			FROM sandbox_group_credentials
			WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3`,
			workspaceID, groupID, credID).Scan(&nextVersionID, &createdAt); err != nil {
			return classify("sandbox group credential", err)
		}
		var targetFingerprint, targetSecretRef string
		var targetConfigJSON []byte
		var targetSecretBytes, targetConfigSerializedBytes int
		err := tx.QueryRow(ctx, `
			SELECT fingerprint, secret_ref, config_json, secret_bytes, config_serialized_bytes
			FROM sandbox_group_credential_versions
			WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3 AND version_id = $4`,
			workspaceID, groupID, credID, targetVersionID).Scan(
			&targetFingerprint, &targetSecretRef, &targetConfigJSON, &targetSecretBytes, &targetConfigSerializedBytes)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSandboxCredentialVersionNotFound
		}
		if err != nil {
			return err
		}
		var cfg SandboxCredentialConfig
		if err := json.Unmarshal(targetConfigJSON, &cfg); err != nil {
			return fmt.Errorf("decode retained credential config: %w", err)
		}
		cfg = CanonicalSandboxCredentialConfig(cfg)
		now := time.Now().UTC()
		versionID := nextVersionID
		restoredFrom := targetVersionID
		metaBytes := sandboxCredentialVersionMetaBytes(versionID, now, updatedBy, targetFingerprint, &restoredFrom)

		if err := insertSandboxCredentialVersionTx(ctx, tx, workspaceID, groupID, credID, sandboxCredentialVersionRow{
			versionID:       versionID,
			updatedAt:       now,
			updatedBy:       updatedBy,
			fingerprint:     targetFingerprint,
			restoredFrom:    &restoredFrom,
			secretRef:       targetSecretRef,
			configJSON:      targetConfigJSON,
			secretBytes:     targetSecretBytes,
			serializedBytes: targetConfigSerializedBytes,
			metaBytes:       metaBytes,
		}); err != nil {
			return err
		}
		if err := updateSandboxCredentialCurrentTx(ctx, tx, workspaceID, groupID, credID,
			versionID, cfg, targetFingerprint, targetSecretRef, targetConfigSerializedBytes, now); err != nil {
			return err
		}
		evicted, err := pruneSandboxCredentialVersions(ctx, tx, workspaceID, groupID, credID, targetFingerprint)
		if err != nil {
			return err
		}
		if err := checkSandboxGroupCredentialStorageCaps(ctx, tx, workspaceID, groupID); err != nil {
			return err
		}
		evictedSecretRefs = evicted
		restored = SandboxCredential{
			WorkspaceID:            workspaceID,
			SandboxGroupID:         groupID,
			ID:                     credID,
			Config:                 cfg,
			CurrentSecretRef:       targetSecretRef,
			CurrentSerializedBytes: targetConfigSerializedBytes,
			CreatedAt:              createdAt,
			UpdatedAt:              now,
			CurrentVersion: SandboxCredentialVersion{
				ID:                    versionID,
				UpdatedAt:             now,
				UpdatedBy:             updatedBy,
				Fingerprint:           targetFingerprint,
				RestoredFromVersionID: &restoredFrom,
				SecretRef:             targetSecretRef,
				Config:                cfg,
				SecretBytes:           targetSecretBytes,
			},
		}
		return nil
	})
	if err != nil {
		if isCredentialContentionErr(err) {
			return SandboxCredential{}, nil, ErrSandboxCredentialBusy
		}
		return SandboxCredential{}, nil, err
	}
	return restored, evictedSecretRefs, nil
}

// DeleteSandboxGroupCredential deletes one credential (cascading all its
// version rows) under the group lock and returns all OpenBao secret_refs that
// belonged to it. When onBeforeCommit is non-nil, it is invoked with the
// credential's secret_refs before Postgres commits (so a sealed/unavailable
// OpenBao fails closed and rolls back the delete rather than orphaning secrets).
// Idempotent: deleting an absent credential in a live group returns (nil, false, nil);
// a missing/deleting group returns ErrNotFound.
func (s *PGStore) DeleteSandboxGroupCredential(
	ctx context.Context, workspaceID, groupID, credID string, onBeforeCommit func(ctx context.Context, secretRefs []string) error,
) (secretRefs []string, deleted bool, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := s.lockSandboxGroupForCredentials(ctx, tx, workspaceID, groupID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT secret_ref
			FROM sandbox_group_credential_versions
			WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3
			ORDER BY secret_ref`, workspaceID, groupID, credID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var ref string
			if err := rows.Scan(&ref); err != nil {
				rows.Close()
				return err
			}
			secretRefs = append(secretRefs, ref)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM sandbox_group_credentials
			WHERE workspace_id = $1 AND sandbox_group_id = $2 AND credential_id = $3`,
			workspaceID, groupID, credID)
		if err != nil {
			return err
		}
		deleted = tag.RowsAffected() > 0
		if onBeforeCommit != nil {
			if err := onBeforeCommit(ctx, secretRefs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if isCredentialContentionErr(err) {
			return nil, false, ErrSandboxCredentialBusy
		}
		return nil, false, err
	}
	return secretRefs, deleted, nil
}

// ListSandboxGroupCredentialSecretRefs returns all distinct OpenBao secret_refs
// for a workspace (or one group when groupID != ""). Used before group/workspace
// deletion so every OpenBao entry is purged cleanly before rows cascade.
func (s *PGStore) ListSandboxGroupCredentialSecretRefs(ctx context.Context, workspaceID, groupID string) ([]SandboxCredentialSecretRefEntry, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if groupID == "" {
		rows, err = s.Pool.Query(ctx, `
			SELECT DISTINCT sandbox_group_id, credential_id, secret_ref
			FROM sandbox_group_credential_versions
			WHERE workspace_id = $1
			ORDER BY sandbox_group_id, credential_id, secret_ref`, workspaceID)
	} else {
		rows, err = s.Pool.Query(ctx, `
			SELECT DISTINCT sandbox_group_id, credential_id, secret_ref
			FROM sandbox_group_credential_versions
			WHERE workspace_id = $1 AND sandbox_group_id = $2
			ORDER BY sandbox_group_id, credential_id, secret_ref`, workspaceID, groupID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SandboxCredentialSecretRefEntry, 0)
	for rows.Next() {
		var e SandboxCredentialSecretRefEntry
		if err := rows.Scan(&e.SandboxGroupID, &e.CredentialID, &e.SecretRef); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
