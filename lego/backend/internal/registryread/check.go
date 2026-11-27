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

package registryread

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/bex-co/bex/lego/types/netutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The image check before a service is created (w1/m673): Render's
// isDeployableImage. It reads the manifest a deploy would pull, with the
// credential the deploy would use, and answers deployable or one of Render's
// five failure codes. The table it follows is in
// docs/render-artifacts/new-service-wizard.md § The image check.

// Render's UndeployableImageFailureCode values.
const (
	FailureDoesNotExist              = "does_not_exist"
	FailureImageTooLarge             = "image_too_large"
	FailureInvalidInputURL           = "invalid_input_url"
	FailureInvalidRegistryCredential = "invalid_registry_credential"
	FailureUnsupportedPlatform       = "unsupported_platform"
)

// FailureCodes lists Render's codes in its enum's order.
var FailureCodes = []string{
	FailureDoesNotExist, FailureImageTooLarge, FailureInvalidInputURL,
	FailureInvalidRegistryCredential, FailureUnsupportedPlatform,
}

// Verdict is Render's IsDeployableImageResult. Code is "" when the image is
// deployable, and also when the registry failed in a way none of Render's
// codes names (it did not answer, or answered 429 or 5xx).
type Verdict struct {
	Deployable bool
	Code       string
	Reason     string
}

// Refused is an undeployable verdict.
func Refused(code, reason string) Verdict { return Verdict{Code: code, Reason: reason} }

// Credential is the username and secret a deploy's pull secret carries.
type Credential struct {
	Username string
	Password string
}

// DefaultPlatform is production's node pool, the operator's default.
var DefaultPlatform = v1.Platform{OS: "linux", Architecture: "amd64"}

// ParsePlatform parses BEX_IMAGE_PLATFORM exactly as the operator does
// (lego/operator/internal/registry ParsePlatform): "" is DefaultPlatform,
// otherwise "linux/amd64" or "linux/arm64". The backend cannot import the
// operator, so the rule is kept twice; both refuse any other value.
func ParsePlatform(value string) (v1.Platform, error) {
	switch value {
	case "":
		return DefaultPlatform, nil
	case "linux/amd64", "linux/arm64":
		goos, arch, _ := strings.Cut(value, "/")
		return v1.Platform{OS: goos, Architecture: arch}, nil
	}
	return v1.Platform{}, fmt.Errorf("image platform %q is not linux/amd64 or linux/arm64", value)
}

const (
	defaultTimeout      = 10 * time.Second
	defaultPerWorkspace = 3
)

// Checker reads an image the way a deploy will. The zero value checks
// linux/amd64 through GuardedTransport.
type Checker struct {
	// Transport carries every registry request; nil is GuardedTransport.
	// Tests reach an in-process registry through their own.
	Transport http.RoundTripper
	// Platform is the hosting node pool's (BEX_IMAGE_PLATFORM); zero is
	// DefaultPlatform.
	Platform v1.Platform
	// MaxBytes is the compressed-size limit; 0 is Render's 10 GB
	// (appv1alpha1.MaxImageCompressedBytes), as the deploy applies it.
	MaxBytes int64
	// Timeout bounds the whole read; 0 is 10 s.
	Timeout time.Duration
	// PerWorkspace is how many checks one workspace runs at once on this
	// replica; 0 is 3.
	PerWorkspace int

	mu       sync.Mutex
	inFlight map[string]int
}

func (c *Checker) platform() v1.Platform {
	if c.Platform.OS == "" {
		return DefaultPlatform
	}
	return c.Platform
}

// Acquire takes one of workspace's check slots, without waiting. ok is false
// when the workspace already runs PerWorkspace checks.
func (c *Checker) Acquire(workspace string) (release func(), ok bool) {
	limit := c.PerWorkspace
	if limit <= 0 {
		limit = defaultPerWorkspace
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inFlight == nil {
		c.inFlight = map[string]int{}
	}
	if c.inFlight[workspace] >= limit {
		return nil, false
	}
	c.inFlight[workspace]++
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.inFlight[workspace]--; c.inFlight[workspace] <= 0 {
				delete(c.inFlight, workspace)
			}
		})
	}, true
}

