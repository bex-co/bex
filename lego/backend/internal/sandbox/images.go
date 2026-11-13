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
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/netutil"
)

// Imported sandbox images (w2/m358, docs/ADR104-sandbox-render-parity.md
// § Images): Render's sandbox builds. An import resolves the caller's OCI
// reference to a digest while the request waits, so the build Render returns
// always names its digest, then copies the linux/amd64 image into the group's
// repository in the sandbox registry in the background. A sandbox create that
// names the build (buildId) or its name (buildName) starts from that copy.

// Codes. Render's errorCode vocabulary names some of these refusals; the rest
// are bex's, because Render documents a message only.
const (
	// CodeBuildNotFound is Render's `build_not_found`: no such build in the group.
	CodeBuildNotFound = "build_not_found"
	// CodeInvalidBuildStatus is Render's `invalid_status` on the build list.
	CodeInvalidBuildStatus = "invalid_status"
	// CodeSandboxImageInvalid refuses a name or source image Render's create
	// would refuse with a message.
	CodeSandboxImageInvalid = "SANDBOX_IMAGE_INVALID"
	// CodeSandboxImageUnavailable refuses a source the registry cannot serve.
	CodeSandboxImageUnavailable = "SANDBOX_IMAGE_UNAVAILABLE"
	// CodeSandboxBuildLimit is the 429 for a group with its limit of imports
	// pending or running.
	CodeSandboxBuildLimit = "SANDBOX_BUILD_LIMIT"
	// CodeSandboxImageNotReady refuses a create from a build that has not
	// succeeded.
	CodeSandboxImageNotReady = "SANDBOX_IMAGE_NOT_READY"
	// CodeSandboxImagesUnavailable is the 503 of a deployment with no sandbox
	// registry or store to keep imported images in.
	CodeSandboxImagesUnavailable = "SANDBOX_IMAGES_UNAVAILABLE"
)

const (
	maxImageName = 255
	// imageTag is the one tag a build's repository holds; registry retention
	// reaps untagged manifests, so the copy needs one.
	imageTag = "image"
	// importTimeout bounds one copy.
	importTimeout = 30 * time.Minute
	// importHeartbeat is how often a running import says it is alive, and
	// importStaleAfter how long one may stay silent before the sweeper fails it.
	importHeartbeat  = 30 * time.Second
	importStaleAfter = 5 * time.Minute
	importSweepEvery = time.Minute
	// defaultImportsInFlight is how many imports of one group may be pending or
	// running at once (Render's limit is unpublished; its 429 names the limit).
	defaultImportsInFlight = 5
	// defaultMaxImageBytes caps an image's compressed size (10 GB, a quarter of
	// a sandbox's 40 GB filesystem).
	defaultMaxImageBytes = 10 * store.BytesPerDecimalGB
	// imagesSegment is the path under a workspace's snapshot namespace that
	// holds its imported images, one repository per build.
	imagesSegment = "/images/"
	// importWorkers is how many copies one bex-api replica runs at once.
	importWorkers = 2
)

var linuxAMD64 = v1.Platform{OS: "linux", Architecture: "amd64"}

var buildStatuses = []string{store.SandboxBuildPending, store.SandboxBuildRunning, store.SandboxBuildSucceeded, store.SandboxBuildFailed}

// Build is Render's sandbox build, the shape of the pinned OpenAPI schema.
type Build struct {
	ID             string      `json:"id"`
	SandboxGroupID string      `json:"sandboxGroupId"`
	Name           string      `json:"name"`
	Source         BuildSource `json:"source"`
	SourceDigest   string      `json:"sourceDigest"`
	Status         string      `json:"status"`
	FailureReason  *string     `json:"failureReason"`
	CreatedAt      time.Time   `json:"createdAt"`
	FinishedAt     *time.Time  `json:"finishedAt"`
}

// BuildSource is the image a build converts: the registry, the image path in
// it, and the tag or digest the caller named.
type BuildSource struct {
	Repository string `json:"repository"`
	Image      string `json:"image"`
	Reference  string `json:"reference"`
}

// BuildEnvelope wraps a Build for Render's cursor-paginated list.
type BuildEnvelope struct {
	Build  Build  `json:"build"`
	Cursor string `json:"cursor"`
}

