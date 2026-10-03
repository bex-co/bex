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
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestStaticRouteMalformedDestinationWritesPreserveState(t *testing.T) {
	for _, destination := range []string{
		"/bad%", "/bad%2", "/ok?value=%zz", "/ok#bad%zz", "/%2foutside.example", "/%5coutside.example", "/nested/%5cfile", "/%00file", "/%1ffile", "/%7ffile", "/bad\tfile", "/ok?value=\\bad",
	} {
		for _, kind := range []string{"redirect", "rewrite"} {
			t.Run(kind+destination, func(t *testing.T) {
				existing := sampleStaticApp("site")
				svc, cl := newService(nil, existing)
				before := getApp(t, cl, "site")
				rules := []StaticRouteView{{Type: kind, Source: "/old/*", Destination: destination}}
				if _, err := svc.SetRoutes(t.Context(), "site", rules); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "destination") {
					t.Fatalf("SetRoutes malformed destination = %v", err)
				}
				after := getApp(t, cl, "site")
				if !reflect.DeepEqual(before, after) {
					t.Fatal("refused route update modified the existing App")
				}

				createSvc, createClient := newService(nil)
				if _, err := createSvc.Create(t.Context(), CreateRequest{Name: "new-site", Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/acme/site", PublishPath: "dist", Routes: rules}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "destination") {
					t.Fatalf("Create malformed destination = %v", err)
				}
				apps := &appv1alpha1.AppList{}
				if err := createClient.List(t.Context(), apps, client.InNamespace("default")); err != nil {
					t.Fatal(err)
				}
				if len(apps.Items) != 0 {
					t.Fatal("refused create persisted an App")
				}
			})
		}
	}
}
