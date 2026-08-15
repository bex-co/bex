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

package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type allowAllChecker struct{}

func (allowAllChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// TestRESTUnconfiguredStoreIs503 pins RegisterREST's documented contract
// ("Store unconfigured ⇒ the Service returns ErrJobsUnavailable ⇒ 503").
// It did not hold: the sentinel was a plain errors.New that core.WriteErr's
// hand-maintained unavailable list never named, so every job route answered
// 500 — a status that invites a client to retry a request which cannot succeed
// until an operator sets BEX_CP_DB_URI. Declaring it with core.Unavailable
// makes the documented behavior real.
//
// The App exists (so AuthorizeApp passes); only the control-plane store is
// absent, which is exactly the BEX_CP_DB_URI-unset deployment.
func TestRESTUnconfiguredStoreIs503(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "web:v1"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(client.Object(app)).Build()
	svc := &Service{Base: &core.Base{Authz: allowAllChecker{}, Client: cl, Namespace: "default"}} // Store nil

	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/v1/services/web/jobs"},
		{method: http.MethodGet, path: "/v1/services/web/jobs/job-1"},
		{method: http.MethodPost, path: "/v1/services/web/jobs/job-1/cancel"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(route.method, route.path, nil)
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
			mux.ServeHTTP(rec, req.WithContext(ctx))

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			if body["id"] != "unavailable" || body["message"] != ErrJobsUnavailable.Error() {
				t.Fatalf("REST body = %#v, want the Render unavailable envelope", body)
			}
		})
	}
}
