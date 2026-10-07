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

package envgroups

import (
	"context"
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appPatchRacer applies race to the live App once, just before the first App
// patch goes out: a write landing between a writer's read and its patch.
type appPatchRacer struct {
	client.Client
	race func(*appv1alpha1.App)
}

func (c *appPatchRacer) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*appv1alpha1.App); ok && c.race != nil {
		race := c.race
		c.race = nil
		live := &appv1alpha1.App{}
		if err := c.Client.Get(ctx, client.ObjectKeyFromObject(obj), live); err != nil {
			return err
		}
		race(live)
		if err := c.Client.Update(ctx, live); err != nil {
			return err
		}
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

// TestALinkThatLosesTheAppRaceKeepsTheServicesFilesMount (w5/m131): a link
// computed the App's files list from its read, and a merge patch replaces the
// whole list. When a secrets write added the service's own files Secret in
// between, the link's patch dropped it, on a rolling service and on one whose
// auto-deploy is off alike.
func TestALinkThatLosesTheAppRaceKeepsTheServicesFilesMount(t *testing.T) {
	for name, app := range map[string]*appv1alpha1.App{"rolling": sampleApp("web"), "auto-deploy off": repoApp("web", false)} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			svc := newService(newFakeStore(), app)
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			svc.Client = &appPatchRacer{Client: svc.Client, race: func(a *appv1alpha1.App) {
				a.Spec.FilesFromSecrets = append(a.Spec.FilesFromSecrets, "web-files")
			}}

			if err := svc.LinkService(ctx, group.ID, "web"); err != nil {
				t.Fatalf("link: %v", err)
			}
			files := getApp(t, svc.Client, "web").Spec.FilesFromSecrets
			if !slices.Contains(files, "web-files") || !slices.Contains(files, filesSecretName(group.ID)) {
				t.Fatalf("files = %v, want the service's own Secret kept beside the group's", files)
			}
		})
	}
}
