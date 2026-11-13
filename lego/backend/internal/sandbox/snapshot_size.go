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

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// SnapshotSizer reads the stored size of a suspended sandbox's retained image
// for the storage meter (ADR023 § Sandbox snapshot storage).
type SnapshotSizer interface {
	// SnapshotBytes returns the compressed bytes of the newest image the
	// registry holds for the sandbox whose substrate id is ref.
	SnapshotBytes(ctx context.Context, workspaceID, ref string) (int64, error)
}

// Defaults of the OpenSandbox controller's snapshot repository layout.
const (
	DefaultSnapshotRepoPrefix = "snapshots"
	DefaultSnapshotContainer  = "sandbox"
)

// RegistryConfig reaches the registry that holds suspended sandboxes' images.
// The repository is `<Prefix>/<ws>-sandbox/<ref>-<Container>` and the tag
// `snap-gen<N>`, as the pinned OpenSandbox controller writes them
// (deploy/opensandbox/patches/controller-snapshot-job-namespace.patch); the
// newest N is the image a resume would pull. The credential is scoped to
// `<Prefix>/**`.
type RegistryConfig struct {
	BaseURL   string // e.g. http://zot.bex-registry.svc:5000
	Prefix    string // the controller's --snapshot-registry path; default DefaultSnapshotRepoPrefix
	Container string // the sandbox container's name in the pod; default DefaultSnapshotContainer
	User      string
	Password  string
	// Credentials, when set, supplies the user and password for each call in
	// place of User and Password, so a credential rotated in a Secret is picked up
	// without a restart.
	Credentials func(context.Context) (user, password string, err error)
	HTTP        *http.Client
}

// RegistrySizer sizes a snapshot from its OCI image manifest in the registry:
// config.size plus the sum of layers[].size, the compressed blob bytes the
// registry stores. The OpenSandbox SandboxSnapshot status carries the image URI
// and digest and no size, and Zot's search extension (which would report one)
// is not enabled by the operator's config, so the distribution API is the
// source: it needs no change to either. It reads through a read-only credential.
type RegistrySizer struct {
	RegistryConfig
}

const (
	manifestAccept = "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"
	// A manifest or tag list is a few KiB; anything past this is not one.
	maxRegistryBody = 1 << 20
)

// snapshotTag is the controller's tag for a sandbox-owned snapshot.
var snapshotTag = regexp.MustCompile(`^snap-gen([0-9]{1,18})$`)

// registryRef limits the substrate id interpolated into a repository path.
var registryRef = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]{0,200}[a-z0-9])?$`)

func (r *RegistryConfig) client() *http.Client {
	if r.HTTP != nil {
		return r.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// namespacePath is the workspace's snapshot namespace in the registry, under
// which its suspended sandboxes' and imported images' repositories live.
func (r *RegistryConfig) namespacePath(workspaceID string) string {
	prefix := r.Prefix
	if prefix == "" {
		prefix = DefaultSnapshotRepoPrefix
	}
	return prefix + "/" + store.SandboxNamespace(workspaceID)
}

func (r *RegistryConfig) repository(workspaceID, ref string) (string, error) {
	if !registryRef.MatchString(ref) {
		return "", fmt.Errorf("sandbox storage: %q is not a registry-safe sandbox id", ref)
	}
	container := r.Container
	if container == "" {
		container = DefaultSnapshotContainer
	}
	return r.namespacePath(workspaceID) + "/" + ref + "-" + container, nil
}

// credentials is the user and password a registry call sends: the Secret's
// when Credentials is set, else User and Password.
func (r *RegistryConfig) credentials(ctx context.Context) (string, string, error) {
	if r.Credentials == nil {
		return r.User, r.Password, nil
	}
	user, password, err := r.Credentials(ctx)
	if err != nil {
		return "", "", fmt.Errorf("registry credentials: %w", err)
	}
	return user, password, nil
}

// registryError is a registry answer that was not a success.
type registryError struct {
	path   string
	status int
}

func (e *registryError) Error() string {
	return fmt.Sprintf("registry %s: status %d", e.path, e.status)
}

// request sends one registry call and returns the body of a 2xx answer; any
// other status is a *registryError.
func (r *RegistryConfig) request(ctx context.Context, method, path, accept string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(r.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	user, password, err := r.credentials(ctx)
	if err != nil {
		return nil, nil, err
	}
	if user != "" {
		req.SetBasicAuth(user, password)
	}
	resp, err := r.client().Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("registry %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRegistryBody+1))
	if err != nil {
		return nil, nil, fmt.Errorf("registry %s: %w", path, err)
	}
	if len(body) > maxRegistryBody {
		return nil, nil, fmt.Errorf("registry %s: response too large", path)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil, &registryError{path: path, status: resp.StatusCode}
	}
	return body, resp.Header, nil
}

func (r *RegistryConfig) get(ctx context.Context, path, accept string) ([]byte, http.Header, error) {
	return r.request(ctx, http.MethodGet, path, accept)
}

// listTags returns a repository's tags; a repository the registry does not have
// is a *registryError with status 404.
func (r *RegistryConfig) listTags(ctx context.Context, escapedRepo, repo string) ([]string, error) {
	body, _, err := r.get(ctx, "/v2/"+escapedRepo+"/tags/list", "")
	if err != nil {
		return nil, err
	}
	var tags struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("registry tags of %s: bad response", repo)
	}
	return tags.Tags, nil
}

// SnapshotBytes implements SnapshotSizer.
func (r *RegistrySizer) SnapshotBytes(ctx context.Context, workspaceID, ref string) (int64, error) {
	repo, err := r.repository(workspaceID, ref)
	if err != nil {
		return 0, err
	}
	escaped := escapeRepository(repo)
	tags, err := r.listTags(ctx, escaped, repo)
	if err != nil {
		return 0, err
	}
	tag, ok := newestSnapshotTag(tags)
	if !ok {
		return 0, fmt.Errorf("registry: no snapshot image in %s yet", repo)
	}
	body, _, err := r.get(ctx, "/v2/"+escaped+"/manifests/"+url.PathEscape(tag), manifestAccept)
	if err != nil {
		return 0, err
	}
	size, err := manifestBytes(body)
	if err != nil {
		return 0, fmt.Errorf("registry manifest %s:%s: %w", repo, tag, err)
	}
	return size, nil
}

func escapeRepository(repo string) string {
	parts := strings.Split(repo, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// newestSnapshotTag picks the highest snap-gen<N>.
func newestSnapshotTag(tags []string) (string, bool) {
	best, bestN, found := "", int64(-1), false
	for _, t := range tags {
		m := snapshotTag.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			continue
		}
		if n > bestN {
			best, bestN, found = t, n, true
		}
	}
	return best, found
}

// manifestBytes is config.size + sum(layers[].size) of an image manifest. An
// index (a multi-platform image) is refused: the commit job pushes one image.
func manifestBytes(body []byte) (int64, error) {
	var m struct {
		MediaType string `json:"mediaType"`
		Config    struct {
			Size int64 `json:"size"`
		} `json:"config"`
		Layers []struct {
			Size int64 `json:"size"`
		} `json:"layers"`
		Manifests []json.RawMessage `json:"manifests"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return 0, fmt.Errorf("bad manifest")
	}
	if len(m.Manifests) > 0 || len(m.Layers) == 0 {
		return 0, fmt.Errorf("not a single-image manifest")
	}
	total := m.Config.Size
	if total < 0 {
		return 0, fmt.Errorf("negative blob size")
	}
	for _, l := range m.Layers {
		if l.Size < 0 || total > store.MaxStorageObjectBytes-l.Size {
			return 0, fmt.Errorf("blob sizes out of range")
		}
		total += l.Size
	}
	return total, nil
}

