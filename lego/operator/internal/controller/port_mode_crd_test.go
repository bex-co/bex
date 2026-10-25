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

package controller

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCRDPortModeEnumCoversEveryPortMode guards w8/200: a port mode the
// backend writes but the App CRD enum omits makes the API server reject every
// App that uses it (w4/m243 shipped web-image-v1 that way and broke every
// prebuilt-image web service create).
func TestCRDPortModeEnumCoversEveryPortMode(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "crd", "bases", "app.bex.co_apps.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(raw, &crd); err != nil {
		t.Fatal(err)
	}
	var enum []string
	for _, v := range crd.Spec.Versions {
		if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			continue
		}
		mode := v.Schema.OpenAPIV3Schema.Properties["spec"].Properties["portMode"]
		for _, e := range mode.Enum {
			var s string
			if err := yaml.Unmarshal(e.Raw, &s); err != nil {
				t.Fatal(err)
			}
			enum = append(enum, s)
		}
	}
	if len(enum) == 0 {
		t.Fatal("App CRD spec.portMode has no enum")
	}
	for _, mode := range []string{
		"configured-v1",
		appv1alpha1.PortModeImageV1,
		appv1alpha1.PortModeImageConfiguredV1,
		appv1alpha1.PortModeWebImageV1,
		appv1alpha1.PortModeWebImageConfiguredV1,
	} {
		if !slices.Contains(enum, mode) {
			t.Errorf("App CRD spec.portMode enum %v lacks %q; regenerate with `make manifests`", enum, mode)
		}
	}
}
