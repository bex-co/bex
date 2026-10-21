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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Render's staticSiteDetailsPOST.publishPath "Defaults to \"public\"": the REST
// create fills it on omission or blank, leaves an explicit value alone, and
// does not default for other types (w8/153).
func TestRESTCreateDefaultsStaticPublishPath(t *testing.T) {
	svc, _ := newService(nil)
	for _, tc := range []struct{ name, body, want string }{
		{"omitted", `{"name":"s","type":"static_site","repo":"https://github.com/x/y"}`, "public"},
		{"blank", `{"name":"s","type":"static_site","repo":"https://github.com/x/y","serviceDetails":{"publishPath":"  "}}`, "public"},
		{"explicit", `{"name":"s","type":"static_site","repo":"https://github.com/x/y","serviceDetails":{"publishPath":"dist"}}`, "dist"},
		{"web service", `{"name":"s","type":"web_service","repo":"https://github.com/x/y"}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(core.WithStrictJSONDecoding(t.Context()), http.MethodPost, "/v1/services", strings.NewReader(tc.body))
			req, err := svc.decodeCreateService(r)
			if err != nil {
				t.Fatal(err)
			}
			if req.PublishPath != tc.want {
				t.Fatalf("PublishPath = %q, want %q", req.PublishPath, tc.want)
			}
		})
	}
}
