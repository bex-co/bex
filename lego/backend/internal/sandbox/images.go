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
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

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
	// CodeImageInUse is Render's `image_in_use`: sandboxes still depend on the
	// build a delete names.
	CodeImageInUse = "image_in_use"
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
	// CodeSandboxImportCapacity refuses an import that would take every
	// workspace's imported images together past the platform budget (w5/m383).
	CodeSandboxImportCapacity = "SANDBOX_IMPORT_CAPACITY"
	// CodeBuildNotSucceeded is Render's 409 for a create from a build, or a
	// name, with no succeeded import.
	CodeBuildNotSucceeded = "build_not_succeeded"
	// CodeSandboxImagesUnavailable is the 503 of a deployment with no sandbox
	// registry or store to keep imported images in.
	CodeSandboxImagesUnavailable = "SANDBOX_IMAGES_UNAVAILABLE"
)

const (
	// maxImageDependents is how many dependents an image_in_use names, as Render.
	maxImageDependents = 10
	maxImageName       = 255
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

// BuildStatuses is the build list's status filter vocabulary, for refusals
// the REST gate answers before the Service (w8/312).
func BuildStatuses() []string { return slices.Clone(buildStatuses) }

// Build is Render's sandbox build, the shape of the pinned OpenAPI schema.
type Build struct {
	ID             string      `json:"id"`
	SandboxGroupID string      `json:"sandboxGroupId"`
	Name           string      `json:"name"`
	Source         BuildSource `json:"source"`
	SourceDigest   string      `json:"sourceDigest"`
	Status         string      `json:"status"`
	FailureReason  *string     `json:"failureReason"`
	// FailureCode is Render's machine-readable failureCode: null unless
	// failed (w3/m367).
	FailureCode *string    `json:"failureCode"`
	CreatedAt   time.Time  `json:"createdAt"`
	FinishedAt  *time.Time `json:"finishedAt"`
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
	SucceedSandboxBuild(ctx context.Context, id, imageDigest string, sizeBytes int64) (bool, error)
	FailSandboxBuild(ctx context.Context, id, code, reason string) error
	DeleteSandboxBuild(ctx context.Context, workspaceID, groupID, id string) (bool, error)
	RestoreSandboxBuild(ctx context.Context, workspaceID, groupID, id string) error
	FailStaleSandboxBuilds(ctx context.Context, staleAfter time.Duration, reason string) (int64, error)
	SandboxBuildIDs(ctx context.Context, workspaceID, groupID string) ([]string, error)
	FailGroupSandboxBuilds(ctx context.Context, workspaceID, groupID, reason string) error
}

// ImageDeleter deletes an imported image, or a sandbox's snapshot images,
// from the registry; *RegistryReaper implements it.
type ImageDeleter interface {
	DeleteImportedImage(ctx context.Context, workspaceID, buildID string) error
	DeleteImages(ctx context.Context, workspaceID, ref string) error
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
	// PlatformBudgetBytes bounds the imported bytes of every workspace
	// together, below the shared registry's volume
	// (BEX_SANDBOX_IMPORT_PLATFORM_BUDGET_GB; 0: none, w5/m383).
	PlatformBudgetBytes int64
	// WorkspaceShareBytes bounds one workspace's imported bytes within that
	// budget (0: a quarter of it), so one workspace cannot hold the whole
	// budget against everyone else.
	WorkspaceShareBytes int64
	// MaxInFlight caps a group's pending and running imports (0:
	// defaultImportsInFlight).
	MaxInFlight int
	// SourceTransport reaches source registries; nil is the SSRF-guarded default
	// (no private, loopback, link-local or metadata destination, no ambient
	// proxy). Tests override it.
	SourceTransport http.RoundTripper
	// Reaper deletes a deleted group's images from the registry (w2/m359);
	// nil leaves them, as a deployment without the reaper credential does.
	Reaper ImageDeleter

	workersOnce sync.Once
	workers     chan struct{}
	tenants     workspaceSlots
}

// slots bounds the copies one replica runs at once.
func (im *Images) slots() chan struct{} {
	im.workersOnce.Do(func() { im.workers = make(chan struct{}, importWorkers) })
	return im.workers
}

// workspaceSlots gives each workspace one import slot per replica (w5/m383).
// An import takes its workspace's slot before a shared one, so a workspace
// with a slow source queues behind itself and never holds more than one of
// the replica's shared slots; other workspaces keep the rest.
type workspaceSlots struct {
	mu sync.Mutex
	m  map[string]*workspaceSlot
}

type workspaceSlot struct {
	held  chan struct{}
	users int // imports holding or waiting for it; the entry goes at zero
}

// acquire waits for workspace's slot until ctx is done, and returns its
// release.
func (w *workspaceSlots) acquire(ctx context.Context, workspace string) (func(), bool) {
	w.mu.Lock()
	if w.m == nil {
		w.m = map[string]*workspaceSlot{}
	}
	slot := w.m[workspace]
	if slot == nil {
		slot = &workspaceSlot{held: make(chan struct{}, 1)}
		w.m[workspace] = slot
	}
	slot.users++
	w.mu.Unlock()
	leave := func() {
		w.mu.Lock()
		if slot.users--; slot.users == 0 {
			delete(w.m, workspace)
		}
		w.mu.Unlock()
	}
	select {
	case slot.held <- struct{}{}:
		return func() { <-slot.held; leave() }, true
	case <-ctx.Done():
		leave()
		return nil, false
	}
}

func (im *Images) maxBytes() int64 {
	if im.MaxBytes > 0 {
		return im.MaxBytes
	}
	return defaultMaxImageBytes
}

func (im *Images) workspaceShare() int64 {
	if im.WorkspaceShareBytes > 0 {
		return im.WorkspaceShareBytes
	}
	return im.PlatformBudgetBytes / 4
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

// sameRegistryHost reports whether a source registry is the sandbox
// registry: the same host, ignoring case, on the same port or with either
// port left implicit.
func sameRegistryHost(source, sink string) bool {
	split := func(h string) (host, port string) {
		if hp, p, err := net.SplitHostPort(h); err == nil {
			h, port = hp, p
		}
		return strings.ToLower(strings.TrimSuffix(h, ".")), port
	}
	sourceHost, sourcePort := split(source)
	sinkHost, sinkPort := split(sink)
	return sourceHost == sinkHost && (sourcePort == sinkPort || sourcePort == "" || sinkPort == "")
}

// foreignBlob names the first blob of img the sandbox registry would not hold
// on its own (w5/m383): one with download URLs, which containerd on the
// sandbox node tries first, over the host network; or a non-distributable
// layer, which the write leaves out of the registry while the manifest still
// names it. "" when there is none.
func foreignBlob(img v1.Image) (string, error) {
	manifest, err := img.Manifest()
	if err != nil {
		return "", err
	}
	if len(manifest.Config.URLs) > 0 {
		return manifest.Config.Digest.String(), nil
	}
	for _, l := range manifest.Layers {
		if len(l.URLs) > 0 || !l.MediaType.IsDistributable() {
			return l.Digest.String(), nil
		}
	}
	return "", nil
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
	return im.repository(im.Registry.importedRepository(workspace, buildID))
}

// repository is a repository path's name in the sandbox registry.
func (im *Images) repository(path string) (name.Repository, error) {
	host, insecure, err := im.registryHost()
	if err != nil {
		return name.Repository{}, err
	}
	var opts []name.Option
	if insecure {
		opts = append(opts, name.Insecure)
	}
	return name.NewRepository(host+"/"+path, opts...)
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
		SourceDigest: b.SourceDigest, Status: b.Status, FailureReason: b.FailureReason, FailureCode: failureCode(b),
		CreatedAt: b.CreatedAt, FinishedAt: b.FinishedAt,
	}
}

// failureCode is a failed build's recorded code, or internal_error for one
// that failed before codes were recorded; null for any other status.
func failureCode(b store.SandboxBuild) *string {
	if b.Status != store.SandboxBuildFailed {
		return nil
	}
	if b.FailureCode != nil {
		return b.FailureCode
	}
	code := buildFailedInternal
	return &code
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
	image  v1.Hash     // the linux/amd64 image checked here, which the copy must write
	size   int64       // the linux/amd64 image's compressed size, as its manifest declares
}

// resolveSource resolves ref to its digest and checks the image it names can
// run in a sandbox: linux/amd64 and within the size cap.
func (im *Images) resolveSource(ctx context.Context, ref string, cred *RegistryCredential) (resolvedSource, error) {
	parsed, err := name.ParseReference(strings.TrimSpace(ref))
	if err != nil {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("sourceImage %q is not an image reference: %v", ref, err), map[string]any{"field": "sourceImage"})
	}
	// Never the sandbox registry itself (w5/m383): for a source on the sink's
	// host the write mounts blobs across repositories with the importer's
	// credential. The in-cluster registry is also behind the source dial
	// guard; this names the refusal wherever the registry is reachable.
	// (Zot's dedupe linking a mounted digest across repositories is closed
	// by uploadOnly, w5/192.)
	if sink, _, err := im.registryHost(); err == nil && sameRegistryHost(parsed.Context().RegistryStr(), sink) {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("sourceImage %s is in the sandbox registry itself; import it from the registry it was published to", parsed),
			map[string]any{"field": "sourceImage"})
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
	if err := checkConfigSize(img); err != nil {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("%s: %v", parsed, err), map[string]any{"field": "sourceImage"})
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
	blob, err := foreignBlob(img)
	if err != nil {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("%s has an unreadable manifest: %v", parsed, err), map[string]any{"field": "sourceImage"})
	}
	if blob != "" {
		return resolvedSource{}, core.NewBadRequestError(CodeSandboxImageInvalid,
			fmt.Sprintf("%s has a blob bex would not hold (%s carries download URLs or is non-distributable); sandbox images must hold every layer", parsed, blob),
			map[string]any{"field": "sourceImage", "layer": blob})
	}
	checked, err := img.Digest()
	if err != nil {
		return resolvedSource{}, unavailable(err)
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
		image:  checked,
		size:   size,
	}, nil
}

