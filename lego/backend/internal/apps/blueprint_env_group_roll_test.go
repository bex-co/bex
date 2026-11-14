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

package apps

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// groupRollManifest declares env group shared (GRP_A=group) and web linked to
// it, on image.
func groupRollManifest(group, image string) string {
	return "envVarGroups:\n  - name: shared\n    envVars:\n      - {key: GRP_A, value: " + group + "}\n" +
		"services:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: " + image + "}\n    envVars:\n      - {fromGroup: shared}\n"
}

// newGroupRollService is a stack over an existing store-managed web (srv-1)
// whose group owes the listed services a roll on change.
func newGroupRollService(t *testing.T, owed ...string) (*Service, *recordingStore, *fakeEnvGroups) {
	t.Helper()
	rec := &recordingStore{}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, _ := newService(rec, existing)
	groups := newFakeEnvGroups()
	svc.EnvGroups, svc.EnvSeeder = groups, &fakeSeeder{}
	// Settle web on the manifest first, so a later apply changes only what
	// each test changes.
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: groupRollManifest("group", "new:1")}); err != nil {
		t.Fatalf("settling DeployStack: %v", err)
	}
	rec.deployCalls, groups.owed = nil, map[string][]string{"shared": owed}
	return svc, rec, groups
}

// w4/633: one Blueprint apply that changes an env group and a service linked
// to it deploys the service once, with the Blueprint trigger. The group's roll
// runs after the service writes and skips the service they deployed, so no
// config_change deploy is opened only to be canceled by the Blueprint's.
func TestBlueprintGroupAndServiceChangeDeploysOnce(t *testing.T) {
	svc, rec, groups := newGroupRollService(t, "srv-1")
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: groupRollManifest("changed", "newer:1")}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(rec.deployCalls) != 1 || rec.deployCalls[0].Trigger != store.TriggerBlueprintSync {
		t.Errorf("service deploys = %+v, want one Blueprint deploy", rec.deployCalls)
	}
	if len(groups.rolled) != 0 {
		t.Errorf("group rolled %v again after the Blueprint deployed it", groups.rolled)
	}
}

// A service changed only through its group still gets its deploy, from the
// group's roll (recorded with the Blueprint trigger by envgroups), and so does
// a linked service the Blueprint does not declare.
func TestBlueprintGroupOnlyChangeRollsEachLinkedService(t *testing.T) {
	svc, rec, groups := newGroupRollService(t, "srv-1", "srv-undeclared")
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: groupRollManifest("changed", "new:1")}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(rec.deployCalls) != 0 {
		t.Errorf("service step deploys = %+v, want none: web's own declaration did not change", rec.deployCalls)
	}
	if want := []string{"srv-1", "srv-undeclared"}; !slices.Equal(groups.rolled, want) {
		t.Errorf("group rolled %v, want %v", groups.rolled, want)
	}
}

// A group's values are written before the service step, so a later failure
// still rolls the linked services rather than leaving them on the old values
// with nothing owed to a retry (the retry finds the group unchanged).
func TestBlueprintGroupRollSurvivesALaterFailure(t *testing.T) {
	svc, rec, groups := newGroupRollService(t, "srv-1")
	rec.err = errors.New("deploy history unavailable")
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: groupRollManifest("changed", "newer:1")}); err == nil {
		t.Fatal("DeployStack succeeded, want the service step's failure")
	}
	if !slices.Equal(groups.rolled, []string{"srv-1"}) {
		t.Errorf("group rolled %v, want srv-1, which the failed step never deployed", groups.rolled)
	}
}