// Check reads ref with cred (nil reads anonymously) and classifies what the
// registry answers. ref must already pass the reference rules create applies
// (store.ValidateImage); the caller answers invalid_input_url for one that
// does not, without a dial.
func (c *Checker) Check(ctx context.Context, ref string, cred *Credential) Verdict {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return Refused(FailureInvalidInputURL, fmt.Sprintf("%q is not an image reference: %v", ref, err))
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	auth := authn.Anonymous
	if cred != nil {
		auth = authn.FromConfig(authn.AuthConfig{Username: cred.Username, Password: cred.Password})
	}
	next := c.Transport
	if next == nil {
		next = GuardedTransport
	}
	platform := c.platform()
	opts := []remote.Option{
		remote.WithContext(ctx), remote.WithAuth(auth), remote.WithPlatform(platform),
		remote.WithTransport(CapManifests{Next: next}),
		// One attempt: the check answers while someone types, and the next
		// keystroke checks again.
		remote.WithRetryBackoff(remote.Backoff{Duration: time.Millisecond, Steps: 1}),
		remote.WithRetryStatusCodes(),
	}
	fail := func(err error) Verdict { return classify(err, cred != nil, timeout) }

	desc, err := remote.Get(parsed, opts...)
	if err != nil {
		return fail(err)
	}
	var img v1.Image
	if desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return fail(err)
		}
		manifest, err := idx.IndexManifest()
		if err != nil {
			return fail(err)
		}
		child, ok := platformChild(manifest, platform)
		if !ok {
			return Refused(FailureUnsupportedPlatform,
				fmt.Sprintf("%s has no %s image; images must be built for %s", parsed, platformName(platform), platformName(platform)))
		}
		if img, err = idx.Image(child.Digest); err != nil {
			return fail(err)
		}
	} else if img, err = desc.Image(); err != nil {
		return fail(err)
	}

	if err := CheckConfigSize(img); err != nil {
		return Refused("", fmt.Sprintf("%s: %v", parsed, err))
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return fail(err)
	}
	if cfg.OS != platform.OS || cfg.Architecture != platform.Architecture {
		return Refused(FailureUnsupportedPlatform,
			fmt.Sprintf("%s is a %s/%s image; images must be built for %s", parsed, cfg.OS, cfg.Architecture, platformName(platform)))
	}
	manifest, err := img.Manifest()
	if err != nil {
		return fail(err)
	}
	size, err := layerBytes(manifest)
	if err != nil {
		return Refused("", fmt.Sprintf("%s has an unreadable manifest: %v", parsed, err))
	}
	limit := c.MaxBytes
	if limit <= 0 {
		limit = appv1alpha1.MaxImageCompressedBytes
	}
	if size > limit {
		return Refused(FailureImageTooLarge,
			fmt.Sprintf("%s is %d bytes compressed; the limit is %d bytes", parsed, size, limit))
	}
	return Verdict{Deployable: true}
}

func platformName(p v1.Platform) string { return p.OS + "/" + p.Architecture }

// platformChild is an index's image entry for platform. An arm64 entry may
// name its variant (v8); amd64 entries carry none.
func platformChild(m *v1.IndexManifest, platform v1.Platform) (v1.Descriptor, bool) {
	for _, d := range m.Manifests {
		if d.Platform != nil && d.MediaType.IsImage() && d.Platform.Satisfies(platform) {
			return d, true
		}
	}
	return v1.Descriptor{}, false
}

// layerBytes sums a manifest's compressed layer sizes, refusing the negative
// or overflowing sizes a hostile manifest could carry.
func layerBytes(m *v1.Manifest) (int64, error) {
	var total int64
	for _, l := range m.Layers {
		if l.Size < 0 || total > (1<<62)-l.Size {
			return 0, errors.New("layer sizes out of range")
		}
		total += l.Size
	}
	return total, nil
}

// classify names a failed read with Render's code, or none. A request on the
// registry's own /v2/ API is the image's answer; any other request is the
// token endpoint's, which refuses a credential, not an image.
func classify(err error, withCredential bool, timeout time.Duration) Verdict {
	if errors.Is(err, netutil.ErrBlockedAddress) {
		return Refused(FailureInvalidInputURL, "the registry host resolves to a private or reserved address")
	}
	var dns *net.DNSError
	if errors.As(err, &dns) && dns.IsNotFound {
		return Refused(FailureDoesNotExist, fmt.Sprintf("the registry host %s does not exist", dns.Name))
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		registryAPI := terr.Request != nil && terr.Request.URL != nil && strings.HasPrefix(terr.Request.URL.Path, "/v2/")
		switch terr.StatusCode {
		case http.StatusNotFound:
			if registryAPI {
				return Refused(FailureDoesNotExist, "the registry has no such image: "+terr.Error())
			}
		case http.StatusUnauthorized, http.StatusForbidden:
			switch {
			case !withCredential:
				return Refused(FailureDoesNotExist, "the registry refused an anonymous read: "+terr.Error())
			case registryAPI:
				return Refused(FailureDoesNotExist, "the credential cannot read this image: "+terr.Error())
			default:
				return Refused(FailureInvalidRegistryCredential, "the registry refused the credential: "+terr.Error())
			}
		}
		return Refused("", fmt.Sprintf("the registry answered %d: %v", terr.StatusCode, terr))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Refused("", fmt.Sprintf("the registry did not answer within %s", timeout))
	}
	return Refused("", "the registry could not be read: "+err.Error())
}