// RegistryCredential pulls a private source image. It is used for one import
// and never stored or logged.
type RegistryCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ImportImageRequest is the body of Render's create-sandbox-build.
type ImportImageRequest struct {
	Name               string              `json:"name"`
	SourceImage        string              `json:"sourceImage"`
	RegistryCredential *RegistryCredential `json:"registryCredential,omitempty"`
}

// ImageStore is where builds live; *store.PGStore implements it.
type ImageStore interface {
	CreateSandboxBuild(ctx context.Context, in store.NewSandboxBuild, maxInFlight int) (store.SandboxBuild, bool, error)
	SandboxBuild(ctx context.Context, workspaceID, groupID, id string) (store.SandboxBuild, error)
	SandboxBuilds(ctx context.Context, workspaceID, groupID, name string, statuses []string) ([]store.SandboxBuild, error)
	NewestSucceededSandboxBuild(ctx context.Context, workspaceID, groupID, name string) (store.SandboxBuild, error)
	StartSandboxBuild(ctx context.Context, id string) (bool, error)
	TouchSandboxBuild(ctx context.Context, id string) (bool, error)
	SucceedSandboxBuild(ctx context.Context, id, imageDigest string, sizeBytes int64) error
	FailSandboxBuild(ctx context.Context, id, reason string) error
	FailStaleSandboxBuilds(ctx context.Context, staleAfter time.Duration, reason string) (int64, error)
}

// Images imports sandbox images into the sandbox registry. nil on the Service
// answers every image verb 503.
type Images struct {
	Store ImageStore
	// Registry is the sandbox registry and the importer's credential, which may
	// write a workspace's snapshot repositories.
	Registry RegistryConfig
	// MaxBytes caps an image's compressed size (0: defaultMaxImageBytes).
	MaxBytes int64
	// MaxInFlight caps a group's pending and running imports (0:
	// defaultImportsInFlight).
	MaxInFlight int
	// SourceTransport reaches source registries; nil is the SSRF-guarded default
	// (no private, loopback, link-local or metadata destination, no ambient
	// proxy). Tests override it.
	SourceTransport http.RoundTripper

	workersOnce sync.Once
	workers     chan struct{}
}

// slots bounds the copies one replica runs at once.
func (im *Images) slots() chan struct{} {
	im.workersOnce.Do(func() { im.workers = make(chan struct{}, importWorkers) })
	return im.workers
}

func (im *Images) maxBytes() int64 {
	if im.MaxBytes > 0 {
		return im.MaxBytes
	}
	return defaultMaxImageBytes
}

func (im *Images) maxInFlight() int {
	if im.MaxInFlight > 0 {
		return im.MaxInFlight
	}
	return defaultImportsInFlight
}

// guardedSourceTransport is the default source transport: every dial, a
// redirect's and a token realm's included, refuses a private destination, so a
// source reference cannot probe the cluster or cloud metadata.
var guardedSourceTransport = func() http.RoundTripper {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = netutil.SafeDialContext(30 * time.Second)
	tr.Proxy = nil // an ambient proxy would dial in the guard's place
	return tr
}()

func (im *Images) sourceTransport() http.RoundTripper {
	if im.SourceTransport != nil {
		return im.SourceTransport
	}
	return guardedSourceTransport
}

// registryHost is the sandbox registry's host and whether it speaks plain HTTP.
func (im *Images) registryHost() (string, bool, error) {
	u, err := url.Parse(im.Registry.BaseURL)
	if err != nil || u.Host == "" {
		return "", false, fmt.Errorf("sandbox registry URL %q has no host", im.Registry.BaseURL)
	}
	return u.Host, u.Scheme == "http", nil
}

// imageRepository is a build's repository in the sandbox registry, inside the
// workspace's snapshot namespace so its pods pull it with the namespace's own
// read-only credential.
func (im *Images) imageRepository(workspace, buildID string) (name.Repository, error) {
	host, insecure, err := im.registryHost()
	if err != nil {
		return name.Repository{}, err
	}
	var opts []name.Option
	if insecure {
		opts = append(opts, name.Insecure)
	}
	return name.NewRepository(host+"/"+im.Registry.namespacePath(workspace)+imagesSegment+buildID, opts...)
}

