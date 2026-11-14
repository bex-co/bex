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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/graphql-go/graphql"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w2/m365: creating a service under the name of one just deleted, whose App
// is still being torn down (held by the operator's finalizer), succeeds at
// once, as on Render: the new service gets its own App object beside the old
// one (w2/075 answered a named 409, w2/m361 waited up to 20 s).

// deletingService is a Service whose "web" App was deleted and is still
// Terminating behind the operator's finalizer.
func deletingService(t *testing.T) (*Service, client.Client) {
	t.Helper()
	held := sampleApp("web")
	held.Finalizers = []string{"app.bex.co/finalizer"}
	svc, cl := newService(nil, held)
	if err := cl.Delete(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(held), &a); err != nil || a.DeletionTimestamp.IsZero() {
		t.Fatalf("the deleted App is not Terminating: %v", err)
	}
	return svc, cl
}

// liveAndDeleting splits the Apps answering to the service name name: how
// many are being deleted, and the live ones.
func liveAndDeleting(t *testing.T, cl client.Client, name string) (deleting int, live []appv1alpha1.App) {
	t.Helper()
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	for _, a := range list.Items {
		switch {
		case core.AppPublicName(&a) != name:
		case a.DeletionTimestamp.IsZero():
			live = append(live, a)
		default:
			deleting++
		}
	}
	return deleting, live
}

func TestREST_CreateUnderADeletedNameSucceedsAtOnce(t *testing.T) {
	svc, cl := deletingService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	// The dry run must come first: once the create lands, "web" is taken.
	for _, step := range []struct {
		body string
		want int
	}{
		{`{"name":"web","image":{"imagePath":"nginx"},"dryRun":true}`, http.StatusOK},
		{`{"name":"web","image":{"imagePath":"nginx"}}`, http.StatusCreated},
	} {
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(step.body)))
		if rec.Code != step.want {
			t.Fatalf("POST /v1/services %s = %d %s, want %d", step.body, rec.Code, rec.Body, step.want)
		}
	}
	deleting, live := liveAndDeleting(t, cl, "web")
	if deleting != 1 || len(live) != 1 || !strings.HasPrefix(live[0].Name, "web-") || len(live[0].Name) != len("web-")+4 {
		t.Fatalf("Apps: %d deleting, live %v; want the old one tearing down beside a new web-xxxx", deleting, live)
	}
	if got, err := svc.Get(context.Background(), "web"); err != nil || got.Name != "web" {
		t.Fatalf("GET web = %+v, %v; want the new service", got, err)
	}
}

func TestGraphQL_CreateServiceUnderADeletedNameSucceeds(t *testing.T) {
	svc, cl := deletingService(t)
	gqlSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema: gqlSchema, Context: context.Background(),
		RequestString: `mutation { createService(name: "web", image: "nginx") { name } }`,
	})
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %v, want none", res.Errors)
	}
	if _, live := liveAndDeleting(t, cl, "web"); len(live) != 1 {
		t.Fatalf("live Apps = %v, want the new service", live)
	}
}

// Once a create has taken a just-deleted name beside the old teardown, the
// live successor holds it: a second create of the name is refused, so two
// live services never share it (with the store off too, where no row
// uniqueness backs the check).
func TestASecondCreateBesideTheTeardownIsNameInUse(t *testing.T) {
	svc, cl := deletingService(t)
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "nginx:1"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	for _, dryRun := range []bool{true, false} {
		_, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "nginx:1", DryRun: dryRun})
		if err == nil || err.Error() != errServiceNameInUse("web").Error() {
			t.Fatalf("second create (dry run %v) = %v, want %v", dryRun, err, errServiceNameInUse("web"))
		}
	}
	if deleting, live := liveAndDeleting(t, cl, "web"); deleting != 1 || len(live) != 1 {
		t.Fatalf("web Apps: %d deleting, %d live; want one of each", deleting, len(live))
	}
}

// A live holder keeps the name-in-use refusal.
func TestCreateUnderALiveNameStaysNameInUse(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	_, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "nginx:1"})
	if err == nil || coded(t, err) != "CONFLICT" || err.Error() != errServiceNameInUse("web").Error() {
		t.Fatalf("create = %v, want %v", err, errServiceNameInUse("web"))
	}
}

// hiddenUntilCreate hides the App named hidden from Gets until the first
// Create: the create's name probe misses the holder, as in a race with its
// arrival, and only storage, checking the name last, meets it.
type hiddenUntilCreate struct {
	client.Client
	hidden string
	shown  atomic.Bool
	// dryRunOnly keeps hiding through dry runs, so the real write meets it.
	dryRunOnly bool
}

func (c *hiddenUntilCreate) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, app := obj.(*appv1alpha1.App); app && key.Name == c.hidden && !c.shown.Load() {
		return apierrors.NewNotFound(schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, key.Name)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *hiddenUntilCreate) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	var o client.CreateOptions
	o.ApplyOptions(opts)
	if !c.dryRunOnly || !slices.Contains(o.DryRun, metav1.DryRunAll) {
		c.shown.Store(true)
	}
	return c.Client.Create(ctx, obj, opts...)
}

// Storage's AlreadyExists — the dry-run's admission and the write's alike —
// for a holder the name probe missed in a race with its delete is retried
// once under a suffixed object name, so the create still succeeds.
func TestCreateWhoseStorageMeetsADeletedNameRetriesUnderItsOwnObject(t *testing.T) {
	for name, tc := range map[string]struct {
		dryRun     bool
		dryRunOnly bool
	}{
		"the plan's dry-run admission":   {},
		"a dry-run create":               {dryRun: true},
		"the write after a passing plan": {dryRunOnly: true},
	} {
		t.Run(name, func(t *testing.T) {
			svc, cl := deletingService(t)
			if tc.dryRunOnly {
				// The fake's dry run never meets a held name; the write does.
				svc.Client = &hiddenUntilCreate{Client: cl, hidden: "web", dryRunOnly: true}
			} else {
				svc.Client = &hiddenUntilCreate{Client: apiServerDryRuns{cl}, hidden: "web"}
			}
			if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "nginx:1", DryRun: tc.dryRun}); err != nil {
				t.Fatalf("create = %v, want it retried under its own object name", err)
			}
			if _, live := liveAndDeleting(t, cl, "web"); !tc.dryRun && (len(live) != 1 || !strings.HasPrefix(live[0].Name, "web-")) {
				t.Fatalf("live Apps = %v, want one web-xxxx", live)
			}
		})
	}
}
