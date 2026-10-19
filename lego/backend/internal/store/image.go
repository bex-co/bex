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
	_ "crypto/sha256" // Register OCI digest algorithms independently of other imports.
	_ "crypto/sha512"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	digest "github.com/opencontainers/go-digest"

	"github.com/bex-co/bex/lego/types/netutil"
)

// ValidateImage checks an OCI image reference's grammar and registry policy.
// Short Docker Hub names and tag+digest references are accepted without rewriting
// the stored value. The kubelet pulls from the node network, outside pod egress
// policy: private/reserved IPs and untrusted registry hosts must stay blocked.
// Checking DNS here would not protect the kubelet's later resolution from rebinding.
func ValidateImage(v string) error {
	if v == "" {
		return fmt.Errorf("image reference is required")
	}
	if len(v) > 512 {
		return fmt.Errorf("image reference must be at most 512 bytes")
	}
	// The reference grammar can reject a short digest before the digest parser
	// sees it. Validate the suffix first so callers get the actual digest reason.
	if _, checksum, ok := strings.Cut(v, "@"); ok {
		if _, err := digest.Parse(checksum); err != nil {
			return fmt.Errorf("image digest is invalid: %w", err)
		}
	}
	named, err := reference.ParseNormalizedNamed(v)
	if err != nil {
		return fmt.Errorf("image reference is malformed: %w", err)
	}
	registry := reference.Domain(named)
	// Preserve the explicit host before Docker Hub alias normalization; parsing
	// index.docker.io as docker.io must not expand the platform's trusted set.
	if segment, _, ok := registrySegment(v); ok {
		registry = segment
	}
	host := (&url.URL{Host: registry}).Hostname()
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("image registry host %q is private or reserved", host)
	}
	if ip := net.ParseIP(host); ip != nil && netutil.UnsafeOriginIP(ip) {
		return fmt.Errorf("image registry host %q is private or reserved", host)
	}
	if !trustedImageRegistry(host) {
		return fmt.Errorf("image registry host %q is not trusted", host)
	}
	return nil
}

// ImageRepository returns the registry host + repository of an image reference
// with Docker Hub's short forms normalized (`nginx` and
// `docker.io/library/nginx` are one repository) and the tag/digest dropped.
// Render's imageUrl contract compares exactly this: a per-deploy image must
// keep the service's configured host, repository and image name.
func ImageRepository(v string) (string, error) {
	named, err := reference.ParseNormalizedNamed(v)
	if err != nil {
		return "", fmt.Errorf("image reference is malformed: %w", err)
	}
	return strings.ToLower(reference.Domain(named)) + "/" + reference.Path(named), nil
}

// PublicImageRef returns the tenant-facing form of a stored deploy image
// reference (w4/m236). A reference whose registry host is cluster-internal
// (the platform's own zot build registry, any *.svc or *.cluster.local host)
// loses that host, keeping the repository path with its tag/digest, so no
// tenant surface prints cluster-internal DNS. Every other reference — a
// user-supplied image above all — passes through byte-identical. The stored
// value is never changed; this runs at the read-projection boundary only, so
// rollback still runs the exact stored image.
func PublicImageRef(v string) string {
	segment, rest, ok := registrySegment(v)
	if !ok {
		return v
	}
	hostname := normalizeRegistryHost((&url.URL{Host: segment}).Hostname())
	if hostname == "zot.bex-registry.svc" ||
		strings.HasSuffix(hostname, ".svc") ||
		strings.HasSuffix(hostname, ".cluster.local") {
		return rest
	}
	return v
}

// registrySegment returns the explicit registry host segment (host[:port]) of
// an image reference and the remainder after it. A first path segment counts
// as a registry only when it contains a dot or a port colon — the reference
// grammar's own rule, which keeps Docker Hub short forms (`traefik/whoami`)
// from being mistaken for a host.
func registrySegment(v string) (segment, rest string, ok bool) {
	segment, rest, hasSlash := strings.Cut(v, "/")
	if !hasSlash || !strings.ContainsAny(segment, ".:") {
		return "", "", false
	}
	return segment, rest, true
}

func normalizeRegistryHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

func trustedImageRegistry(host string) bool {
	host = normalizeRegistryHost(host)
	switch host {
	case "docker.io", "registry-1.docker.io", "ghcr.io", "quay.io", "gcr.io",
		"registry.k8s.io", "public.ecr.aws", "mcr.microsoft.com",
		"zot.bex-registry.svc", "zot.bex-registry.svc.cluster.local":
		return true
	}
	return strings.HasSuffix(host, ".pkg.dev")
}