// requireImages is the 503 of a deployment that cannot keep imported images.
func (s *Service) requireImages() error {
	if s.Images == nil || s.Groups == nil {
		return core.NewUnavailableError(CodeSandboxImagesUnavailable,
			"sandbox images are not available on this deployment: they need the control-plane store and the sandbox registry", nil)
	}
	return nil
}

func validateImageName(n string) error {
	if labelOK(n, maxImageName) {
		return nil
	}
	return core.NewBadRequestError(CodeSandboxImageInvalid,
		fmt.Sprintf("name must be 1 to %d characters, with no leading or trailing space and no control characters", maxImageName),
		map[string]any{"field": "name", "maxLength": maxImageName})
}

func validateBuildID(id string) error {
	if kind, ok := ids.KindOf(id); ok && kind == ids.SandboxBuild {
		return nil
	}
	return core.NewBadRequestError(CodeSandboxImageInvalid,
		fmt.Sprintf("invalid build ID %q; they look like %s-…", id, ids.SandboxBuild.Prefix()),
		map[string]any{"field": "buildId"})
}

func buildNotFound(id string) error {
	return core.NewNotFoundError(CodeBuildNotFound, "sandbox image not found in this sandbox group", map[string]any{"id": id})
}

func buildView(b store.SandboxBuild) Build {
	return Build{
		ID: b.ID, SandboxGroupID: b.SandboxGroupID, Name: b.Name,
		Source:       BuildSource{Repository: b.SourceRepository, Image: b.SourceImage, Reference: b.SourceReference},
		SourceDigest: b.SourceDigest, Status: b.Status, FailureReason: b.FailureReason,
		CreatedAt: b.CreatedAt, FinishedAt: b.FinishedAt,
	}
}

// imageGroup scopes a group verb, authorizes rel there and loads the group.
func (s *Service) imageGroup(ctx context.Context, groupID, rel string) (context.Context, store.SandboxGroup, error) {
	ctx, err := s.scopeGroup(ctx, groupID)
	if err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	if err := s.Authorize(ctx, rel); err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	if err := s.requireImages(); err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	g, err := s.storedGroup(ctx, groupID)
	return ctx, g, err
}

// imageBytes is an image's compressed size from its manifest, refusing the
// negative or overflowing sizes a hostile manifest could carry.
func imageBytes(img v1.Image) (int64, error) {
	raw, err := img.RawManifest()
	if err != nil {
		return 0, err
	}
	return manifestBytes(raw)
}

// resolvedSource is a source reference resolved to the digest an import
// copies, pinned so a tag that moves during the copy does not change it.
type resolvedSource struct {
	source BuildSource
	ref    name.Digest // repository@digest (an index's digest, when it is one)
}

// resolveSource resolves ref to its digest and checks the image it names can
// run in a sandbox: linux/amd64 and within the size cap.
func (im *Images) resolveSource(ctx context.Context, ref string, cred *RegistryCredential) (resolvedSource, error) {
	parsed, err := name.ParseReference(strings.TrimSpace(ref))
	if err != nil {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("sourceImage %q is not an image reference: %v", ref, err), map[string]any{"field": "sourceImage"})
	}
	unavailable := func(err error) error {
		return core.NewBadRequestError(CodeSandboxImageUnavailable,
			fmt.Sprintf("the registry could not serve %s: %v", parsed, err), map[string]any{"field": "sourceImage"})
	}
	opts := im.sourceOptions(ctx, cred)
	desc, err := remote.Get(parsed, opts...)
	if err != nil {
		return resolvedSource{}, unavailable(err)
	}
	img, err := desc.Image() // an index picks its linux/amd64 child (WithPlatform)
	if err != nil {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("%s has no linux/amd64 image, the platform sandboxes run: %v", parsed, err), map[string]any{"field": "sourceImage"})
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return resolvedSource{}, unavailable(err)
	}
	if cfg.OS != linuxAMD64.OS || cfg.Architecture != linuxAMD64.Architecture {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("%s is a %s/%s image; sandboxes run linux/amd64 images", parsed, cfg.OS, cfg.Architecture),
			map[string]any{"field": "sourceImage", "platform": cfg.OS + "/" + cfg.Architecture})
	}
	size, err := imageBytes(img)
	if err != nil {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("%s has an unreadable manifest: %v", parsed, err), map[string]any{"field": "sourceImage"})
	}
	if size > im.maxBytes() {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("%s is %s compressed; sandbox images are limited to %s", parsed, formatGB(size), formatGB(im.maxBytes())),
			map[string]any{"field": "sourceImage", "sizeBytes": size, "limitBytes": im.maxBytes()})
	}
	registry := parsed.Context().RegistryStr()
	if registry == name.DefaultRegistry {
		registry = "docker.io" // the name people write; the API host is index.docker.io
	}
	return resolvedSource{
		source: BuildSource{Repository: registry, Image: parsed.Context().RepositoryStr(), Reference: parsed.Identifier()},
		ref:    parsed.Context().Digest(desc.Digest.String()),
	}, nil
}

