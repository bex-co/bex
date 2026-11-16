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

package deploys

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Render (render.com/docs/rollbacks § Registry-hosted Docker images): "The
// rollback uses the same image tag or digest as the target deploy. With a tag,
// Render pulls the latest image under that tag ... A digest always refers to
// the same image" (w4/m432).

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// rollbackImageFixture: the target deploy ran image (as deployed) and the
// operator resolved it to resolved; a later deploy of web:v2 serves now.
func rollbackImageFixture(t *testing.T, image, resolved string) (*Service, *fakeStore, store.Deploy) {
	t.Helper()
	ctx := context.Background()
	ds := newFakeStore()
	target, _ := ds.CreateDeploy(ctx, "srv-1", "create", image, 1, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, target.ID, store.DeployLive, resolved); err != nil || !won {
		t.Fatalf("close target: %v %v", won, err)
	}
	next, _ := ds.CreateDeploy(ctx, "srv-1", "api", "docker.io/acme/web:v2", 2, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, next.ID, store.DeployLive, "docker.io/acme/web@"+digestB); err != nil || !won {
		t.Fatalf("close next: %v %v", won, err)
	}
	app := sampleApp("web", "srv-1")
	app.Generation = 2
	app.Spec.Image = "docker.io/acme/web:v2"
	svc, _ := newService(ds, app)
	return svc, ds, target
}

func TestRollbackSelectsTheTargetsImageReference(t *testing.T) {
	for _, tc := range []struct {
		name, image, resolved, want string
	}{
		// A tag is pulled again: the operator resolves it as a new release, so
		// the rollback runs whatever the tag names now.
		{"tag", "docker.io/acme/web:v1", "docker.io/acme/web@" + digestA, "docker.io/acme/web:v1"},
		// A digest always names the same image.
		{"digest", "docker.io/acme/web@" + digestA, "docker.io/acme/web@" + digestA, "docker.io/acme/web@" + digestA},
		// A build bex made from source is its own retained artifact: the exact
		// image the target ran, never its mutable gen-N tag.
		{"repository build", "", "zot.bex-registry.svc:5000/t/web:gen-1@" + digestA, "zot.bex-registry.svc:5000/t/web:gen-1@" + digestA},
		{"platform registry reference", "zot.bex-registry.svc:5000/t/web:gen-1", "zot.bex-registry.svc:5000/t/web:gen-1@" + digestA, "zot.bex-registry.svc:5000/t/web:gen-1@" + digestA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, target := rollbackImageFixture(t, tc.image, tc.resolved)
			rolled, err := svc.Rollback(context.Background(), "web", target.ID)
			if err != nil {
				t.Fatalf("Rollback: %v", err)
			}
			selected := getApp(t, svc.Client, "web").Spec.ReleaseConfig
			if selected == nil || selected.Image != tc.want {
				t.Fatalf("selection = %+v, want image %q", selected, tc.want)
			}
			// The view prints the public form of a platform reference.
			if want := store.PublicImageRef(tc.want); rolled.Image != want {
				t.Fatalf("rollback deploy image = %q, want %q", rolled.Image, want)
			}
		})
	}
}

// The rollback row records the digest its own release resolved, so a later
// rollback to the rollback reruns the tag again, and Restart of it reruns
// exactly what it ran.
func TestRestartAfterATagRollbackRerunsTheResolvedImage(t *testing.T) {
	ctx := context.Background()
	svc, ds, target := rollbackImageFixture(t, "docker.io/acme/web:v1", "docker.io/acme/web@"+digestA)
	rolled, err := svc.Rollback(ctx, "web", target.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The tag moved: the operator resolved it to B for the rollback's release.
	if won, err := ds.CloseDeploy(ctx, rolled.ID, store.DeployLive, "docker.io/acme/web@"+digestB); err != nil || !won {
		t.Fatalf("close rollback: %v %v", won, err)
	}
	app := getApp(t, svc.Client, "web")
	app.Generation = 3
	if err := svc.Client.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	restarted, err := svc.Restart(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	selected := getApp(t, svc.Client, "web").Spec.ReleaseConfig
	if selected == nil || selected.Image != "docker.io/acme/web@"+digestB || restarted.Image != "docker.io/acme/web@"+digestB {
		t.Fatalf("restart selection = %+v, deploy %q; want the digest the rollback ran", selected, restarted.Image)
	}
}