// ImageReaper deletes the images of a sandbox that is gone from the registry.
type ImageReaper interface {
	// DeleteImages removes every snapshot image the registry holds for the
	// sandbox whose substrate id is ref. A repository that does not exist is
	// success.
	DeleteImages(ctx context.Context, workspaceID, ref string) error
}

// RegistryReaper deletes a gone sandbox's images through the distribution API:
// each of its `snap-gen<N>` tags, by tag, never by digest (a digest delete
// removes every tag that points at the manifest, including one the controller
// did not write). Zot then sees untagged manifests and its retention
// (`deleteUntagged`) and blob garbage collection reclaim them after their delay.
// Zot's retention keeps a repository's five newest tags forever, so a
// terminated sandbox's images would otherwise stay. It needs a credential with
// the delete action on `<Prefix>/**`, separate from the sizer's read-only one.
// Checked against a local Zot v2.1.22 on 2026-11-10: a tag delete answers 202,
// removes that tag only, and an emptied repository then answers 404.
type RegistryReaper struct {
	RegistryConfig
}

// DeleteImages implements ImageReaper.
func (r *RegistryReaper) DeleteImages(ctx context.Context, workspaceID, ref string) error {
	repo, err := r.repository(workspaceID, ref)
	if err != nil {
		return err
	}
	escaped := escapeRepository(repo)
	tags, err := r.listTags(ctx, escaped, repo)
	var re *registryError
	if errors.As(err, &re) && re.status == http.StatusNotFound {
		return nil // no repository: nothing to delete
	}
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if !snapshotTag.MatchString(tag) {
			continue // never delete what the controller did not write
		}
		if err := r.deleteTag(ctx, escaped, tag); err != nil {
			return err
		}
	}
	return nil
}

// DeleteImportedImage deletes an imported image (w2/m358) once its group is
// deleted (w2/m359): the one tag its repository holds. An image already gone
// is not an error.
func (r *RegistryReaper) DeleteImportedImage(ctx context.Context, workspaceID, buildID string) error {
	if err := validateBuildID(buildID); err != nil {
		return err
	}
	return r.deleteTag(ctx, escapeRepository(r.importedRepository(workspaceID, buildID)), imageTag)
}

// importedRepository is an imported image's repository path in the registry.
func (r *RegistryConfig) importedRepository(workspaceID, buildID string) string {
	return r.namespacePath(workspaceID) + imagesSegment + buildID
}

// deleteTag deletes one tag of an escaped repository; a tag already gone is
// not an error.
func (r *RegistryReaper) deleteTag(ctx context.Context, escapedRepo, tag string) error {
	_, _, err := r.request(ctx, http.MethodDelete, "/v2/"+escapedRepo+"/manifests/"+url.PathEscape(tag), "")
	var re *registryError
	if errors.As(err, &re) && re.status == http.StatusNotFound {
		return nil
	}
	return err
}
