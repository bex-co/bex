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

package logs

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m477: below Pro a service that sets AWS_ROLE_ARN keeps the variable but
// gets no token, and its deploy log says why. The line belongs to the deploy
// whose release made the decision, never to another deploy, and a release
// that mounts its token, or sets no trigger, says nothing.
func TestManagedOIDCRefusalIsALineInTheDeployThatAskedForIt(t *testing.T) {
	const refused = "==> Managed OIDC requires a Pro workspace or higher: AWS_ROLE_ARN is set, but no identity token is mounted"
	read := func(st *appv1alpha1.ManagedOIDCStatus, deploy string) []string {
		t.Helper()
		app := sampleApp("web")
		app.Status.ManagedOIDC = st
		svc := newService(nil, app)
		svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
		svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return hookRows(), nil }
		return inOrder(buildRead(t, svc, LogQuery{Deploy: deploy, Since: hookLive.Add(-2 * time.Minute), End: hookLive.Add(2 * time.Minute)}))
	}
	hobby := &appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"aws"}}

	got := read(hobby, "dep-second")
	want := []string{"==> Deploy queued", "==> Rolling out the current release", refused, "==> Your service is live 🎉"}
	if !slices.Equal(got, want) {
		t.Fatalf("the refused release's deploy log = %q\nwant %q", got, want)
	}
	for name, c := range map[string]struct {
		st     *appv1alpha1.ManagedOIDCStatus
		deploy string
	}{
		"an earlier deploy":     {hobby, "dep-first"},
		"a mounted release":     {&appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"aws"}, PlanAllowed: true}, "dep-second"},
		"a release with no ask": {nil, "dep-second"},
	} {
		if slices.Contains(read(c.st, c.deploy), refused) {
			t.Errorf("%s: the log carries the refusal; it belongs only to the refused release's deploy", name)
		}
	}

	// w2/m478: a release that asks for several providers names every trigger
	// in one line, in the order the providers mount.
	for providers, want := range map[string]string{
		"anthropic":            "==> Managed OIDC requires a Pro workspace or higher: ANTHROPIC_FEDERATION_RULE_ID is set, but no identity token is mounted",
		"aws openai":           "==> Managed OIDC requires a Pro workspace or higher: AWS_ROLE_ARN and OPENAI_IDENTITY_PROVIDER_ID are set, but no identity token is mounted",
		"aws anthropic openai": "==> Managed OIDC requires a Pro workspace or higher: AWS_ROLE_ARN, ANTHROPIC_FEDERATION_RULE_ID and OPENAI_IDENTITY_PROVIDER_ID are set, but no identity token is mounted",
	} {
		got := read(&appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: strings.Fields(providers)}, "dep-second")
		if !slices.Contains(got, want) {
			t.Errorf("a refused release asking for %s: log %q\nwant the line %q", providers, got, want)
		}
	}

	// w2/m479: a GCP_WORKLOAD_IDENTITY_PROVIDER naming the project by its id
	// mints nothing, and the deploy says so by name, in a Pro workspace
	// beside the providers it does mount and below Pro after the plan line.
	gcp := appv1alpha1.NewManagedOIDCGCP("projects/my-project/locations/global/workloadIdentityPools/p/providers/x", "")
	const projectID = `==> GCP_WORKLOAD_IDENTITY_PROVIDER names project "my-project" by its ID: Google Cloud expects the project number (IAM & Admin > Settings) in the provider's audience, so no Google Cloud credential configuration is mounted`
	got = read(&appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"aws"}, PlanAllowed: true, GCP: &gcp}, "dep-second")
	want = []string{"==> Deploy queued", "==> Rolling out the current release", projectID, "==> Your service is live 🎉"}
	if !slices.Equal(got, want) {
		t.Errorf("a Pro release with a project id: log %q\nwant %q", got, want)
	}
	got = read(&appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"aws"}, GCP: &gcp}, "dep-second")
	want = []string{"==> Deploy queued", "==> Rolling out the current release", refused, projectID, "==> Your service is live 🎉"}
	if !slices.Equal(got, want) {
		t.Errorf("a Hobby release with a project id: log %q\nwant %q", got, want)
	}
	if got := read(&appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"aws"}, PlanAllowed: true, GCP: &gcp}, "dep-first"); slices.Contains(got, projectID) {
		t.Errorf("an earlier deploy's log carries the GCP refusal: %q", got)
	}
	// A provider Google can take, mounted or refused by the plan, has no GCP line.
	ok := appv1alpha1.NewManagedOIDCGCP("projects/123/locations/global/workloadIdentityPools/p/providers/x", "")
	got = read(&appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"gcp"}, GCP: &ok}, "dep-second")
	if want := "==> Managed OIDC requires a Pro workspace or higher: GCP_WORKLOAD_IDENTITY_PROVIDER is set, but no identity token is mounted"; !slices.Contains(got, want) {
		t.Errorf("a Hobby release asking for GCP: log %q\nwant the line %q", got, want)
	}
}
