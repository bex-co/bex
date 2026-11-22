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
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// imageFileTimeout bounds the layer download of ImageFileSHA256.
	imageFileTimeout = 30 * time.Second
	// maxImageFileLayerBytes caps the compressed layer, checked against its
	// descriptor before the download; maxImageFileBytes caps the file in it.
	maxImageFileLayerBytes = 64 << 20
	maxImageFileBytes      = 128 << 20
)

// ImageFileSHA256 returns the hex sha256 of file in the layer that the build
// step step (an image config history "created_by") added to repo@digest's
// image for platform, read anonymously. The operator compares an image's
// /kv-internal-proxy with its own for its own architecture (w6/m469);
// deploy.yml compares two production images' SNI proxies for linux/amd64 from
// an arm64 runner (w6/m470). The manifest, config and layer are each checked
// against their digests, through the same bounded reader as ReadImageConfig.
// Any doubt is an error: a reference that is not a digest, a build step that
// is missing or repeated, a history that disagrees with the layers, an
// unexpected layer type, or a size over its cap.
func ImageFileSHA256(ctx context.Context, httpClient *http.Client, registryHost, repo, digest string, platform Platform, step, file string) (string, error) {
	if !IsDigest(digest) {
		return "", fmt.Errorf("image reference %q is not a digest", digest)
	}
	_, _, document, err := readImageConfigDocument(ctx, httpClient, registryHost, repo, digest, "", platform)
	if err != nil {
		return "", err
	}
	layer, err := stepLayer(document, step)
	if err != nil {
		return "", err
	}
	if layer.MediaType != "application/vnd.oci.image.layer.v1.tar+gzip" &&
		layer.MediaType != "application/vnd.docker.image.rootfs.diff.tar.gzip" {
		return "", fmt.Errorf("layer media type %q is not a gzip tar", layer.MediaType)
	}
	if !IsDigest(layer.Digest) || layer.Size <= 0 || layer.Size > maxImageFileLayerBytes {
		return "", fmt.Errorf("layer %q of %d bytes is not a digest of at most %d bytes", layer.Digest, layer.Size, maxImageFileLayerBytes)
	}
	base, err := imageRepoBase(registryHost, repo, digest, "")
	if err != nil {
		return "", err
	}
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	client := *httpClient
	client.Timeout = imageFileTimeout
	client.CheckRedirect = redirectPolicy
	resp, err := getBlob(ctx, &client, base+"blobs/"+layer.Digest)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	return fileInLayer(resp.Body, layer, file)
}

// stepLayer maps step's history entry to its layer: each history entry that
// is not an empty layer added the next layer, in order.
func stepLayer(document imageConfigDocument, step string) (layerDescriptor, error) {
	found, added := -1, 0
	for _, entry := range document.History {
		if entry.EmptyLayer {
			continue
		}
		if entry.CreatedBy == step {
			if found >= 0 {
				return layerDescriptor{}, fmt.Errorf("build step %q appears twice", step)
			}
			found = added
		}
		added++
	}
	if added != len(document.layers) {
		return layerDescriptor{}, fmt.Errorf("image history adds %d layers, its manifest lists %d", added, len(document.layers))
	}
	if found < 0 {
		return layerDescriptor{}, fmt.Errorf("image has no build step %q", step)
	}
	return document.layers[found], nil
}

// getBlob GETs a blob anonymously, answering one Bearer challenge.
func getBlob(ctx context.Context, client *http.Client, target string) (*http.Response, error) {
	bearer := ""
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, &TransientError{err: err}
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized || bearer != "" {
			return nil, statusFailure("image layer request", resp.StatusCode, resp.Header)
		}
		if bearer, err = bearerToken(ctx, client, resp.Header.Get("WWW-Authenticate"), ""); err != nil {
			return nil, err
		}
	}
}

// fileInLayer hashes file in a gzip tar layer, then reads the rest so the
// whole layer is checked against its digest before the hash is trusted.
func fileInLayer(body io.Reader, layer layerDescriptor, file string) (string, error) {
	compressed := sha256.New()
	stream := io.TeeReader(io.LimitReader(body, layer.Size+1), compressed)
	unzipped, err := gzip.NewReader(stream)
	if err != nil {
		return "", fmt.Errorf("image layer: %w", err)
	}
	sum, err := fileInTar(tar.NewReader(unzipped), file)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(io.Discard, stream); err != nil {
		return "", fmt.Errorf("image layer: %w", err)
	}
	if got := "sha256:" + hex.EncodeToString(compressed.Sum(nil)); got != layer.Digest {
		return "", errors.New("image layer digest mismatch")
	}
	return sum, nil
}

func fileInTar(r *tar.Reader, file string) (string, error) {
	for {
		header, err := r.Next()
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("image layer has no %s", file)
		}
		if err != nil {
			return "", fmt.Errorf("image layer: %w", err)
		}
		if strings.TrimPrefix(header.Name, "./") != file {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return "", fmt.Errorf("%s in the image layer is not a regular file", file)
		}
		sum := sha256.New()
		n, err := io.Copy(sum, io.LimitReader(r, maxImageFileBytes+1))
		if err != nil {
			return "", fmt.Errorf("image layer: %w", err)
		}
		if n > maxImageFileBytes {
			return "", fmt.Errorf("%s exceeds %d bytes", file, maxImageFileBytes)
		}
		return hex.EncodeToString(sum.Sum(nil)), nil
	}
}