func (im *Images) sourceOptions(ctx context.Context, cred *RegistryCredential) []remote.Option {
	auth := authn.Anonymous
	if cred != nil && (cred.Username != "" || cred.Password != "") {
		auth = authn.FromConfig(authn.AuthConfig{Username: cred.Username, Password: cred.Password})
	}
	return []remote.Option{
		remote.WithContext(ctx), remote.WithAuth(auth), remote.WithPlatform(linuxAMD64),
		remote.WithTransport(im.sourceTransport()),
	}
}

// ImportImage starts importing an image into a group and returns its build. A
// name and digest the group already has, or is importing, returns that build.
func (s *Service) ImportImage(ctx context.Context, groupID string, req ImportImageRequest) (Build, error) {
	ctx, group, err := s.imageGroup(ctx, groupID, core.RelCanCreate)
	if err != nil {
		return Build{}, err
	}
	if err := validateImageName(req.Name); err != nil {
		return Build{}, err
	}
	if strings.TrimSpace(req.SourceImage) == "" {
		return Build{}, core.NewBadRequestError(CodeSandboxImageInvalid, "sourceImage is required", map[string]any{"field": "sourceImage"})
	}
	if used, full := s.storageFull(ctx, group.WorkspaceID); full {
		s.Metrics.observeRefusal("storage", "import")
		return Build{}, core.NewTooManyRequestsError(CodeSandboxStorageLimit,
			fmt.Sprintf("this workspace's sandbox images and suspended sandboxes already hold %s of its %s sandbox storage limit; terminate sandboxes or delete sandbox groups you no longer need, then import again",
				formatGB(used), formatGB(s.MaxStorageBytes)),
			map[string]any{"limitBytes": s.MaxStorageBytes, "usedBytes": used})
	}
	src, err := s.Images.resolveSource(ctx, req.SourceImage, req.RegistryCredential)
	if err != nil {
		return Build{}, err
	}
	b, created, err := s.Images.Store.CreateSandboxBuild(ctx, store.NewSandboxBuild{
		WorkspaceID: group.WorkspaceID, SandboxGroupID: group.ID, Name: req.Name,
		SourceRepository: src.source.Repository, SourceImage: src.source.Image, SourceReference: src.source.Reference,
		SourceDigest: src.ref.DigestStr(),
	}, s.Images.maxInFlight())
	switch {
	case errors.Is(err, store.ErrSandboxBuildLimit):
		return Build{}, core.NewTooManyRequestsError(CodeSandboxBuildLimit,
			fmt.Sprintf("this sandbox group already has %d images importing, its limit; wait for one to finish, then import again", s.Images.maxInFlight()),
			map[string]any{"limit": s.Images.maxInFlight()})
	case errors.Is(err, store.ErrNotFound):
		return Build{}, groupNotFound(groupID)
	case err != nil:
		return Build{}, store.MapError(err)
	}
	if created {
		// The request's context ends with the response, so the copy fetches the
		// pinned digest again on its own; the credential travels in memory only.
		go s.Images.run(b, src.ref, req.RegistryCredential)
	}
	return buildView(b), nil
}

