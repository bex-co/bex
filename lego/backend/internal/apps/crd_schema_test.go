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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	openapierrors "k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// crdBasesDir holds the CRDs the operator module generates from lego/types.
// The backend never imports the operator, so its tests read them as files.
var crdBasesDir = filepath.Join("..", "..", "..", "operator", "config", "crd", "bases")

// crdValidatingClient is fakeClient with the API server's schema check: an
// App, Database or KeyValue created, updated or merge-patched through it, a
// dry run included, is checked against its CRD's openAPIV3Schema and refused
// the way the API server refuses it (Invalid, one cause per field). fakeClient
// checks no schema, so TestRenderExamplesCorpus read pg_pinecone_render's
// 4,717-byte dockerCommand as valid while production's dry run refused it
// against the CRD's former 4096-byte cap (w6/183). The CRD's CEL rules
// (x-kubernetes-validations) are not evaluated.
func crdValidatingClient(t testing.TB) client.Client {
	t.Helper()
	validators := crdSchemaValidators(t)
	check := func(c client.WithWatch, obj client.Object) error {
		gvk, err := apiutil.GVKForObject(obj, c.Scheme())
		if err != nil {
			return err
		}
		validator, ok := validators[gvk.GroupKind()]
		if !ok {
			return nil
		}
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return err
		}
		content["apiVersion"], content["kind"] = gvk.GroupVersion().String(), gvk.Kind
		// A write to the main resource leaves status alone (the status
		// subresource owns it), so the API server does not validate it there.
		delete(content, "status")
		if errs := schemaFieldErrors(validator.Validate(content)); len(errs) > 0 {
			return apierrors.NewInvalid(gvk.GroupKind(), obj.GetName(), errs)
		}
		return nil
	}
	return fakeClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if err := check(c, obj); err != nil {
				return err
			}
			return rendertest.StampOnCreate.Create(ctx, c, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if err := check(c, obj); err != nil {
				return err
			}
			return c.Update(ctx, obj, opts...)
		},
		// The object a client.MergeFrom patch carries is the whole
		// patched object, which is what the API server validates.
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if err := check(c, obj); err != nil {
				return err
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	}).Build()
}

// crdSchemaValidators reads the storage version's openAPIV3Schema of every
// CRD in crdBasesDir, keyed by the kind it serves.
func crdSchemaValidators(t testing.TB) map[schema.GroupKind]*validate.SchemaValidator {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(crdBasesDir, "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no CRDs in %s: %v", crdBasesDir, err)
	}
	validators := map[schema.GroupKind]*validate.SchemaValidator{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var crd struct {
			Spec struct {
				Group string `json:"group"`
				Names struct {
					Kind string `json:"kind"`
				} `json:"names"`
				Versions []struct {
					Storage bool `json:"storage"`
					Schema  struct {
						OpenAPIV3Schema json.RawMessage `json:"openAPIV3Schema"`
					} `json:"schema"`
				} `json:"versions"`
			} `json:"spec"`
		}
		if err := sigsyaml.Unmarshal(raw, &crd); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, v := range crd.Spec.Versions {
			if !v.Storage {
				continue
			}
			var s spec.Schema
			if err := json.Unmarshal(v.Schema.OpenAPIV3Schema, &s); err != nil {
				t.Fatalf("%s: openAPIV3Schema: %v", f, err)
			}
			validators[schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind}] =
				validate.NewSchemaValidator(&s, nil, "", strfmt.Default)
		}
	}
	return validators
}

// schemaFieldErrors words a schema validation result as the API server's
// apiextensions validation does for the codes bex's CRDs can fail.
func schemaFieldErrors(result *validate.Result) field.ErrorList {
	var errs field.ErrorList
	for _, err := range result.Errors {
		v, ok := err.(*openapierrors.Validation)
		if !ok {
			errs = append(errs, field.Invalid(nil, "", err.Error()))
			continue
		}
		path := field.NewPath(strings.TrimPrefix(v.Name, "."))
		switch v.Code() {
		case openapierrors.RequiredFailCode:
			errs = append(errs, field.Required(path, ""))
		case openapierrors.EnumFailCode:
			values := make([]string, 0, len(v.Values))
			for _, allowed := range v.Values {
				values = append(values, fmt.Sprint(allowed))
			}
			errs = append(errs, field.NotSupported(path, v.Value, values))
		case openapierrors.TooLongFailCode:
			maxLength, _ := v.Valid.(int64)
			errs = append(errs, field.TooLong(path, "", int(maxLength)))
		case openapierrors.MaxItemsFailCode:
			actual, _ := v.Value.(int64)
			maxItems, _ := v.Valid.(int64)
			errs = append(errs, field.TooMany(path, int(actual), int(maxItems)))
		default:
			errs = append(errs, field.Invalid(path, v.Value, v.Error()))
		}
	}
	return errs
}
