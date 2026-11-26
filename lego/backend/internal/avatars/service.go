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

// Package avatars owns account and workspace avatars (w1/m648,
// docs/render-artifacts/avatars.md), as Render's dashboard has them: one
// uploaded image per owner or none. The dashboard crops a photo, or draws a
// monogram, into a 128x128 JPEG and uploads it through Render's two
// operations: the query avatarPresignedUpload mints a one-time upload URL,
// which the browser PUTs the image to, and the mutation deleteAvatar resets
// the owner to its default (a Gravatar, else a monogram, drawn by the
// client). Images are served from GET /v1/avatars/<key>, the URL every
// avatarDownload field answers.
//
// The bytes live in the control-plane database (migration 0348): an image is
// a few kilobytes, and a row deletes with its workspace or account like every
// other row. The upload URL is bex-api's own, so it needs no object-store
// credential, bucket CORS rule or second origin in the dashboard's CSP.
//
// Render's public REST API and MCP server have no avatar operation, so this
// package has only the GraphQL fragment and the two HTTP routes.
package avatars

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Store is the avatar rows and upload slots, and the own- id of the caller;
// *store.PGStore satisfies it.
type Store interface {
	OwnerIDForSubject(ctx context.Context, subject string) (string, error)
	CreateAvatarUpload(ctx context.Context, u store.AvatarUpload) error
	RedeemAvatarUpload(ctx context.Context, tokenHash []byte, now time.Time) (store.AvatarUpload, error)
	PutAvatar(ctx context.Context, a store.Avatar, subject string) error
	DeleteAvatar(ctx context.Context, owner core.AvatarOwner) (bool, error)
	AvatarByDownloadKey(ctx context.Context, key string) (store.Avatar, error)
	AvatarDownloadKeys(ctx context.Context, owners []core.AvatarOwner) (map[core.AvatarOwner]string, error)
}

type Service struct {
	*core.Base
	Store Store
	// PublicURL is bex-api's public origin (BEX_API_PUBLIC_URL). The upload
	// and download URLs are absolute on it; without it they are root-relative
	// paths, which the dashboard resolves against the API origin it calls.
	PublicURL string
}

// Paths of the two HTTP routes. Both are outside the auth gate: the upload
// URL's token is its credential, as a presigned URL's signature is, and an
// avatar image is shown wherever its owner is, so its 128-bit key is the only
// thing a viewer needs.
const (
	UploadPath   = "/v1/avatar-uploads/"
	DownloadPath = "/v1/avatars/"
)

// UploadTTL is how long an upload URL stays good.
const UploadTTL = 10 * time.Minute

// UploadVerb is the HTTP method the browser uploads with, Render's `verb`.
const UploadVerb = "PUT"

// PresignedUpload is Render's PresignedUpload: where and how to upload.
type PresignedUpload struct {
	URL  string
	Verb string
}

const (
	codeUnavailable = "AVATARS_UNAVAILABLE"
	codeSize        = "AVATAR_SIZE_INVALID"
	codeType        = "AVATAR_TYPE_INVALID"
)

// owner authorizes a write of ownerID's avatar and resolves whose it is. A
// workspace's avatar is its Admins' to set, as Render's "Only workspace
// admins can set the workspace image"; an account's is the account's own, so
// only a person signed in as that account may set it, and any other id reads
// as a user that does not exist (appearance.UpdateUser's rule). subject is the
// caller, kept with an upload slot so account deletion can find it.
func (s *Service) owner(ctx context.Context, ownerID, avatarType string) (core.AvatarOwner, string, error) {
	if avatarType == core.AvatarTypeWorkspace {
		id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, ownerID)
		if err != nil {
			return core.AvatarOwner{}, "", err
		}
		if s.Store == nil || id == core.DefaultTenant {
			return core.AvatarOwner{}, "", unavailable()
		}
		caller, _ := core.IdentityFrom(ctx)
		return core.WorkspaceAvatar(id), caller.Subject, nil
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return core.AvatarOwner{}, "", err
	}
	if avatarType != core.AvatarTypeAccount {
		return core.AvatarOwner{}, "", core.NewBadRequestError(codeType, "avatarType must be ACCOUNT or WORKSPACE",
			map[string]any{"allowed": []string{core.AvatarTypeAccount, core.AvatarTypeWorkspace}})
	}
	caller, ok := core.IdentityFrom(ctx)
	if !ok || caller.Subject == "" || !caller.Human {
		return core.AvatarOwner{}, "", core.NewForbiddenError("AVATAR_PERSON_REQUIRED",
			"an account's avatar belongs to a signed-in person, not an API key", nil)
	}
	if s.Store == nil {
		return core.AvatarOwner{}, "", unavailable()
	}
	own, err := s.Store.OwnerIDForSubject(ctx, caller.Subject)
	if err != nil {
		return core.AvatarOwner{}, "", err
	}
	owner := core.AccountAvatar(ownerID)
	if own == "" || owner.ID != own {
		return core.AvatarOwner{}, "", core.NotFound("user")
	}
	return owner, caller.Subject, nil
}

