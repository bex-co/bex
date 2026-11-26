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

package avatars

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg" // the two formats an avatar may be
	_ "image/png"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// UploadPattern and DownloadPattern are the two routes, mounted outside the
// auth gate.
const (
	UploadPattern   = UploadVerb + " " + UploadPath + "{token}"
	DownloadPattern = "GET " + DownloadPath + "{key}"
)

// maxDimension bounds an image's width and height. The dashboard uploads
// 128x128; a direct caller may upload larger, never a decompression bomb.
const maxDimension = 4096

// UploadHandler serves the PUT an upload URL names. The token is spent by the
// first PUT, whatever its body: a refused body needs a new URL. The body must
// be exactly the size the URL was minted for, and a PNG or JPEG of at most
// maxDimension pixels a side.
func (s *Service) UploadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Store == nil {
			core.WriteErrStatus(w, http.StatusServiceUnavailable, "avatars are not configured")
			return
		}
		token := r.PathValue("token")
		if len(token) != base64.RawURLEncoding.EncodedLen(uploadTokenBytes) {
			core.WriteErrStatus(w, http.StatusNotFound, "this upload URL is invalid or has expired")
			return
		}
		slot, err := s.Store.RedeemAvatarUpload(r.Context(), hashToken(token), s.Base.Now())
		if errors.Is(err, store.ErrNotFound) {
			core.WriteErrStatus(w, http.StatusNotFound, "this upload URL is invalid or has expired")
			return
		}
		if err != nil {
			log.Printf("avatars: redeem upload: %v", err)
			core.WriteErrStatus(w, http.StatusInternalServerError, "the upload could not be stored")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(slot.SizeBytes)+1))
		if err != nil || len(body) != slot.SizeBytes {
			core.WriteErrStatus(w, http.StatusBadRequest,
				"the upload must be exactly "+strconv.Itoa(slot.SizeBytes)+" bytes, the avatarSizeBytes it was requested for")
			return
		}
		contentType, ok := imageType(body)
		if !ok {
			core.WriteErrStatus(w, http.StatusBadRequest, "an avatar must be a PNG or JPEG image")
			return
		}
		key, err := newDownloadKey()
		if err != nil {
			core.WriteErrStatus(w, http.StatusInternalServerError, "the upload could not be stored")
			return
		}
		err = s.Store.PutAvatar(r.Context(), store.Avatar{
			Owner: slot.Owner, DownloadKey: key, ContentType: contentType, Content: body,
		}, slot.Subject)
		switch {
		case errors.Is(err, store.ErrAccountDeletionPending):
			core.WriteErrStatus(w, http.StatusForbidden, "this account is being deleted")
			return
		case errors.Is(err, store.ErrNotFound):
			core.WriteErrStatus(w, http.StatusNotFound, "the workspace no longer exists")
			return
		case err != nil:
			log.Printf("avatars: store upload: %v", err)
			core.WriteErrStatus(w, http.StatusInternalServerError, "the upload could not be stored")
			return
		}
		w.Header().Set("ETag", strconv.Quote(key))
		w.WriteHeader(http.StatusOK)
	})
}

// imageType answers the type of a PNG or JPEG image whose header decodes to
// sane dimensions. The served Content-Type is this, never what the uploader
// claimed.
func imageType(body []byte) (string, bool) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxDimension || cfg.Height > maxDimension {
		return "", false
	}
	switch format {
	case "jpeg":
		return "image/jpeg", true
	case "png":
		return "image/png", true
	}
	return "", false
}

// DownloadHandler serves an avatar image. A new upload gets a new key, so an
// image may be cached for good; nothing on the response can run.
func (s *Service) DownloadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Store == nil {
			core.WriteErrStatus(w, http.StatusServiceUnavailable, "avatars are not configured")
			return
		}
		a, err := s.Store.AvatarByDownloadKey(r.Context(), r.PathValue("key"))
		if errors.Is(err, store.ErrNotFound) {
			core.WriteErrStatus(w, http.StatusNotFound, "avatar not found")
			return
		}
		if err != nil {
			log.Printf("avatars: read: %v", err)
			core.WriteErrStatus(w, http.StatusInternalServerError, "avatar unavailable")
			return
		}
		h := w.Header()
		h.Set("Content-Type", a.ContentType)
		h.Set("Content-Length", strconv.Itoa(len(a.Content)))
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		h.Set("ETag", strconv.Quote(a.DownloadKey))
		// The API's own middleware sets these too; the bytes are a user's
		// upload, so the route does not depend on being wrapped.
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
		// The dashboard is another origin and shows it as an <img>.
		h.Set("Cross-Origin-Resource-Policy", "cross-origin")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(a.Content)
		}
	})
}
