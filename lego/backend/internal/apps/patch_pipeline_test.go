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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func serveRESTPatch(t *testing.T, svc *Service, name, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/"+name, strings.NewReader(body)))
	return rec
}

func TestRESTPatchEmptyBodyIsReadOnlyNoOp(t *testing.T) {
	for _, body := range []string{`{}`, `{"serviceDetails":{}}`} {
		svc, cl := newService(nil, sampleApp("web"))
		before := getApp(t, cl, "web").ResourceVersion
		rec := serveRESTPatch(t, svc, "web", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %s => 200 (no-op read), got %d: %s", body, rec.Code, rec.Body)
		}
		if after := getApp(t, cl, "web").ResourceVersion; after != before {
			t.Errorf("PATCH %s must not write: resourceVersion %s -> %s", body, before, after)
		}
	}
}

func TestRESTPatchNestedHealthCheckPathWinsOverTopLevel(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	body := `{"healthCheckPath":"/top","serviceDetails":{"healthCheckPath":"/nested"}}`
	rec := serveRESTPatch(t, svc, "web", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/nested" {
		t.Errorf("spec.healthCheckPath = %q, want /nested (nested spelling wins)", got)
	}
}

func TestRESTPatchNestedPreDeployCommandWinsOverTopLevel(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	body := `{"preDeployCommand":"top.sh","serviceDetails":{"preDeployCommand":"nested.sh"}}`
	rec := serveRESTPatch(t, svc, "web", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "nested.sh" {
		t.Errorf("spec.preDeployCommand = %q, want nested.sh (nested spelling wins)", got)
	}
}

func TestRESTPatchNestedScheduleWinsAndKeepsTopLevelCommand(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	// The nested schedule wins, while the top-level command still applies in
	// the same coalesced SetCronJob call.
	body := `{"schedule":"0 5 * * *","command":"run.py","serviceDetails":{"schedule":"0 7 * * *"}}`
	rec := serveRESTPatch(t, svc, "nightly", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH => 200, got %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Schedule != "0 7 * * *" || a.Spec.Command != "run.py" {
		t.Errorf("spec schedule=%q command=%q, want 0 7 * * * / run.py", a.Spec.Schedule, a.Spec.Command)
	}
}

func TestDecodeIPAllowListAbsentAndNullMeanNotProvided(t *testing.T) {
	for _, raw := range []string{"", "null"} {
		entries, ok, err := decodeIPAllowList(context.Background(), []byte(raw))
		if err != nil || ok || entries != nil {
			t.Errorf("decodeIPAllowList(%q) = %v, %v, %v; want nil, false, nil", raw, entries, ok, err)
		}
	}
}

func TestDecodeIPAllowListEmptyArrayMeansReplaceWithEmpty(t *testing.T) {
	entries, ok, err := decodeIPAllowList(context.Background(), []byte(`[]`))
	if err != nil || !ok || len(entries) != 0 {
		t.Errorf("decodeIPAllowList([]) = %v, %v, %v; want provided-and-empty", entries, ok, err)
	}
}

func TestDecodeIPAllowListDecodesEntries(t *testing.T) {
	entries, ok, err := decodeIPAllowList(context.Background(), []byte(`[{"cidrBlock":"192.0.2.0/24","description":"office"}]`))
	if err != nil || !ok {
		t.Fatalf("decodeIPAllowList: ok=%v err=%v", ok, err)
	}
	if len(entries) != 1 || entries[0].CIDRBlock != "192.0.2.0/24" || entries[0].Description != "office" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestDecodeIPAllowListMalformedIsBadRequest(t *testing.T) {
	_, _, err := decodeIPAllowList(context.Background(), []byte(`{"not":"a list"}`))
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "ipAllowList") {
		t.Errorf("malformed payload => named ErrBadRequest, got %v", err)
	}
}
