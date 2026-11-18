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
}
