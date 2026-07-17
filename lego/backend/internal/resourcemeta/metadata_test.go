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

package resourcemeta

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type recordingResolver struct {
	calls int
	ids   []string
}

func (r *recordingResolver) ResolveResourceOwners(_ context.Context, ids []string) map[string]Owner {
	r.calls++
	r.ids = append([]string(nil), ids...)
	return map[string]Owner{"tea-a": {ID: "tea-a", Name: "acme", Type: "team"}}
}

func TestResolveOwnersBatchesUniqueNonEmptyIDs(t *testing.T) {
	r := &recordingResolver{}
	got := ResolveOwners(context.Background(), r, []string{"tea-a", "", "tea-a", "tea-b"})
	if r.calls != 1 || len(r.ids) != 2 || r.ids[0] != "tea-a" || r.ids[1] != "tea-b" {
		t.Fatalf("resolver calls=%d ids=%v", r.calls, r.ids)
	}
	if got["tea-a"].Name != "acme" {
		t.Fatalf("resolved = %#v", got)
	}
}

func TestConfigAndTimestampOmissionRules(t *testing.T) {
	configured := Config{Region: " fsn1 ", DashboardBaseURL: "https://dashboard.bex.co/root?discard=yes"}
	if got := configured.PlatformRegion(); got != "fsn1" {
		t.Fatalf("region = %q", got)
	}
	if got := configured.DashboardURL("services", "srv-one"); got != "https://dashboard.bex.co/root/services/srv-one" {
		t.Fatalf("dashboard URL = %q", got)
	}
	if got := (Config{DashboardBaseURL: "/relative"}).DashboardURL("services", "srv-one"); got != "" {
		t.Fatalf("relative dashboard URL = %q", got)
	}

	obj := &metav1.PartialObjectMetadata{}
	obj.SetCreationTimestamp(metav1.NewTime(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)))
	if got := UpdatedAt(obj); got != "" {
		t.Fatalf("legacy timestamp copied creation time: %q", got)
	}
	Touch(obj, time.Date(2026, 7, 15, 13, 0, 0, 123, time.UTC))
	if got := UpdatedAt(obj); got != "2026-07-15T13:00:00.000000123Z" {
		t.Fatalf("touched timestamp = %q", got)
	}
	managed := metav1.NewTime(time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC))
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "operator", Time: &managed}})
	if got := UpdatedAt(obj); got != "2026-07-15T14:00:00Z" {
		t.Fatalf("managed-field timestamp = %q", got)
	}
}

// Render's dashboard URL scheme (docs/render-artifacts/dashboard-routes.md):
// type-aware service segments with the CLI's `web` fallback, `/d/` for
// Postgres, `/r/` for Key Value.
func TestServiceDashboardRouteMatchesRenderScheme(t *testing.T) {
	cases := map[string]string{
		"web_service":       "web",
		"private_service":   "pserv",
		"background_worker": "worker",
		"cron_job":          "cron",
		"static_site":       "static",
		"":                  "web", // unknown/unset falls back like Render's CLI
		"someday_new_type":  "web",
	}
	for serviceType, want := range cases {
		if got := ServiceDashboardRoute(serviceType); got != want {
			t.Errorf("ServiceDashboardRoute(%q) = %q, want %q", serviceType, got, want)
		}
	}
	c := Config{DashboardBaseURL: "https://dashboard.bex.co"}
	if got := c.DashboardURL(ServiceDashboardRoute("cron_job"), "srv-x"); got != "https://dashboard.bex.co/cron/srv-x" {
		t.Errorf("service URL = %q", got)
	}
	if got := c.DashboardURL(PostgresDashboardRoute, "dpg-x"); got != "https://dashboard.bex.co/d/dpg-x" {
		t.Errorf("postgres URL = %q", got)
	}
	if got := c.DashboardURL(KeyValueDashboardRoute, "red-x"); got != "https://dashboard.bex.co/r/red-x" {
		t.Errorf("key-value URL = %q", got)
	}
}
