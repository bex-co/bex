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

package objectstorage_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type publicRepo struct {
	objectstorage.Repository
	mu      sync.Mutex
	buckets map[string]store.ObjectBucket
}

func (r *publicRepo) GetObjectBucketByID(_ context.Context, id string) (store.ObjectBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.buckets[id]; ok {
		return b, nil
	}
	return store.ObjectBucket{}, store.ErrNotFound
}

func (r *publicRepo) SetObjectBucketPublic(_ context.Context, ws, id string, public bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[id]
	if !ok || b.WorkspaceID != ws {
		return store.ErrNotFound
	}
	b.Public = public
	r.buckets[id] = b
	return nil
}

func newPublicEnv(t *testing.T) (*objectstorage.Service, *publicRepo, store.ObjectBucket) {
	t.Helper()
	stores, _ := oregonOnly(t)
	repo := &publicRepo{buckets: map[string]store.ObjectBucket{}}
	b := store.ObjectBucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: ids.New(ids.Workspace), Name: "photos", Region: "oregon", CORS: json.RawMessage(`[]`)}
	repo.buckets[b.ID] = b
	return objectstorage.NewService(repo, stores), repo, b
}

func TestSetPublicTurnsAPublicHostOnAndOff(t *testing.T) {
	svc, _, b := newPublicEnv(t)
	ctx := t.Context()
	host, err := objectstorage.PublicDomain(b, "onbex.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicBucketForHost(ctx, host, "onbex.test"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Fatalf("a private bucket resolved: %v", err)
	}
	if err := svc.SetPublic(ctx, b.WorkspaceID, b, true, "onbex.test"); err != nil {
		t.Fatal(err)
	}
	pb, err := svc.PublicBucketForHost(ctx, host, "onbex.test")
	if err != nil || pb.BucketID != b.ID || pb.WorkspaceID != b.WorkspaceID || pb.Region != objectstorage.Oregon {
		t.Fatalf("public host = %+v, %v", pb, err)
	}
	if err := svc.SetPublic(ctx, b.WorkspaceID, b, false, "onbex.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicBucketForHost(ctx, host, "onbex.test"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Errorf("a bucket turned private still resolves: %v", err)
	}
}

func TestSetPublicRefusesWhatCannotBeServed(t *testing.T) {
	svc, repo, b := newPublicEnv(t)
	ctx := t.Context()
	if err := svc.SetPublic(ctx, b.WorkspaceID, b, true, ""); !errors.Is(err, objectstorage.ErrNoHostingDomain) {
		t.Errorf("no hosting domain = %v, want ErrNoHostingDomain", err)
	}
	if repo.buckets[b.ID].Public {
		t.Error("a bucket was marked public where nothing would serve it")
	}
	// Turning it off never needs the domain: a bucket published before the domain
	// was removed must still be made private.
	if err := svc.SetPublic(ctx, b.WorkspaceID, b, true, "onbex.test"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPublic(ctx, b.WorkspaceID, b, false, ""); err != nil || repo.buckets[b.ID].Public {
		t.Errorf("disabling with no hosting domain = %v, public %v; want it to work", err, repo.buckets[b.ID].Public)
	}
	// Another workspace cannot publish this bucket: the repository scopes by workspace.
	if err := svc.SetPublic(ctx, ids.New(ids.Workspace), b, true, "onbex.test"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Errorf("another workspace = %v, want ErrBucketNotFound", err)
	}
	if repo.buckets[b.ID].Public {
		t.Error("another workspace published the bucket")
	}
	off := objectstorage.NewService(repo, nil)
	if err := off.SetPublic(ctx, b.WorkspaceID, b, true, "onbex.test"); !errors.Is(err, objectstorage.ErrNotConfigured) {
		t.Errorf("object storage off = %v, want ErrNotConfigured", err)
	}
	if _, err := off.PublicBucketForHost(ctx, "pub-x.onbex.test", "onbex.test"); !errors.Is(err, objectstorage.ErrNotConfigured) {
		t.Errorf("object storage off, lookup = %v, want ErrNotConfigured", err)
	}
}

// TestPublicBucketForHostAnswersOnlyForAPublicLiveBucketOfAServedRegion: every
// other case is the one answer, ErrBucketNotFound, so nothing tells a private
// bucket from one that does not exist.
func TestPublicBucketForHostAnswersOnlyForAPublicLiveBucketOfAServedRegion(t *testing.T) {
	svc, repo, b := newPublicEnv(t)
	ctx := t.Context()
	host, _ := objectstorage.PublicDomain(b, "onbex.test")
	_ = svc.SetPublic(ctx, b.WorkspaceID, b, true, "onbex.test")

	for name, in := range map[string]string{
		"another domain": mustLabel(b.ID) + ".example.org", "the apex": "onbex.test", "a service": "api.onbex.test", "empty": "",
		"unknown bucket": mustLabel(ids.New(ids.ObjectBucket)) + ".onbex.test",
	} {
		if _, err := svc.PublicBucketForHost(ctx, in, "onbex.test"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
			t.Errorf("%s: %v, want ErrBucketNotFound", name, err)
		}
	}
	if _, err := svc.PublicBucketForHost(ctx, host, ""); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Errorf("no hosting domain: %v, want ErrBucketNotFound", err)
	}

	// A bucket in a region this deployment does not serve.
	repo.mu.Lock()
	odd := repo.buckets[b.ID]
	odd.Region = "ohio"
	repo.buckets[b.ID] = odd
	repo.mu.Unlock()
	if _, err := svc.PublicBucketForHost(ctx, host, "onbex.test"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Errorf("an unserved region: %v, want ErrBucketNotFound", err)
	}
}

func TestPublicBucketForHostCarriesTheCORSRulesAndRefusesAStoredSetItCannotRead(t *testing.T) {
	svc, repo, b := newPublicEnv(t)
	ctx := t.Context()
	host, _ := objectstorage.PublicDomain(b, "onbex.test")
	repo.mu.Lock()
	b.Public = true
	b.CORS = json.RawMessage(`[{"allowed":{"origins":["https://a.example"],"methods":["GET"]},"maxAgeSeconds":30}]`)
	repo.buckets[b.ID] = b
	repo.mu.Unlock()
	pb, err := svc.PublicBucketForHost(ctx, host, "onbex.test")
	if err != nil || len(pb.CORS) != 1 || pb.CORS[0].Allowed.Origins[0] != "https://a.example" || *pb.CORS[0].MaxAgeSeconds != 30 {
		t.Fatalf("rules = %+v, %v", pb.CORS, err)
	}
	// A row that does not parse is an error the gateway turns into 503, not a
	// bucket served to every origin or none.
	repo.mu.Lock()
	b.CORS = json.RawMessage(`{"not":"a list"}`)
	repo.buckets[b.ID] = b
	repo.mu.Unlock()
	if _, err := svc.PublicBucketForHost(ctx, host, "onbex.test"); err == nil || errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Errorf("an unreadable stored rule set = %v, want an error that is not a not-found", err)
	}
}

func mustLabel(bucketID string) string {
	label, _ := publichost.Label(bucketID)
	return label
}
