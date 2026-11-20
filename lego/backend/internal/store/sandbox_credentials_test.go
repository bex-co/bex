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
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSandboxCredentialFingerprintDeterministicAndGroupSalted(t *testing.T) {
	saltA := []byte("0123456789abcdef0123456789abcdef")
	saltB := []byte("fedcba9876543210fedcba9876543210")
	cfg1 := SandboxCredentialConfig{
		Domain:  "api.example.com",
		Enabled: true,
		Methods: []string{"POST", "GET"},
		Paths:   []string{"/v2/**", "/v1/*"},
		Injection: SandboxCredentialInjection{
			Type:     "header",
			Name:     "Authorization",
			Template: "",
		},
	}
	cfg2 := SandboxCredentialConfig{
		Domain:  "api.example.com",
		Enabled: true,
		Methods: []string{"GET", "POST", "GET"},
		Paths:   []string{"/v1/*", "/v2/**"},
		Injection: SandboxCredentialInjection{
			Type:     "header",
			Name:     "Authorization",
			Template: "${SECRET}",
		},
	}
	fp1 := SandboxCredentialFingerprint(saltA, cfg1, "secret-123")
	fp2 := SandboxCredentialFingerprint(saltA, cfg2, "secret-123")
	if len(fp1) != 64 {
		t.Fatalf("fingerprint len = %d, want 64", len(fp1))
	}
	if fp1 != fp2 {
		t.Fatalf("equivalent configs hashed differently: %s vs %s", fp1, fp2)
	}
	if fpOtherGroup := SandboxCredentialFingerprint(saltB, cfg1, "secret-123"); fpOtherGroup == fp1 {
		t.Fatalf("different group salts produced identical fingerprints: %s", fp1)
	}
	if fpOtherSecret := SandboxCredentialFingerprint(saltA, cfg1, "secret-456"); fpOtherSecret == fp1 {
		t.Fatalf("different secret values produced identical fingerprints: %s", fp1)
	}
	ref := SandboxCredentialSecretRef(saltA, "secret-123")
	if len(ref) != 64 || ref == fp1 {
		t.Fatalf("secretRef = %q (fp=%q), want distinct 64-hex", ref, fp1)
	}
}

func makeCredPrepare(cfg SandboxCredentialConfig, secret, actor string) SandboxCredentialPrepareCreateFn {
	return func(_ context.Context, salt []byte) (SandboxCredentialPreparedWrite, error) {
		return SandboxCredentialPreparedWrite{
			Config:      cfg,
			UpdatedBy:   actor,
			Fingerprint: SandboxCredentialFingerprint(salt, cfg, secret),
			SecretRef:   SandboxCredentialSecretRef(salt, secret),
			SecretBytes: len(secret),
		}, nil
	}
}

