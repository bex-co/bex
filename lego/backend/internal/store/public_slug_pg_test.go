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
	"context"
	"strings"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
)

// TestPGAServiceCannotTakeAPublicBucketsHost: a service whose name has the shape
// of a public bucket's host label (pub-<xid>) would claim <name>.<base domain>,
// the host objectgw serves that bucket at (ADR112 D4), and a service called
// objects would claim the direct API host. It keeps its name and, as for a taken
// subdomain on Render, gets a suffixed slug. A service that is only called
// pub-something is untouched.
func TestPGAServiceCannotTakeAPublicBucketsHost(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ten, err := st.CreateTenant(ctx, "public-slug", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, ten.ID) })

	bucketLabel, _ := publichost.Label(ids.New(ids.ObjectBucket))
	create := func(name string) App {
		t.Helper()
		app, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: name, Image: "nginx:1", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatalf("CreateApp(%q): %v", name, err)
		}
		forgetSlugAtCleanup(t, st, app)
		return app
	}

	shaped := create(bucketLabel)
	if shaped.Name != bucketLabel {
		t.Errorf("the service lost its name: %q", shaped.Name)
	}
	if shaped.Slug == bucketLabel || !strings.HasPrefix(shaped.Slug, bucketLabel+"-") || len(shaped.Slug) != len(bucketLabel)+len("-xxxx") {
		t.Errorf("slug = %q, want %s plus a four-character suffix: the bare name is a bucket's host", shaped.Slug, bucketLabel)
	}
	if publichost.Reserved(shaped.Slug) {
		t.Errorf("the suffixed slug %q still has a bucket host's shape", shaped.Slug)
	}
	got, err := st.GetApp(ctx, shaped.ID)
	if err != nil || got.Slug != shaped.Slug {
		t.Errorf("stored slug = %q, %v; want %q", got.Slug, err, shaped.Slug)
	}

	// objects.<base domain> is the direct host of the Cloudflare-shaped object
	// storage API (ADR112 D3), so a service called objects gets a suffixed slug too.
	objects := create(publichost.APILabel)
	if objects.Name != publichost.APILabel || objects.Slug == publichost.APILabel || !strings.HasPrefix(objects.Slug, publichost.APILabel+"-") {
		t.Errorf("service %q got name %q and slug %q, want its name kept and a suffixed slug: the bare name is the API's host", publichost.APILabel, objects.Name, objects.Slug)
	}

	// Names that merely start with pub- keep the bare slug: Render lets a service be called that.
	for _, name := range []string{"pub-site", "pub-" + strings.Repeat("z", 20), "pub-" + strings.Repeat("0", 19), "pub", "xpub-" + strings.Repeat("0", 20)} {
		if app := create(name); app.Slug != name {
			t.Errorf("service %q got slug %q, want its own name: only the exact shape of a bucket host is reserved", name, app.Slug)
		}
	}
}
