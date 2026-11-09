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
	"encoding/json"
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
)

// w2/075: creating a service under the name of one just deleted, whose App
// is still being torn down (held by the operator's finalizer), is refused by
// name with a retry hint, never a 500 and never the live-holder wording.

const stillDeletingMsg = `a service named "web" was just deleted and its cleanup is still finishing; retry in a minute to create a service with this name`

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

func TestREST_CreateUnderAStillDeletingNameIsANamed409(t *testing.T) {
	svc, _ := deletingService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, body := range []string{
		`{"name":"web","image":{"imagePath":"nginx"}}`,
		`{"name":"web","image":{"imagePath":"nginx"},"dryRun":true}`,
	} {
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
		var out struct{ Code, Message string }
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if rec.Code != http.StatusConflict || out.Code != ServiceNameStillDeletingCode || out.Message != stillDeletingMsg {
			t.Fatalf("POST /v1/services %s = %d %s, want 409 %s", body, rec.Code, rec.Body, ServiceNameStillDeletingCode)
		}
	}
}

func TestGraphQL_CreateServiceUnderAStillDeletingNameIsCoded(t *testing.T) {
	svc, _ := deletingService(t)
	gqlSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema: gqlSchema, Context: context.Background(),
		RequestString: `mutation { createService(name: "web", image: "nginx") { id } }`,
	})
	if len(res.Errors) != 1 {
		t.Fatalf("errors = %v, want one", res.Errors)
	}
	if code, _ := res.Errors[0].Extensions["code"].(string); code != ServiceNameStillDeletingCode || res.Errors[0].Message != stillDeletingMsg {
		t.Fatalf("error = %q %v, want %s %q", res.Errors[0].Message, res.Errors[0].Extensions, ServiceNameStillDeletingCode, stillDeletingMsg)
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
// for a still-deleting holder is the named refusal, not a raw 500.
func TestCreateWhoseStorageMeetsAStillDeletingNameIsNamed(t *testing.T) {
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
			_, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "nginx:1", DryRun: tc.dryRun})
			if err == nil || coded(t, err) != ServiceNameStillDeletingCode || err.Error() != stillDeletingMsg {
				t.Fatalf("create = %v, want %s", err, ServiceNameStillDeletingCode)
			}
		})
	}
}
