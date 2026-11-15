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

package objectstorage

import (
	"context"
	"errors"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ErrNoHostingDomain answers a request to publish a bucket on a deployment
// that has no hosting domain: a public bucket's host lives under
// BEX_BASE_DOMAIN (ADR112 D4), so without one there is nothing to serve.
var ErrNoHostingDomain = errors.New("public buckets need a platform hosting domain")

// PublicDomain is the host a bucket is served at when public, under baseDomain.
func PublicDomain(b store.ObjectBucket, baseDomain string) (string, error) {
	if baseDomain == "" {
		return "", ErrNoHostingDomain
	}
	host, ok := publichost.Host(b.ID, baseDomain)
	if !ok {
		return "", ErrNoHostingDomain
	}
	return host, nil
}

// SetPublic turns public reads of a bucket on or off (ADR112 D4). Turning them
// on is refused when the deployment has no hosting domain, so a bucket is never
// marked public where nothing would serve it; turning them off always works.
func (s *Service) SetPublic(ctx context.Context, workspaceID string, b store.ObjectBucket, enabled bool, baseDomain string) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	if enabled {
		if _, err := PublicDomain(b, baseDomain); err != nil {
			return err
		}
	}
	if err := s.repo.SetObjectBucketPublic(ctx, workspaceID, b.ID, enabled); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBucketNotFound
		}
		return err
	}
	return nil
}

// PublicBucket is what the gateway needs to serve one public bucket: where its
// objects live and the CORS rules to apply to them.
type PublicBucket struct {
	BucketID    string
	WorkspaceID string
	Region      Region
	CORS        []CORSRule
}

// PublicBucketForHost resolves a request host to the public bucket it names.
// Anything else answers ErrBucketNotFound: a host that is not a bucket's, a
// bucket that is private, deleted or unknown, and a region this deployment does
// not serve. The gateway cannot tell them apart, and neither can a visitor.
func (s *Service) PublicBucketForHost(ctx context.Context, host, baseDomain string) (PublicBucket, error) {
	if !s.Enabled() {
		return PublicBucket{}, ErrNotConfigured
	}
	id, ok := publichost.BucketIDOfHost(host, baseDomain)
	if !ok {
		return PublicBucket{}, ErrBucketNotFound
	}
	b, err := s.repo.GetObjectBucketByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return PublicBucket{}, ErrBucketNotFound
	}
	if err != nil {
		return PublicBucket{}, err
	}
	region, err := ParseRegion(b.Region)
	if err != nil || !b.Public {
		return PublicBucket{}, ErrBucketNotFound
	}
	if _, err := s.Store(region); err != nil {
		return PublicBucket{}, ErrBucketNotFound
	}
	rules, err := ParseCORS(b.CORS)
	if err != nil {
		// Stored rules are validated on the way in; a row that does not parse
		// must not open the bucket to every origin or close it to all.
		return PublicBucket{}, err
	}
	return PublicBucket{BucketID: b.ID, WorkspaceID: b.WorkspaceID, Region: region, CORS: rules}, nil
}