// Source registries are untrusted, and bex-api reads an image's manifest and
// config into memory (round 24, S-2: a config declared 400 MB, or -1 with a
// chunked body, took a 128 MiB bex-api replica down). A config is a few KiB;
// real manifests are under a few hundred KiB.
const (
	maxSourceManifestBytes = 4 << 20
	maxSourceConfigBytes   = 1 << 20
)

// checkConfigSize refuses an image whose manifest declares its config empty,
// negative or larger than maxSourceConfigBytes, before the config is read: the
// read is bounded by the declared size, so the declaration is what to check.
func checkConfigSize(img v1.Image) error {
	m, err := img.Manifest()
	if err != nil {
		return fmt.Errorf("unreadable manifest: %w", err)
	}
	if size := m.Config.Size; size <= 0 || size > maxSourceConfigBytes {
		return fmt.Errorf("its manifest declares a %d-byte config; a config must be between 1 byte and %d bytes", size, maxSourceConfigBytes)
	}
	return nil
}

// capManifests bounds every manifest response from a source registry at
// maxSourceManifestBytes. Blobs are left alone: layers stream through the copy,
// and the config is bounded by checkConfigSize.
type capManifests struct{ next http.RoundTripper }

func (t capManifests) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(r)
	if err != nil || !isManifestRequest(r) {
		return resp, err
	}
	if resp.ContentLength > maxSourceManifestBytes {
		_ = resp.Body.Close()
		return nil, errManifestTooLarge
	}
	resp.Body = &cappedBody{ReadCloser: resp.Body, left: maxSourceManifestBytes}
	return resp, nil
}

