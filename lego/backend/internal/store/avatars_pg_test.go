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
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestAvatarsPG is w1/m648's storage: an upload slot is spent once and never
// after it expires; a second image replaces the first under a new download
// key; the batch read answers only owners that have one; a workspace's avatar
// and slots go with the workspace; an account's go with account cleanup, and
// an account whose deletion is pending cannot store a new one.
func TestAvatarsPG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	if _, err := st.Pool.Exec(ctx, `TRUNCATE avatars, avatar_uploads, account_deletions, owner_ids, tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	ws, err := st.CreateWorkspace(ctx, "avatars", PlanPro, "identity-avatar")
	if err != nil {
		t.Fatal(err)
	}
	own, err := st.OwnerIDForSubject(ctx, "identity-avatar")
	if err != nil {
		t.Fatal(err)
	}
	account := AvatarOwner{ID: own, Type: core.AvatarTypeAccount}
	workspace := AvatarOwner{ID: ws.ID, Type: core.AvatarTypeWorkspace}
	now := time.Now()

	// A slot is spent by its first redemption, and an expired one is gone.
	for _, u := range []AvatarUpload{
		{TokenHash: []byte("live"), Owner: account, SizeBytes: 10, Subject: "identity-avatar", ExpiresAt: now.Add(time.Minute)},
		{TokenHash: []byte("stale"), Owner: workspace, SizeBytes: 10, Subject: "identity-avatar", ExpiresAt: now.Add(time.Minute)},
	} {
		if err := st.CreateAvatarUpload(ctx, u); err != nil {
			t.Fatalf("create slot: %v", err)
		}
	}
	got, err := st.RedeemAvatarUpload(ctx, []byte("live"), now)
	if err != nil || got.Owner != account || got.SizeBytes != 10 || got.Subject != "identity-avatar" {
		t.Fatalf("redeem = %+v, %v", got, err)
	}
	if _, err := st.RedeemAvatarUpload(ctx, []byte("live"), now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second redemption = %v, want ErrNotFound", err)
	}
	if _, err := st.RedeemAvatarUpload(ctx, []byte("stale"), now.Add(2*time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired redemption = %v, want ErrNotFound", err)
	}
	// A slot naming a workspace that does not exist is refused.
	if err := st.CreateAvatarUpload(ctx, AvatarUpload{TokenHash: []byte("ghost"), Owner: AvatarOwner{ID: "tea-ghost", Type: core.AvatarTypeWorkspace},
		SizeBytes: 10, Subject: "identity-avatar", ExpiresAt: now.Add(time.Minute)}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("slot for a missing workspace = %v, want ErrNotFound", err)
	}

	jpeg := []byte("\xff\xd8\xff jpeg")
	if err := st.PutAvatar(ctx, Avatar{Owner: account, DownloadKey: "key-1", ContentType: "image/jpeg", Content: jpeg}, "identity-avatar"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := st.PutAvatar(ctx, Avatar{Owner: account, DownloadKey: "key-2", ContentType: "image/png", Content: []byte("png")}, "identity-avatar"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := st.AvatarByDownloadKey(ctx, "key-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replaced key still serves: %v", err)
	}
	a, err := st.AvatarByDownloadKey(ctx, "key-2")
	if err != nil || a.Owner != account || a.ContentType != "image/png" || !bytes.Equal(a.Content, []byte("png")) {
		t.Fatalf("read = %+v, %v", a, err)
	}
	if err := st.PutAvatar(ctx, Avatar{Owner: workspace, DownloadKey: "key-ws", ContentType: "image/jpeg", Content: jpeg}, "identity-avatar"); err != nil {
		t.Fatalf("put workspace: %v", err)
	}
	// An empty or oversized image never lands.
	for _, content := range [][]byte{{}, make([]byte, MaxAvatarBytes+1)} {
		if err := st.PutAvatar(ctx, Avatar{Owner: account, DownloadKey: "key-bad", ContentType: "image/jpeg", Content: content}, "identity-avatar"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("put %d bytes = %v, want ErrInvalid", len(content), err)
		}
	}

	stranger := AvatarOwner{ID: "own-nobody", Type: core.AvatarTypeAccount}
	keys, err := st.AvatarDownloadKeys(ctx, []AvatarOwner{account, workspace, stranger})
	if err != nil || len(keys) != 2 || keys[account] != "key-2" || keys[workspace] != "key-ws" {
		t.Fatalf("keys = %v, %v", keys, err)
	}

	if deleted, err := st.DeleteAvatar(ctx, workspace); err != nil || !deleted {
		t.Fatalf("delete = %v, %v", deleted, err)
	}
	if deleted, err := st.DeleteAvatar(ctx, workspace); err != nil || deleted {
		t.Fatalf("second delete = %v, %v", deleted, err)
	}

	// The workspace's avatar and open slots go with the workspace.
	if err := st.PutAvatar(ctx, Avatar{Owner: workspace, DownloadKey: "key-ws-2", ContentType: "image/jpeg", Content: jpeg}, "identity-avatar"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAvatarUpload(ctx, AvatarUpload{TokenHash: []byte("ws-slot"), Owner: workspace, SizeBytes: 10,
		Subject: "identity-avatar", ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM avatars WHERE owner_id = $1) + (SELECT count(*) FROM avatar_uploads WHERE owner_id = $1)`, ws.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("%d avatar rows outlived their workspace", left)
	}

	// The account's avatar and slots go with account cleanup, and once its
	// deletion is pending it cannot store another.
	if err := st.CreateAvatarUpload(ctx, AvatarUpload{TokenHash: []byte("acct-slot"), Owner: account, SizeBytes: 10,
		Subject: "identity-avatar", ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	deletion, err := st.BeginAccountDeletion(ctx, "identity-avatar", "avatar@example.com", nil)
	if err != nil {
		t.Fatalf("begin deletion: %v", err)
	}
	if err := st.PutAvatar(ctx, Avatar{Owner: account, DownloadKey: "key-late", ContentType: "image/jpeg", Content: jpeg}, "identity-avatar"); !errors.Is(err, ErrAccountDeletionPending) {
		t.Fatalf("put after deletion began = %v, want ErrAccountDeletionPending", err)
	}
	if err := st.CleanupAccountSubject(ctx, "identity-avatar", deletion.DeletedMarker); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM avatars WHERE owner_id = $1) + (SELECT count(*) FROM avatar_uploads WHERE subject = 'identity-avatar')`, own).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("%d avatar rows outlived account cleanup", left)
	}
}
