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

package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRenderPostgresRecoveryAndCredentials(t *testing.T) {
	spec := loadRenderSpec(t)
	db := conformDatabase(ids.New(ids.Postgres))
	db.Spec.Name = "source"
	db.Spec.Plan = "basic-1gb"
	db.Status.BackupsEnabled = true
	db.Spec.Users = []appv1alpha1.DatabaseUser{{Name: "reader"}}
	objectStore := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "barmancloud.cnpg.io/v1", "kind": "ObjectStore",
		"metadata": map[string]any{"name": "bex-tenant-postgres", "namespace": "default"},
		"status": map[string]any{"serverRecoveryWindow": map[string]any{db.Name: map[string]any{
			"firstRecoverabilityPoint": "2026-09-01T00:00:00Z", "lastSuccessfulBackupTime": "2026-09-20T00:00:00Z",
		}}},
	}}
	cl := interceptor.NewClient(fakeClient(db, objectStore).(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			obj.SetCreationTimestamp(metav1.NewTime(conformEpoch))
			return cl.Create(ctx, obj, opts...)
		},
	})
	h, _ := serverWith(t, &core.Base{Client: cl, Namespace: "default"}, Deps{})
	path := "/v1/postgres/" + db.Name
	res := do(t, h, "GET", path+"/credentials", testToken, "")
	if res.Code != 200 {
		t.Fatalf("credentials: %d %s", res.Code, res.Body.String())
	}
	if errs := spec.validate("list-postgres-users", res.Body.Bytes()); len(errs) > 0 {
		t.Fatal(errs)
	}
	if !strings.Contains(res.Body.String(), `"username":"reader"`) || !strings.Contains(res.Body.String(), `"default":false`) || !strings.Contains(res.Body.String(), `"default":true`) {
		t.Fatalf("credentials = %s", res.Body.String())
	}
	res = do(t, h, "DELETE", path+"/credentials/reader", testToken, "")
	if res.Code != 200 {
		t.Fatalf("delete: %d %s", res.Code, res.Body.String())
	}
	if errs := spec.validateStatus("delete-postgres-user", res.Code, res.Body.Bytes()); len(errs) > 0 {
		t.Fatal(errs)
	}
	var persisted appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(db), &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted.Spec.Users) != 0 || len(persisted.Spec.DeletedUsers) != 1 {
		t.Fatalf("delete did not revoke reader: %+v", persisted.Spec)
	}
	res = do(t, h, "DELETE", path+"/credentials/"+db.Spec.EffectiveDatabaseUser(db.Name), testToken, "")
	if res.Code != 404 {
		t.Fatalf("owner credential deletion: %d %s", res.Code, res.Body.String())
	}

	res = do(t, h, "POST", path+"/recovery", testToken, `{"restoreName":"restored","restoreTime":"2026-09-10T00:00:00Z"}`)
	if res.Code != 200 {
		t.Fatalf("recover: %d %s", res.Code, res.Body.String())
	}
	// Recovery returns the same partial postgres detail as retrieve-postgres;
	// only those existing, documented metadata omissions are allowed.
	if errs := filterAllowed("retrieve-postgres", spec.validateStatus("recover-postgres", res.Code, res.Body.Bytes())); len(errs) > 0 {
		t.Fatal(errs)
	}
	var recovered struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &recovered); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: recovered.ID}, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Spec.Recovery == nil || persisted.Spec.Recovery.SourceDatabase != db.Name || persisted.Spec.Recovery.TargetTime != "2026-09-10T00:00:00Z" {
		t.Fatalf("wrong recovery intent: %+v", persisted.Spec.Recovery)
	}
	for _, field := range []string{"environmentId", "datadogApiKey", "datadogSite"} {
		body := `{"restoreName":"unsupported","` + field + `":"value"}`
		res = do(t, h, "POST", path+"/recovery", testToken, body)
		if res.Code != 400 || !strings.Contains(res.Body.String(), field) {
			t.Fatalf("%s: %d %s", field, res.Code, res.Body.String())
		}
	}
}