var errManifestTooLarge = fmt.Errorf("the registry's manifest is over the %d-byte limit", maxSourceManifestBytes)

// isManifestRequest reports whether r, or the request a redirect chain
// started from, is a registry /v2/<name>/manifests/<reference> read.
func isManifestRequest(r *http.Request) bool {
	for r.Response != nil && r.Response.Request != nil {
		r = r.Response.Request
	}
	return path.Base(path.Dir(r.URL.Path)) == "manifests"
}

// cappedBody fails a read past its limit instead of truncating it.
type cappedBody struct {
	io.ReadCloser
	left int64
}

func (b *cappedBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		return 0, errManifestTooLarge
	}
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.ReadCloser.Read(p)
	b.left -= int64(n)
	if b.left < 0 {
		return n + int(b.left), errManifestTooLarge
	}
	return n, err
}

func (im *Images) sourceOptions(ctx context.Context, cred *RegistryCredential) []remote.Option {
	auth := authn.Anonymous
	if cred != nil && (cred.Username != "" || cred.Password != "") {
		auth = authn.FromConfig(authn.AuthConfig{Username: cred.Username, Password: cred.Password})
	}
	return []remote.Option{
		remote.WithContext(ctx), remote.WithAuth(auth), remote.WithPlatform(linuxAMD64),
		remote.WithTransport(capManifests{next: im.sourceTransport()}),
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
	// An import stores up to the image size cap in the platform registry, so it
	// takes the billing gates a sandbox create takes (round 24, S-3).
	if err := s.RequireBillingFor(ctx, group.WorkspaceID, true); err != nil {
		return Build{}, err
	}
	src, err := s.Images.resolveSource(ctx, req.SourceImage, req.RegistryCredential)
	if err != nil {
		return Build{}, err
	}
	// The store counts imports still copying at their declared size and checks
	// this one's against the cap under a workspace-wide lock (round 24, S-3).
	b, created, err := s.Images.Store.CreateSandboxBuild(ctx, store.NewSandboxBuild{
		WorkspaceID: group.WorkspaceID, SandboxGroupID: group.ID, Name: req.Name,
		SourceRepository: src.source.Repository, SourceImage: src.source.Image, SourceReference: src.source.Reference,
		SourceDigest: src.ref.DigestStr(), SizeBytes: src.size, MaxStorageBytes: s.MaxStorageBytes,
		PlatformBudgetBytes: s.Images.PlatformBudgetBytes, WorkspaceShareBytes: s.Images.workspaceShare(),
	}, s.Images.maxInFlight())
	var limit *store.SandboxStorageLimitError
	switch {
	case errors.As(err, &limit):
		return Build{}, s.storageLimitError(limit.UsedBytes, src.size, "import")
	case errors.Is(err, store.ErrSandboxImportBudget):
		// Counted: a registry near its platform budget is what operators most
		// need to see. The platform's used bytes cover other tenants, so the
		// refusal names only this image's size.
		s.Metrics.observeRefusal("platform", "import")
		return Build{}, core.NewTooManyRequestsError(CodeSandboxImportCapacity,
			"the sandbox registry has no room for this image right now; try again later, or delete images you no longer need",
			map[string]any{"sizeBytes": src.size})
	case errors.Is(err, store.ErrSandboxImportShare):
		s.Metrics.observeRefusal("platform", "import")
		share := s.Images.workspaceShare()
		return Build{}, core.NewTooManyRequestsError(CodeSandboxImportCapacity,
			fmt.Sprintf("this workspace's imported images already hold its %s share of the sandbox registry; delete images you no longer need, then import again", formatGB(share)),
			map[string]any{"sizeBytes": src.size, "limitBytes": share})
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
		go s.Images.run(b, src.ref, src.image, req.RegistryCredential)
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

// DeleteImage deletes one build of a group, as Render's delete-sandbox-build
// (w3/m367): it leaves lists and reads, a create can no longer start from it
// by id or name, its registry image goes, and a repeated delete succeeds.
// Sandboxes still running or suspended from it keep it: 409 image_in_use,
// naming up to ten of them. bex has no snapshots, Render's other dependents.
func (s *Service) DeleteImage(ctx context.Context, groupID, buildID string) error {
	ctx, group, err := s.imageGroup(ctx, groupID, core.RelCanCreate)
	if err != nil {
		return err
	}
	if err := validateBuildID(buildID); err != nil {
		return err
	}
	// Mark first, so no create can start from the build while its dependents
	// are read, and undo the mark if one is found.
	deleted, err := s.Images.Store.DeleteSandboxBuild(ctx, group.WorkspaceID, group.ID, buildID)
	if errors.Is(err, store.ErrNotFound) {
		return buildNotFound(buildID)
	}
	if err != nil {
		return store.MapError(err)
	}
	if !deleted {
		return nil // already deleted: Render answers 204 again
	}
	dependents, hidden, err := s.buildDependents(ctx, group.WorkspaceID, buildID)
	if err == nil && len(dependents)+hidden > 0 {
		err = imageInUse(buildID, dependents, hidden)
	}
	if err != nil {
		if rerr := s.Images.Store.RestoreSandboxBuild(ctx, group.WorkspaceID, group.ID, buildID); rerr != nil {
			return errors.Join(err, store.MapError(rerr))
		}
		return err
	}
	// A failure here is retried by the group's or workspace's delete, which
	// still lists the deleted build's image (SandboxBuildIDs).
	if err := s.Images.deleteImported(ctx, group.WorkspaceID, []string{buildID}); err != nil {
		log.Printf("sandbox images: delete the image of %s: %v", buildID, err)
	}
	return nil
}

// imageInUse is Render's 409 image_in_use, naming the dependents the caller
// may see and counting the rest, as a list would hide them.
func imageInUse(buildID string, dependents []string, hidden int) error {
	named := strings.Join(dependents, ", ")
	switch {
	case named == "":
		named = fmt.Sprintf("%d of other members", hidden)
	case hidden > 0:
		named += fmt.Sprintf(" and %d of other members", hidden)
	}
	return core.NewConflictError(CodeImageInUse,
		fmt.Sprintf("sandboxes that have not terminated still depend on this build: %s; terminate them before deleting it", named),
		map[string]any{"id": buildID, "dependents": dependents})
}

// buildDependents are up to maxImageDependents of the workspace's sandboxes,
// every member's, that started from the build and have not terminated or
// errored: suspended ones included, as on Render. dependents are the ones the
// caller may see; hidden counts the others. OpenSandbox's own list is the
// authority, not the best-effort records, so a missed record write cannot let
// a delete pull the image from under a live sandbox.
func (s *Service) buildDependents(ctx context.Context, ws, buildID string) (dependents []string, hidden int, err error) {
	if !s.enabled() {
		return nil, 0, nil
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return nil, 0, err
	}
	raw, err := s.Client.List(ctx, key)
	if err != nil {
		return nil, 0, err
	}
	visible := s.ownerCheck(ctx, ws)
	for _, r := range raw {
		if !validOwnedSandbox(r, ws) || displayImage(r.Image.URI) != buildID || endedLive(r) {
			continue
		}
		ok, err := visible(r.Metadata[metadataOwner])
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			hidden++
		} else {
			dependents = append(dependents, canonicalID(r))
		}
		if len(dependents)+hidden == maxImageDependents {
			break
		}
	}
	return dependents, hidden, nil
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
			return "", s.noSucceededBuild(ctx, workspace, group.ID, buildName)
		}
	}
	if err != nil {
		return "", store.MapError(err)
	}
	if b.Status != store.SandboxBuildSucceeded || b.ImageDigest == nil {
		return "", core.NewConflictError(CodeBuildNotSucceeded,
			fmt.Sprintf("image %s is %s; a sandbox can start from it once it has succeeded", b.ID, b.Status),
			map[string]any{"buildId": b.ID, "status": b.Status})
	}
	repo, err := s.Images.imageRepository(workspace, b.ID)
	if err != nil {
		return "", err
	}
	return repo.Digest(*b.ImageDigest).String(), nil
}