func TestSandboxGroupCredentialsLifecyclePG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sb-creds").ID

	g, err := st.CreateSandboxGroup(ctx, ws, "creds-group", SandboxGroupRegion, "", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	gOther, err := st.CreateSandboxGroup(ctx, ws, "creds-group-other", SandboxGroupRegion, "", 10, 10)
	if err != nil {
		t.Fatal(err)
	}

	baseCfg := SandboxCredentialConfig{
		Domain:  "api.example.com",
		Enabled: true,
		Methods: []string{"GET"},
		Paths:   []string{"/v1/*"},
		Injection: SandboxCredentialInjection{
			Type:     "header",
			Name:     "Authorization",
			Template: "Bearer ${SECRET}",
		},
	}

	// Case-sensitive unique IDs: "ApiKey" and "apikey" coexist; duplicate "ApiKey" fails.
	cUpper, _, err := st.CreateSandboxGroupCredential(ctx, ws, g.ID, "ApiKey", makeCredPrepare(baseCfg, "sec-1", "usr-a"))
	if err != nil {
		t.Fatalf("create ApiKey: %v", err)
	}
	if cUpper.CurrentVersion.ID != 1 || cUpper.CurrentVersion.RestoredFromVersionID != nil {
		t.Fatalf("unexpected v1: %+v", cUpper.CurrentVersion)
	}
	if _, _, err := st.CreateSandboxGroupCredential(ctx, ws, g.ID, "ApiKey", makeCredPrepare(baseCfg, "sec-1", "usr-a")); !errors.Is(err, ErrSandboxCredentialExists) {
		t.Fatalf("duplicate ApiKey = %v, want ErrSandboxCredentialExists", err)
	}
	cLower, _, err := st.CreateSandboxGroupCredential(ctx, ws, g.ID, "apikey", makeCredPrepare(baseCfg, "sec-1", "usr-a"))
	if err != nil {
		t.Fatalf("create apikey: %v", err)
	}
	// Identical config+secret in same group produces identical fingerprint.
	if cLower.CurrentVersion.Fingerprint != cUpper.CurrentVersion.Fingerprint {
		t.Fatalf("same-group fingerprints differ: %s vs %s", cLower.CurrentVersion.Fingerprint, cUpper.CurrentVersion.Fingerprint)
	}
	// Identical config+secret in different group produces different fingerprint (group-salted).
	cOtherGroup, _, err := st.CreateSandboxGroupCredential(ctx, ws, gOther.ID, "ApiKey", makeCredPrepare(baseCfg, "sec-1", "usr-a"))
	if err != nil {
		t.Fatalf("create in other group: %v", err)
	}
	if cOtherGroup.CurrentVersion.Fingerprint == cUpper.CurrentVersion.Fingerprint {
		t.Fatalf("cross-group fingerprints collided: %s", cOtherGroup.CurrentVersion.Fingerprint)
	}

	// Lexicographical C-collation list ordering: "ApiKey" < "apikey".
	list, err := st.ListSandboxGroupCredentials(ctx, ws, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "ApiKey" || list[1].ID != "apikey" {
		t.Fatalf("list order = %+v, want [ApiKey, apikey]", list)
	}

	// Verify raw secret never appears in any Postgres column of either table.
	var leaked int
	if err := st.Pool.QueryRow(ctx, `
		SELECT count(*) FROM sandbox_group_credential_versions
		WHERE config_json::text LIKE '%sec-1%' OR fingerprint LIKE '%sec-1%' OR secret_ref LIKE '%sec-1%'`).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("found %d rows containing raw secret material", leaked)
	}

	// Update through 4 more distinct configs (v2, v3, v4, v5) -> v1 must be evicted.
	v1Ref := cUpper.CurrentSecretRef
	v1FP := cUpper.CurrentVersion.Fingerprint
	updateWithSecret := func(secret string, enabled bool) (SandboxCredential, []string) {
		t.Helper()
		cfg := baseCfg
		cfg.Enabled = enabled
		u, evicted, _, err := st.UpdateSandboxGroupCredential(ctx, ws, g.ID, "ApiKey", func(_ context.Context, salt []byte, _ SandboxCredential) (SandboxCredentialPreparedWrite, error) {
			return SandboxCredentialPreparedWrite{
				Config:      cfg,
				UpdatedBy:   "usr-b",
				Fingerprint: SandboxCredentialFingerprint(salt, cfg, secret),
				SecretRef:   SandboxCredentialSecretRef(salt, secret),
				SecretBytes: len(secret),
			}, nil
		})
		if err != nil {
			t.Fatalf("update(%s): %v", secret, err)
		}
		return u, evicted
	}

	v2, ev := updateWithSecret("sec-2", true)
	if v2.CurrentVersion.ID != 2 || len(ev) != 0 {
		t.Fatalf("v2 = %d, evicted = %v", v2.CurrentVersion.ID, ev)
	}
	v3, ev := updateWithSecret("sec-3", true)
	if v3.CurrentVersion.ID != 3 || len(ev) != 0 {
		t.Fatalf("v3 = %d, evicted = %v", v3.CurrentVersion.ID, ev)
	}
	v4, ev := updateWithSecret("sec-4", true)
	if v4.CurrentVersion.ID != 4 || len(ev) != 0 {
		t.Fatalf("v4 = %d, evicted = %v", v4.CurrentVersion.ID, ev)
	}
	// v5 is the 5th distinct config: current (v5) + 3 prior (v4, v3, v2) survive; v1 is evicted!
	v5, ev := updateWithSecret("sec-5", true)
	if v5.CurrentVersion.ID != 5 {
		t.Fatalf("v5 id = %d, want 5", v5.CurrentVersion.ID)
	}
	if !slices.Equal(ev, []string{v1Ref}) {
		t.Fatalf("v5 evicted = %v, want [%s]", ev, v1Ref)
	}
	// Restoring evicted v1 must fail with ErrSandboxCredentialVersionNotFound.
	if _, _, err := st.RestoreSandboxGroupCredential(ctx, ws, g.ID, "ApiKey", 1, "usr-c"); !errors.Is(err, ErrSandboxCredentialVersionNotFound) {
		t.Fatalf("restore evicted v1 = %v, want ErrSandboxCredentialVersionNotFound", err)
	}

	// Restore retained v2 -> mints v6 with restoredFromVersionId=2 and matching fingerprint!
	v6, ev, err := st.RestoreSandboxGroupCredential(ctx, ws, g.ID, "ApiKey", 2, "usr-c")
	if err != nil {
		t.Fatalf("restore v2: %v", err)
	}
	if v6.CurrentVersion.ID != 6 || v6.CurrentVersion.RestoredFromVersionID == nil || *v6.CurrentVersion.RestoredFromVersionID != 2 {
		t.Fatalf("v6 = %+v, want id=6 restoredFrom=2", v6.CurrentVersion)
	}
	if v6.CurrentVersion.Fingerprint != v2.CurrentVersion.Fingerprint || len(ev) != 0 {
		t.Fatalf("v6 fp mismatch or unexpected eviction: %s vs %s, ev=%v", v6.CurrentVersion.Fingerprint, v2.CurrentVersion.Fingerprint, ev)
	}

	// Versions list is newest first: 6, 5, 4, 3, 2 (both v6 and v2 share v2's fingerprint!).
	versions, err := st.ListSandboxGroupCredentialVersions(ctx, ws, g.ID, "ApiKey")
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := make([]int64, len(versions))
	for i, v := range versions {
		gotIDs[i] = v.ID
	}
	if !slices.Equal(gotIDs, []int64{6, 5, 4, 3, 2}) {
		t.Fatalf("version IDs = %v, want [6 5 4 3 2]", gotIDs)
	}
	if v1FP == v2.CurrentVersion.Fingerprint {
		t.Fatal("v1 and v2 fingerprints unexpectedly matched")
	}

	// Now push 4 new distinct configs (v7..v10) so v2/v6's shared fingerprint is evicted:
	// BOTH v2 AND v6 must be removed together!
	v7, ev7 := updateWithSecret("sec-7", true) // evicts v3
	if !slices.Equal(ev7, []string{v3.CurrentSecretRef}) {
		t.Fatalf("v7 evicted = %v, want v3 ref", ev7)
	}
	v8, ev8 := updateWithSecret("sec-8", true) // evicts v4
	if !slices.Equal(ev8, []string{v4.CurrentSecretRef}) {
		t.Fatalf("v8 evicted = %v, want v4 ref", ev8)
	}
	v9, ev9 := updateWithSecret("sec-9", true) // evicts v5
	if !slices.Equal(ev9, []string{v5.CurrentSecretRef}) {
		t.Fatalf("v9 evicted = %v, want v5 ref", ev9)
	}
	v10, ev10 := updateWithSecret("sec-10", true) // evicts v2+v6 (shared config)
	if !slices.Equal(ev10, []string{v2.CurrentSecretRef}) {
		t.Fatalf("v10 evicted = %v, want v2/v6 ref", ev10)
	}
	versions, err = st.ListSandboxGroupCredentialVersions(ctx, ws, g.ID, "ApiKey")
	if err != nil {
		t.Fatal(err)
	}
	gotIDs = make([]int64, len(versions))
	for i, v := range versions {
		gotIDs[i] = v.ID
	}
	if !slices.Equal(gotIDs, []int64{10, 9, 8, 7}) {
		t.Fatalf("after shared-config eviction, version IDs = %v, want [10 9 8 7]", gotIDs)
	}

	// Shared secret_ref across two distinct fingerprints (e.g. toggling enabled without changing secret):
	// v11 shares sec-10's secret_ref with v10. When v10 is evicted on v14, sec-10's secret_ref must NOT
	// be evicted because v11 still references it; when v11 is evicted on v15, sec-10's ref IS evicted.
	_, ev11 := updateWithSecret("sec-10", false) // evicts v7
	if !slices.Equal(ev11, []string{v7.CurrentSecretRef}) {
		t.Fatalf("v11 evicted = %v, want v7 ref %s", ev11, v7.CurrentSecretRef)
	}
	_, ev12 := updateWithSecret("sec-12", true) // evicts v8
	if !slices.Equal(ev12, []string{v8.CurrentSecretRef}) {
		t.Fatalf("v12 evicted = %v, want v8 ref %s", ev12, v8.CurrentSecretRef)
	}
	_, ev13 := updateWithSecret("sec-13", true) // evicts v9
	if !slices.Equal(ev13, []string{v9.CurrentSecretRef}) {
		t.Fatalf("v13 evicted = %v, want v9 ref %s", ev13, v9.CurrentSecretRef)
	}
	_, ev14 := updateWithSecret("sec-14", true) // evicts v10; v11 still uses sec-10!
	if len(ev14) != 0 {
		t.Fatalf("v14 evicted = %v, want empty (v11 still references sec-10)", ev14)
	}
	_, ev15 := updateWithSecret("sec-15", true) // evicts v11; sec-10 is now unreferenced
	if !slices.Equal(ev15, []string{v10.CurrentSecretRef}) {
		t.Fatalf("v15 evicted = %v, want sec-10 ref %s", ev15, v10.CurrentSecretRef)
	}

	// Account deletion anonymizes updated_by without removing credential versions.
	if err := st.CleanupAccountSubject(ctx, "usr-b", "deleted-usr-b"); err != nil {
		t.Fatalf("CleanupAccountSubject: %v", err)
	}
	gotCred, err := st.GetSandboxGroupCredential(ctx, ws, g.ID, "ApiKey")
	if err != nil {
		t.Fatalf("get after CleanupAccountSubject: %v", err)
	}
	if gotCred.CurrentVersion.UpdatedBy != "deleted-usr-b" {
		t.Fatalf("updated_by after CleanupAccountSubject = %q, want %q", gotCred.CurrentVersion.UpdatedBy, "deleted-usr-b")
	}

	// Idempotent delete returns all secret refs on first call and none on repeat.
	refs, deleted, err := st.DeleteSandboxGroupCredential(ctx, ws, g.ID, "ApiKey", nil)
	if err != nil || !deleted || len(refs) == 0 {
		t.Fatalf("first delete = (%v, %v, %v), want deleted with refs", refs, deleted, err)
	}
	refs2, deleted2, err := st.DeleteSandboxGroupCredential(ctx, ws, g.ID, "ApiKey", nil)
	if err != nil || deleted2 || len(refs2) != 0 {
		t.Fatalf("repeat delete = (%v, %v, %v), want idempotent no-op", refs2, deleted2, err)
	}
}