// GetImage returns one build of a group.
func (s *Service) GetImage(ctx context.Context, groupID, buildID string) (Build, error) {
	ctx, group, err := s.imageGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return Build{}, err
	}
	if err := validateBuildID(buildID); err != nil {
		return Build{}, err
	}
	b, err := s.Images.Store.SandboxBuild(ctx, group.WorkspaceID, group.ID, buildID)
	if errors.Is(err, store.ErrNotFound) {
		return Build{}, buildNotFound(buildID)
	}
	if err != nil {
		return Build{}, store.MapError(err)
	}
	return buildView(b), nil
}

// ListImages lists a group's builds newest first, filtered by name and status,
// one page after the cursor.
func (s *Service) ListImages(ctx context.Context, groupID, imageName string, statuses []string, after string, limit int) ([]Build, error) {
	ctx, group, err := s.imageGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return nil, err
	}
	for _, st := range statuses {
		if !slices.Contains(buildStatuses, st) {
			return nil, core.NewBadRequestError(CodeInvalidBuildStatus,
				fmt.Sprintf("%q is not a build status; use %s", st, strings.Join(buildStatuses, ", ")),
				map[string]any{"field": "status", "supported": buildStatuses})
		}
	}
	rows, err := s.Images.Store.SandboxBuilds(ctx, group.WorkspaceID, group.ID, imageName, statuses)
	if err != nil {
		return nil, store.MapError(err)
	}
	out := make([]Build, 0, len(rows))
	for _, b := range rows {
		out = append(out, buildView(b))
	}
	return core.Page(out, after, core.PageLimitOrDefault(limit), func(b Build) string { return b.ID }), nil
}

// createImage resolves a create's buildId or buildName, in the group the
// sandbox is created in, to the stored image the sandbox starts from. A name is
// the group's newest succeeded build of it, as on Render.
func (s *Service) createImage(ctx context.Context, workspace, groupID, buildID, buildName string) (string, error) {
	if buildID == "" && buildName == "" {
		return "", nil
	}
	if err := s.requireImages(); err != nil {
		return "", err
	}
	if buildID != "" && buildName != "" {
		return "", core.NewBadRequestError(CodeSandboxImageInvalid, "name an image by buildId or by buildName, not both",
			map[string]any{"fields": []string{"buildId", "buildName"}})
	}
	group, err := s.resolveGroup(ctx, workspace, groupID)
	if err != nil {
		return "", err
	}
	var b store.SandboxBuild
	if buildID != "" {
		if err := validateBuildID(buildID); err != nil {
			return "", err
		}
		b, err = s.Images.Store.SandboxBuild(ctx, workspace, group.ID, buildID)
		if errors.Is(err, store.ErrNotFound) {
			return "", buildNotFound(buildID)
		}
	} else {
		b, err = s.Images.Store.NewestSucceededSandboxBuild(ctx, workspace, group.ID, buildName)
		if errors.Is(err, store.ErrNotFound) {
			return "", core.NewNotFoundError(CodeBuildNotFound,
				fmt.Sprintf("sandbox group %s has no image named %q that has finished importing", group.ID, buildName),
				map[string]any{"buildName": buildName, "sandboxGroupId": group.ID})
		}
	}
	if err != nil {
		return "", store.MapError(err)
	}
	if b.Status != store.SandboxBuildSucceeded || b.ImageDigest == nil {
		return "", core.NewBadRequestError(CodeSandboxImageNotReady,
			fmt.Sprintf("image %s is %s; a sandbox can start from it once it has succeeded", b.ID, b.Status),
			map[string]any{"buildId": b.ID, "status": b.Status})
	}
	repo, err := s.Images.imageRepository(workspace, b.ID)
	if err != nil {
		return "", err
	}
	return repo.Digest(*b.ImageDigest).String(), nil
}

// importedImage matches the image a sandbox started from an import runs, so a
// read and a charge show the build's id instead of the sandbox registry's
// internal address.
var importedImage = regexp.MustCompile(regexp.QuoteMeta(imagesSegment) + `(sbb-[0-9a-v]{20})@sha256:[0-9a-f]{64}$`)

// displayImage is the image a sandbox reports: an import's build id, or the
// template image as it was started.
func displayImage(uri string) string {
	if m := importedImage.FindStringSubmatch(uri); m != nil {
		return m[1]
	}
	return uri
}