// noSucceededBuild is Render's answer for a buildName with no succeeded build:
// 409 build_not_succeeded when the group has builds of that name, and 404
// build_not_found only when it has none (w8/314).
func (s *Service) noSucceededBuild(ctx context.Context, workspace, groupID, name string) error {
	builds, err := s.Images.Store.SandboxBuilds(ctx, workspace, groupID, name, nil)
	if err != nil {
		return store.MapError(err)
	}
	if len(builds) == 0 {
		return core.NewNotFoundError(CodeBuildNotFound, fmt.Sprintf("sandbox group %s has no image named %q", groupID, name),
			map[string]any{"buildName": name, "sandboxGroupId": groupID})
	}
	newest := builds[0]
	return core.NewConflictError(CodeBuildNotSucceeded,
		fmt.Sprintf("no image named %q in sandbox group %s has succeeded; the newest, %s, is %s", name, groupID, newest.ID, newest.Status),
		map[string]any{"buildName": name, "sandboxGroupId": groupID, "buildId": newest.ID, "status": newest.Status})
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
// burst of imports queues, as pending, for its workspace's slot and then a
// worker slot; the heartbeat keeps a queued or running build fresh for the
// sweeper, and stops the import when the sweeper (or anything else) has
// settled the build already.
func (im *Images) run(b store.SandboxBuild, ref name.Digest, image v1.Hash, cred *RegistryCredential) {
	alive, stop := context.WithCancel(context.Background())
	defer stop()
	go core.Heartbeat(alive, "sandbox images "+b.ID, importHeartbeat, stop, func(ctx context.Context) (bool, error) {
		return im.Store.TouchSandboxBuild(ctx, b.ID)
	})
	release, ok := im.tenants.acquire(alive, b.WorkspaceID)
	if !ok {
		return
	}
	defer release()
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
	digest, size, err := im.copy(ctx, b, ref, image, cred)
	// The settle must land even when the copy used up the import's deadline.
	settle, done := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer done()
	if err != nil {
		reason := "the image could not be copied: " + err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			reason = fmt.Sprintf("the import did not finish within %s", importTimeout)
		}
		if ferr := im.Store.FailSandboxBuild(settle, b.ID, buildFailureCode(err), reason); ferr != nil {
			log.Printf("sandbox images: fail %s: %v", b.ID, ferr)
		}
		return
	}
	settled, err := im.Store.SucceedSandboxBuild(settle, b.ID, digest, size)
	if err != nil {
		log.Printf("sandbox images: succeed %s: %v", b.ID, err)
		return
	}
	if !settled {
		// The build was failed or its group deleted while the copy ran: the
		// image it just wrote is named by nothing, so it goes too.
		if err := im.deleteImported(settle, b.WorkspaceID, []string{b.ID}); err != nil {
			log.Printf("sandbox images: delete the orphaned image of %s: %v", b.ID, err)
		}
	}
}

