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

package core

import (
	"context"
	"log"
	"sync"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// avatars.go is the seam through which GraphQL types that show a person or a
// workspace (User, Workspace, members, event users) read its avatar's URL
// (w1/m648) without importing the avatars feature: the composition root puts
// one AvatarBatch per document on the context, every avatarDownload field
// names its owner to it, and the first field to need an answer reads every
// owner named so far in one store query.

// The owner kinds of an avatar, Render's AvatarType.
const (
	AvatarTypeAccount   = "ACCOUNT"
	AvatarTypeWorkspace = "WORKSPACE"
)

// AvatarOwner is whose avatar: an own- id for an ACCOUNT, a tea- id for a
// WORKSPACE.
type AvatarOwner struct {
	ID   string
	Type string
}

// AccountAvatar is the owner of a person's avatar, from either spelling of
// their id (the public usr- id or the own- id it stands for).
func AccountAvatar(userID string) AvatarOwner {
	return AvatarOwner{ID: ids.UserStorageID(userID), Type: AvatarTypeAccount}
}

// WorkspaceAvatar is the owner of a workspace's avatar.
func WorkspaceAvatar(workspaceID string) AvatarOwner {
	return AvatarOwner{ID: workspaceID, Type: AvatarTypeWorkspace}
}

// AvatarURLs answers the download URL of each owner that has an avatar; an
// owner without one is absent from the map.
type AvatarURLs interface {
	AvatarDownloadURLs(ctx context.Context, owners []AvatarOwner) (map[AvatarOwner]string, error)
}

// AvatarBatch collects the owners one GraphQL document asks about and reads
// them together. Safe for concurrent use.
type AvatarBatch struct {
	urls    AvatarURLs
	mu      sync.Mutex
	pending map[AvatarOwner]struct{}
	known   map[AvatarOwner]string // "" for an owner read and found to have none
}

// NewAvatarBatch is a batch reading through urls.
func NewAvatarBatch(urls AvatarURLs) *AvatarBatch {
	return &AvatarBatch{urls: urls, pending: map[AvatarOwner]struct{}{}, known: map[AvatarOwner]string{}}
}

// Add names an owner the document will ask about. An owner named twice (one
// person on every row of an events feed) is read once.
func (b *AvatarBatch) Add(o AvatarOwner) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addLocked(o)
}

func (b *AvatarBatch) addLocked(o AvatarOwner) {
	if _, ok := b.known[o]; !ok {
		b.pending[o] = struct{}{}
	}
}

// URL answers o's download URL, "" when it has no avatar, reading every owner
// added since the last read first. A read that fails answers "" for its
// owners: an avatar is decoration, and the client then draws the default.
func (b *AvatarBatch) URL(ctx context.Context, o AvatarOwner) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addLocked(o)
	if len(b.pending) > 0 {
		owners := make([]AvatarOwner, 0, len(b.pending))
		for owner := range b.pending {
			owners = append(owners, owner)
		}
		clear(b.pending)
		urls, err := b.urls.AvatarDownloadURLs(ctx, owners)
		if err != nil {
			log.Printf("core: avatar URLs unavailable: %v", err)
		}
		for _, owner := range owners {
			b.known[owner] = urls[owner]
		}
	}
	return b.known[o]
}

type avatarBatchCtxKey struct{}

// WithAvatarBatch returns ctx carrying b, the composition root's setter.
func WithAvatarBatch(ctx context.Context, b *AvatarBatch) context.Context {
	return context.WithValue(ctx, avatarBatchCtxKey{}, b)
}

// AvatarBatchFrom returns the document's batch; nil when none is wired, and
// every avatarDownload then reads null.
func AvatarBatchFrom(ctx context.Context) *AvatarBatch {
	b, _ := ctx.Value(avatarBatchCtxKey{}).(*AvatarBatch)
	return b
}