// run copies one build's image into the sandbox registry and settles it. A
// burst of imports queues for a worker slot as pending; the heartbeat keeps a
// queued or running build fresh for the sweeper, and stops the import when the
// sweeper (or anything else) has settled the build already.
func (im *Images) run(b store.SandboxBuild, ref name.Digest, cred *RegistryCredential) {
	alive, stop := context.WithCancel(context.Background())
	defer stop()
	go im.heartbeat(alive, stop, b.ID)
	select {
	case im.slots() <- struct{}{}:
		defer func() { <-im.slots() }()
	case <-alive.Done():
		return
	}
	ctx, cancel := context.WithTimeout(alive, importTimeout)
	defer cancel()
	if started, err := im.Store.StartSandboxBuild(ctx, b.ID); err != nil || !started {
		if err != nil {
			log.Printf("sandbox images: start %s: %v", b.ID, err)
		}
		return
	}
	digest, size, err := im.copy(ctx, b, ref, cred)
	// The settle must land even when the copy used up the import's deadline.
	settle, done := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer done()
	if err != nil {
		reason := "the image could not be copied: " + err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			reason = fmt.Sprintf("the import did not finish within %s", importTimeout)
		}
		if ferr := im.Store.FailSandboxBuild(settle, b.ID, reason); ferr != nil {
			log.Printf("sandbox images: fail %s: %v", b.ID, ferr)
		}
		return
	}
	if err := im.Store.SucceedSandboxBuild(settle, b.ID, digest, size); err != nil {
		log.Printf("sandbox images: succeed %s: %v", b.ID, err)
	}
}

// heartbeat advances the build's updated_at until ctx ends, and calls stop
// once the build is no longer pending or running.
func (im *Images) heartbeat(ctx context.Context, stop context.CancelFunc, id string) {
	t := time.NewTicker(importHeartbeat)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			live, err := im.Store.TouchSandboxBuild(ctx, id)
			if err != nil {
				log.Printf("sandbox images: heartbeat %s: %v", id, err)
			} else if !live {
				stop()
				return
			}
		}
	}
}

// copy writes the linux/amd64 image at ref to the build's repository under
// imageTag and returns the stored manifest's digest and its compressed size.
func (im *Images) copy(ctx context.Context, b store.SandboxBuild, ref name.Digest, cred *RegistryCredential) (string, int64, error) {
	repo, err := im.imageRepository(b.WorkspaceID, b.ID)
	if err != nil {
		return "", 0, err
	}
	desc, err := remote.Get(ref, im.sourceOptions(ctx, cred)...)
	if err != nil {
		return "", 0, err
	}
	img, err := desc.Image()
	if err != nil {
		return "", 0, err
	}
	user, password, err := im.Registry.credentials(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("the sandbox registry credential is unavailable: %w", err)
	}
	opts := []remote.Option{remote.WithContext(ctx), remote.WithAuth(authn.FromConfig(authn.AuthConfig{Username: user, Password: password}))}
	if im.Registry.HTTP != nil && im.Registry.HTTP.Transport != nil {
		opts = append(opts, remote.WithTransport(im.Registry.HTTP.Transport))
	}
	if err := remote.Write(repo.Tag(imageTag), img, opts...); err != nil {
		return "", 0, err
	}
	digest, err := img.Digest()
	if err != nil {
		return "", 0, err
	}
	size, err := imageBytes(img)
	return digest.String(), size, err
}

// RunImageSweeper fails, every minute, the imports whose bex-api replica
// stopped advancing them: the credential a private one used is gone with it,
// so the caller imports again.
func (s *Service) RunImageSweeper(ctx context.Context) {
	if s.Images == nil {
		return
	}
	core.Poll(ctx, "sandbox-images", importSweepEvery, func(ctx context.Context) error {
		n, err := s.Images.Store.FailStaleSandboxBuilds(ctx, importStaleAfter,
			"the import was interrupted; import the image again")
		if n > 0 {
			log.Printf("sandbox images: failed %d interrupted imports", n)
		}
		return err
	})
}

// NewImages returns an importer writing to registry.
func NewImages(st ImageStore, registry RegistryConfig) *Images {
	return &Images{Store: st, Registry: registry}
}
