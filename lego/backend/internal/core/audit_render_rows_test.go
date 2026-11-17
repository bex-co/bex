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

package core

import (
	"context"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DiffEnvVars sorts one save's keys into Render's three kinds (w9/m449).
func TestDiffEnvVarsSortsKeysByKind(t *testing.T) {
	got := DiffEnvVars(
		map[string]string{"KEEP": "1", "EDIT": "1", "GONE": "1", "B_GONE": "1"},
		map[string]string{"KEEP": "1", "EDIT": "2", "NEW": "1", "A_NEW": "1"},
	)
	if !slices.Equal(got.Created, []string{"A_NEW", "NEW"}) || !slices.Equal(got.Updated, []string{"EDIT"}) ||
		!slices.Equal(got.Deleted, []string{"B_GONE", "GONE"}) {
		t.Fatalf("DiffEnvVars = %+v", got)
	}
	if got := DiffEnvVars(map[string]string{"K": "v"}, map[string]string{"K": "v"}); got.Created != nil || got.Updated != nil || got.Deleted != nil {
		t.Fatalf("an unchanged map = %+v, want no keys", got)
	}
}

// An env group linked to or unlinked from a build source records Render's
// build-source env-group verbs; a service's link keeps envgroups.LinkService,
// which its feed reads (w9/m449).
func TestEnvGroupLinkOnABuildSourceRecordsItsOwnVerb(t *testing.T) {
	sink := &recordingSink{}
	b := &Base{Audit: sink}
	app := func(name, typ string) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{LabelTenant: "tea-a", LabelAppID: name}},
			Spec:       appv1alpha1.AppSpec{Type: typ},
		}
	}
	ctx := context.Background()
	b.RecordEnvGroupLinkChanged(ctx, app("bds-a", appv1alpha1.TypeBuildSource), AuditVerbLinkService, "evg-1", "shared")
	b.RecordEnvGroupLinkChanged(ctx, app("bds-a", appv1alpha1.TypeBuildSource), AuditVerbUnlinkService, "evg-1", "shared")
	b.RecordEnvGroupLinkChanged(ctx, app("srv-a", appv1alpha1.TypeWebService), AuditVerbLinkService, "evg-1", "shared")
	var got []string
	for _, ev := range sink.events {
		got = append(got, ev.Verb+" "+ev.Target+" "+ev.EnvGroupID)
	}
	want := []string{
		AuditVerbLinkBuildSourceEnvGroup + " service:bds-a evg-1",
		AuditVerbUnlinkBuildSourceEnvGroup + " service:bds-a evg-1",
		AuditVerbLinkService + " service:srv-a evg-1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
}