// deleteImported deletes imported images from the registry, every one it can;
// without a reaper it leaves them, as a deployment without the reaper
// credential does.
func (im *Images) deleteImported(ctx context.Context, workspace string, buildIDs []string) error {
	if im.Reaper == nil {
		return nil
	}
	var errs []error
	for _, id := range buildIDs {
		if err := im.Reaper.DeleteImportedImage(ctx, workspace, id); err != nil {
			errs = append(errs, fmt.Errorf("delete image %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// Render's sandboxBuildFailureCode values bex records (its SDK's enum, 2026-11-13).
const (
	buildFailedDeadline     = "deadline_exceeded"
	buildFailedAccessDenied = "image_access_denied"
	buildFailedNotFound     = "image_not_found"
	buildFailedPull         = "image_pull_failed"
	buildFailedInternal     = "internal_error"
)

// sourceError is a failure reading the source image, as opposed to writing the
// sandbox registry: the source's fault, which Render names by kind.
type sourceError struct{ err error }

func (e sourceError) Error() string { return e.err.Error() }
func (e sourceError) Unwrap() error { return e.err }

// fromOtherHost reports a registry error for a request to a host other than
// host, the sandbox registry's: the source registry's refusal.
func fromOtherHost(err error, host string) bool {
	var terr *transport.Error
	return errors.As(err, &terr) && terr.Request != nil && terr.Request.URL != nil && terr.Request.URL.Host != host
}

// buildFailureCode classifies an import failure. Platform and size problems are
// refused before the import starts, so they never reach here.
func buildFailureCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return buildFailedDeadline
	}
	var src sourceError
	if !errors.As(err, &src) {
		return buildFailedInternal
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		switch terr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return buildFailedAccessDenied
		case http.StatusNotFound:
			return buildFailedNotFound
		}
	}
	return buildFailedPull
}

// copy writes the linux/amd64 image at ref to the build's repository under
// imageTag and returns the stored manifest's digest and its compressed size.
// It writes only image, the one resolveSource checked: a registry labels a
// manifest by header, so the same bytes could read as an image then and as an
// index now and pick another child (w5/m383).
func (im *Images) copy(ctx context.Context, b store.SandboxBuild, ref name.Digest, image v1.Hash, cred *RegistryCredential) (string, int64, error) {
	repo, err := im.imageRepository(b.WorkspaceID, b.ID)
	if err != nil {
		return "", 0, err
	}
	desc, err := remote.Get(ref, im.sourceOptions(ctx, cred)...)
	if err != nil {
		return "", 0, sourceError{err}
	}
	img, err := desc.Image()
	if err != nil {
		return "", 0, sourceError{err}
	}
	if got, err := img.Digest(); err != nil || got != image {
		return "", 0, sourceError{fmt.Errorf("the source served image %s, not the %s it was checked as", got, image)}
	}
	opts, err := im.registryOptions(ctx)
	if err != nil {
		return "", 0, err
	}
	if err := remote.Write(repo.Tag(imageTag), uploadOnly{img}, opts...); err != nil {
		// The source image is lazy: its layers are read during the write, so
		// a refusal from the source registry surfaces here too.
		if sink, _, herr := im.registryHost(); herr == nil && fromOtherHost(err, sink) {
			return "", 0, sourceError{err}
		}
		return "", 0, err
	}
	digest, err := img.Digest()
	if err != nil {
		return "", 0, err
	}
	size, err := imageBytes(img)
	return digest.String(), size, err
}

// registryOptions reach the sandbox registry with the importer's credential.
func (im *Images) registryOptions(ctx context.Context) ([]remote.Option, error) {
	user, password, err := im.Registry.credentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("the sandbox registry credential is unavailable: %w", err)
	}
	opts := []remote.Option{remote.WithContext(ctx), remote.WithAuth(authn.FromConfig(authn.AuthConfig{Username: user, Password: password}))}
	if im.Registry.HTTP != nil && im.Registry.HTTP.Transport != nil {
		opts = append(opts, remote.WithTransport(im.Registry.HTTP.Transport))
	}
	return opts, nil
}

// snapshotImage is the newest image a paused sandbox committed, and its
// compressed size.
func (im *Images) snapshotImage(ctx context.Context, workspaceID, sandboxRef string) (v1.Image, int64, error) {
	path, tag, err := im.Registry.newestSnapshot(ctx, workspaceID, sandboxRef)
	if err != nil {
		return nil, 0, fmt.Errorf("find the builder's snapshot: %w", err)
	}
	repo, err := im.repository(path)
	if err != nil {
		return nil, 0, err
	}
	opts, err := im.registryOptions(ctx)
	if err != nil {
		return nil, 0, err
	}
	img, err := remote.Image(repo.Tag(tag), opts...)
	if err != nil {
		return nil, 0, fmt.Errorf("read the builder's snapshot: %w", err)
	}
	size, err := imageBytes(img)
	if err != nil {
		return nil, 0, fmt.Errorf("size the builder's snapshot: %w", err)
	}
	return img, size, nil
}

// pushAgentImage writes a builder's committed image to its build's repository
// under imageTag (ADR111 D1) and returns its digest. Its layers are mounted,
// not uploaded again: unlike an import's, they are the ones the controller's
// commit Job wrote into this workspace's own snapshot repository, so no
// digest here names another workspace's blob.
func (im *Images) pushAgentImage(ctx context.Context, workspaceID, buildID string, img v1.Image) (string, error) {
	repo, err := im.imageRepository(workspaceID, buildID)
	if err != nil {
		return "", err
	}
	opts, err := im.registryOptions(ctx)
	if err != nil {
		return "", err
	}
	if err := remote.Write(repo.Tag(imageTag), img, opts...); err != nil {
		return "", fmt.Errorf("copy the builder's snapshot: %w", err)
	}
	digest, err := img.Digest()
	return digest.String(), err
}

// uploadOnly is an image whose layers the write uploads, never mounts
// (w5/192). go-containerregistry asks the sink to mount every layer it read
// from a source (?mount=<digest>&from=…&origin=…), whatever the source
// registry, and Zot links a mounted digest from any repository the pusher can
// read, whatever the from (verified against v2.1.22; the importer reads every
// workspace's). An untrusted source could then name another workspace's layer
// digest and have it linked into its own image. A plain layer is uploaded
// from the bytes the source actually served. (A HEAD links nothing while
// storage.hydrateBlobOnRead stays off, which the operator pins.) The cost: a
// layer the registry already holds is sent again rather than linked.
type uploadOnly struct{ v1.Image }

func (i uploadOnly) Layers() ([]v1.Layer, error) {
	layers, err := i.Image.Layers()
	if err != nil {
		return nil, err
	}
	out := make([]v1.Layer, len(layers))
	for n, l := range layers {
		if mountable, ok := l.(*remote.MountableLayer); ok {
			l = mountable.Layer
		}
		out[n] = l
	}
	return out, nil
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