// PresignedUpload is Render's avatarPresignedUpload: a URL the caller PUTs
// the owner's new image to, good once and for UploadTTL, for exactly
// sizeBytes of PNG or JPEG.
func (s *Service) PresignedUpload(ctx context.Context, ownerID, avatarType string, sizeBytes int) (PresignedUpload, error) {
	owner, subject, err := s.owner(ctx, ownerID, avatarType)
	if err != nil {
		return PresignedUpload{}, err
	}
	if sizeBytes < 1 || sizeBytes > store.MaxAvatarBytes {
		return PresignedUpload{}, core.NewBadRequestError(codeSize,
			"avatarSizeBytes must be between 1 and "+strconv.Itoa(store.MaxAvatarBytes)+" (1 MiB)",
			map[string]any{"field": "avatarSizeBytes", "max": store.MaxAvatarBytes})
	}
	token, hash, err := newUploadToken()
	if err != nil {
		return PresignedUpload{}, err
	}
	if err := s.Store.CreateAvatarUpload(ctx, store.AvatarUpload{
		TokenHash: hash, Owner: owner, SizeBytes: sizeBytes, Subject: subject, ExpiresAt: s.Base.Now().Add(UploadTTL),
	}); err != nil {
		return PresignedUpload{}, mapStoreErr(err)
	}
	return PresignedUpload{URL: s.url(UploadPath + token), Verb: UploadVerb}, nil
}

// DeleteAvatar is Render's deleteAvatar: the owner goes back to its default.
// It answers true whether or not there was an image to delete.
func (s *Service) DeleteAvatar(ctx context.Context, ownerID, avatarType string) (bool, error) {
	owner, _, err := s.owner(ctx, ownerID, avatarType)
	if err != nil {
		return false, err
	}
	if _, err := s.Store.DeleteAvatar(ctx, owner); err != nil {
		return false, mapStoreErr(err)
	}
	return true, nil
}

// URLs is the reader core.AvatarBatch asks for avatarDownload fields. It is a
// type of its own, not a Service method, because it authorizes nothing: the
// verb sweeps would rightly refuse an unauthorized verb on Service.
func (s *Service) URLs() core.AvatarURLs { return downloadURLs{s} }

type downloadURLs struct{ s *Service }

// AvatarDownloadURLs answers the image URL of each owner that has one. It
// authorizes nothing: every field that asks sits on an object its resolver
// already authorized, and the URL shows only an image.
func (d downloadURLs) AvatarDownloadURLs(ctx context.Context, owners []core.AvatarOwner) (map[core.AvatarOwner]string, error) {
	s := d.s
	if s.Store == nil {
		return nil, nil
	}
	keys, err := s.Store.AvatarDownloadKeys(ctx, owners)
	if err != nil {
		return nil, err
	}
	urls := make(map[core.AvatarOwner]string, len(keys))
	for owner, key := range keys {
		urls[owner] = s.url(DownloadPath + key)
	}
	return urls, nil
}

func (s *Service) url(path string) string {
	return strings.TrimRight(s.PublicURL, "/") + path
}

// uploadTokenBytes is an upload URL token's entropy: 256 bits.
const uploadTokenBytes = 32

// newUploadToken mints an upload URL's token and the SHA-256 the store keeps
// instead of it.
func newUploadToken() (token string, hash []byte, err error) {
	raw := make([]byte, uploadTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// newDownloadKey mints an image's download key (128 random bits).
func newDownloadKey() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func unavailable() error {
	return core.NewUnavailableError(codeUnavailable, "avatars are not configured", nil)
}

// mapStoreErr answers a write refused because the account is being deleted,
// or whose workspace is gone.
func mapStoreErr(err error) error {
	switch {
	case errors.Is(err, store.ErrAccountDeletionPending):
		return core.NewAccountDeletionPendingError()
	case errors.Is(err, store.ErrNotFound):
		return core.NotFound("workspace")
	}
	return err
}