func TestSandboxGroupCredentialsMigrationDownUpPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hasTable := func(name string) bool {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = $1`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	hasSaltCol := func() bool {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'sandbox_groups' AND column_name = 'credential_fingerprint_salt'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	readMigration := func(name string) string {
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	if !hasTable("sandbox_group_credentials") || !hasTable("sandbox_group_credential_versions") || !hasSaltCol() {
		t.Fatal("0318 schema objects missing after Migrate")
	}
	if _, err := tx.Exec(ctx, readMigration("0318_sandbox_group_credentials.down.sql")); err != nil {
		t.Fatalf("down migration: %v", err)
	}
	if hasTable("sandbox_group_credentials") || hasTable("sandbox_group_credential_versions") || hasSaltCol() {
		t.Fatal("down migration left 0318 schema objects behind")
	}
	if _, err := tx.Exec(ctx, readMigration("0318_sandbox_group_credentials.up.sql")); err != nil {
		t.Fatalf("up migration reapply: %v", err)
	}
	if !hasTable("sandbox_group_credentials") || !hasTable("sandbox_group_credential_versions") || !hasSaltCol() {
		t.Fatal("up migration reapply did not restore 0318 schema objects")
	}
}

func TestSandboxGroupCredentialsCapsAndContentionPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sb-creds-caps").ID

	// 1. 25-credential cap.
	gCount, err := st.CreateSandboxGroup(ctx, ws, "cap-count", SandboxGroupRegion, "", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	smallCfg := SandboxCredentialConfig{
		Domain:    "api.example.com",
		Enabled:   true,
		Injection: SandboxCredentialInjection{Type: "header", Name: "X-Key"},
	}
	for i := range SandboxCredentialMaxPerGroup {
		if _, _, err := st.CreateSandboxGroupCredential(ctx, ws, gCount.ID, fmt.Sprintf("cred-%02d", i), makeCredPrepare(smallCfg, "s", "usr-1")); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, _, err := st.CreateSandboxGroupCredential(ctx, ws, gCount.ID, "cred-25", makeCredPrepare(smallCfg, "s", "usr-1")); !errors.Is(err, ErrSandboxCredentialLimit) {
		t.Fatalf("26th credential = %v, want ErrSandboxCredentialLimit", err)
	}

	// 2. 200 KiB current combined serialized size cap.
	gCurSize, err := st.CreateSandboxGroup(ctx, ws, "cap-cur-size", SandboxGroupRegion, "", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	bigPaths := make([]string, 100)
	for i := range bigPaths {
		bigPaths[i] = fmt.Sprintf("/segment-%03d/%s", i, strings.Repeat("a", 80))
	}
	bigCfg := SandboxCredentialConfig{
		Domain:    "api.example.com",
		Enabled:   true,
		Paths:     bigPaths, // ~9.5 KiB JSON
		Injection: SandboxCredentialInjection{Type: "header", Name: "X-Key"},
	}
	bigSecret := strings.Repeat("k", 5120) // 5 KiB => ~14.5 KiB per cred; 15th exceeds 200 KiB
	hitCurrentCap := false
	for i := range SandboxCredentialMaxPerGroup {
		_, _, err := st.CreateSandboxGroupCredential(ctx, ws, gCurSize.ID, fmt.Sprintf("big-%02d", i), makeCredPrepare(bigCfg, bigSecret, "usr-1"))
		if errors.Is(err, ErrSandboxCredentialCurrentSizeLimit) {
			hitCurrentCap = true
			break
		}
		if err != nil {
			t.Fatalf("big create %d: %v", i, err)
		}
	}
	if !hitCurrentCap {
		t.Fatal("expected 200 KiB current size cap to trigger before 25 credentials")
	}

	// 3. 1 MiB total versioned storage cap (with current < 200 KiB).
	gTotalSize, err := st.CreateSandboxGroup(ctx, ws, "cap-total-size", SandboxGroupRegion, "", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Make a config ~19 KiB total (14 KiB paths + 5 KiB secret) across 10 creds (~190 KiB current < 200 KiB).
	// Wait: with 4 distinct versions per cred, 10 * 4 * 19 KiB = 760 KiB. With ~25 KiB per cred across 8 creds = 200 KiB current,
	// 4 distinct configs = 800 KiB. Add larger version history or 15 creds * 13 KiB = 195 KiB current -> 4 distinct = 780 KiB;
	// to exceed 1 MiB while staying <= 200 KiB current, create a large initial version (190 KiB) then shrink current to small,
	// retaining 3 huge prior configs (~190 KiB each => 570 KiB) across 2 credentials = 1.14 MiB!
	hugePaths := make([]string, 1500)
	for i := range hugePaths {
		hugePaths[i] = fmt.Sprintf("/p-%04d/%s", i, strings.Repeat("z", 100))
	}
	hugeCfg := SandboxCredentialConfig{
		Domain:    "api.example.com",
		Enabled:   true,
		Paths:     hugePaths, // ~165 KiB
		Injection: SandboxCredentialInjection{Type: "header", Name: "X-Key"},
	}
	hitTotalCap := false
	for ci := range 4 {
		credID := fmt.Sprintf("hist-%d", ci)
		_, _, err := st.CreateSandboxGroupCredential(ctx, ws, gTotalSize.ID, credID, makeCredPrepare(hugeCfg, fmt.Sprintf("v1-%d", ci), "usr-1"))
		if errors.Is(err, ErrSandboxCredentialTotalSizeLimit) {
			hitTotalCap = true
			break
		}
		if err != nil {
			t.Fatalf("hist create %d: %v", ci, err)
		}
		for vi := 2; vi <= 3; vi++ {
			_, _, _, err = st.UpdateSandboxGroupCredential(ctx, ws, gTotalSize.ID, credID, func(_ context.Context, salt []byte, _ SandboxCredential) (SandboxCredentialPreparedWrite, error) {
				sec := fmt.Sprintf("v%d-%d", vi, ci)
				return SandboxCredentialPreparedWrite{
					Config:      hugeCfg,
					UpdatedBy:   "usr-1",
					Fingerprint: SandboxCredentialFingerprint(salt, hugeCfg, sec),
					SecretRef:   SandboxCredentialSecretRef(salt, sec),
					SecretBytes: len(sec),
				}, nil
			})
			if errors.Is(err, ErrSandboxCredentialTotalSizeLimit) {
				hitTotalCap = true
				break
			}
			if err != nil {
				t.Fatalf("hist update %d/%d: %v", ci, vi, err)
			}
		}
		if hitTotalCap {
			break
		}
		// Shrink current config so the next credential fits inside the 200 KiB current cap,
		// while the 3 huge prior versions remain retained against the 1 MiB total cap.
		_, _, _, err = st.UpdateSandboxGroupCredential(ctx, ws, gTotalSize.ID, credID, func(_ context.Context, salt []byte, _ SandboxCredential) (SandboxCredentialPreparedWrite, error) {
			return SandboxCredentialPreparedWrite{
				Config:      smallCfg,
				UpdatedBy:   "usr-1",
				Fingerprint: SandboxCredentialFingerprint(salt, smallCfg, "tiny"),
				SecretRef:   SandboxCredentialSecretRef(salt, "tiny"),
				SecretBytes: len("tiny"),
			}, nil
		})
		if errors.Is(err, ErrSandboxCredentialTotalSizeLimit) {
			hitTotalCap = true
			break
		}
		if err != nil {
			t.Fatalf("hist shrink %d: %v", ci, err)
		}
	}
	if !hitTotalCap {
		t.Fatal("expected 1 MiB total versioned storage cap to trigger")
	}

	// 4. Lock contention surfaces ErrSandboxCredentialBusy.
	tx, err := st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT id FROM sandbox_groups WHERE id = $1 FOR UPDATE`, gCount.ID); err != nil {
		t.Fatal(err)
	}
	origTimeout := st.SandboxCredentialLockTimeout
	st.SandboxCredentialLockTimeout = 50 * time.Millisecond
	t.Cleanup(func() { st.SandboxCredentialLockTimeout = origTimeout })
	if _, _, err := st.CreateSandboxGroupCredential(ctx, ws, gCount.ID, "contended", makeCredPrepare(smallCfg, "s", "usr-1")); !errors.Is(err, ErrSandboxCredentialBusy) {
		t.Fatalf("contended create = %v, want ErrSandboxCredentialBusy", err)
	}
}
