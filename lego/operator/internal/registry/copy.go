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

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// maxCopyDepth bounds index recursion: an image index names platform
// manifests, which name blobs; nothing legitimate nests deeper.
const maxCopyDepth = 2

// CopyManifest makes digest, a manifest in srcRepo, available in dstRepo under
// the same digest, server-side: each blob it references is cross-repository
// mounted (no bytes move) and the manifest is PUT unchanged, so the digest —
// and a deploy's image provenance — stays exact. An image index copies each
// platform manifest first. A Build Source's linked service deploys its build
// this way (ADR097 §4): the service then pulls from its own repository with its
// own credential, and no repository ever gains a read grant on another.
// The copy is also stored under tag, so registry retention (which reaps
// untagged manifests) keeps it. authorization must be allowed to read srcRepo
// and write dstRepo (the platform builder's).
func CopyManifest(ctx context.Context, httpClient *http.Client, registryHost, srcRepo, dstRepo, digest, tag, authorization string) error {
	if !imageRepoPattern.MatchString(srcRepo) || !imageRepoPattern.MatchString(dstRepo) || !IsDigest(digest) || !imageTagPattern.MatchString(tag) {
		return fmt.Errorf("invalid copy reference %s@%s → %s:%s", srcRepo, digest, dstRepo, tag)
	}
	m, err := newManifestRequester(ctx, httpClient, registryHost, authorization)
	if err != nil {
		return err
	}
	if err := m.copyManifest(srcRepo, dstRepo, digest, 0); err != nil {
		return err
	}
	manifest, mediaType, err := m.getManifest(dstRepo, digest)
	if err != nil {
		return err
	}
	return m.putManifest(dstRepo, tag, mediaType, manifest)
}

// manifestRefs is the subset of an OCI/Docker manifest or index that names
// what it references.
type manifestRefs struct {
	Config    *descriptor  `json:"config"`
	Layers    []descriptor `json:"layers"`
	Manifests []descriptor `json:"manifests"`
}

type descriptor struct {
	Digest string `json:"digest"`
}

func (m *manifestRequester) copyManifest(srcRepo, dstRepo, digest string, depth int) error {
	manifest, mediaType, err := m.getManifest(srcRepo, digest)
	if err != nil {
		return err
	}
	var refs manifestRefs
	if err := json.Unmarshal(manifest, &refs); err != nil {
		return fmt.Errorf("parse manifest %s: %w", digest, err)
	}
	if len(refs.Manifests) > 0 && depth >= maxCopyDepth {
		return fmt.Errorf("manifest %s nests indexes deeper than %d", digest, maxCopyDepth)
	}
	for _, child := range refs.Manifests {
		if !IsDigest(child.Digest) {
			return fmt.Errorf("manifest %s names an invalid child digest", digest)
		}
		if err := m.copyManifest(srcRepo, dstRepo, child.Digest, depth+1); err != nil {
			return err
		}
	}
	blobs := refs.Layers
	if refs.Config != nil {
		blobs = append([]descriptor{*refs.Config}, blobs...)
	}
	for _, blob := range blobs {
		if !IsDigest(blob.Digest) {
			return fmt.Errorf("manifest %s names an invalid blob digest", digest)
		}
		if err := m.mountBlob(srcRepo, dstRepo, blob.Digest); err != nil {
			return err
		}
	}
	return m.putManifest(dstRepo, digest, mediaType, manifest)
}

// mountBlob mounts digest from srcRepo into dstRepo. 201 is the mount; a 202
// means the registry declined to mount and opened an upload instead, which this
// copy never completes — so the upload is abandoned and the copy fails rather
// than leave a manifest naming a missing blob.
func (m *manifestRequester) mountBlob(srcRepo, dstRepo, digest string) error {
	query := url.Values{"mount": {digest}, "from": {srcRepo}}
	resp, err := m.do(http.MethodPost, fmt.Sprintf("/v2/%s/blobs/uploads/?%s", dstRepo, query.Encode()), "", nil)
	if err != nil {
		return fmt.Errorf("mount blob %s: %w", digest, err)
	}
	if resp.status == http.StatusCreated {
		return nil
	}
	if location := resp.header.Get("Location"); resp.status == http.StatusAccepted && strings.HasPrefix(location, "/v2/") {
		_, _ = m.do(http.MethodDelete, location, "", nil)
	}
	return fmt.Errorf("mount blob %s from %s into %s: status %d: %s", digest, srcRepo, dstRepo, resp.status, resp.body)
}
