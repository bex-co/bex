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

package store

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// w4/m709: the operator verb is the control-plane API's, behind its bearer,
// and answers each refusal with a 400 that names the rule.
func TestEgressAttackWindowVerbOverTheControlPlaneAPI(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ws, err := s.CreateTenant(ctx, "flooded", "pro")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Hour)
	putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, now.Add(-6*time.Hour), 1)
	api := &API{Store: newMemStore(), Token: "cp-secret", AttackWindows: s}
	h := api.Handler()
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer cp-secret")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	base := "/v1/tenants/" + ws.ID + "/egress-attack-windows"
	rfc := func(t time.Time) string { return t.Format(time.RFC3339) }
	start, end := now.Add(-5*time.Hour), now.Add(-2*time.Hour)

	if rr := do(t, h, http.MethodGet, base, ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("without the bearer = %d, want 401", rr.Code)
	}
	for name, tc := range map[string]struct {
		body string
		code int
		says string
	}{
		"not RFC3339":       {`{"startsAt":"yesterday","endsAt":"` + rfc(end) + `","actor":"ops","reason":"flood"}`, 400, "startsAt must be RFC3339"},
		"start after end":   {`{"startsAt":"` + rfc(end) + `","endsAt":"` + rfc(start) + `","actor":"ops","reason":"flood"}`, 400, "startsAt must be before endsAt"},
		"no reason":         {`{"startsAt":"` + rfc(start) + `","endsAt":"` + rfc(end) + `","actor":"ops"}`, 400, "actor and reason are required"},
		"foreign service":   {`{"startsAt":"` + rfc(start) + `","endsAt":"` + rfc(end) + `","serviceIds":["srv-other"],"actor":"ops","reason":"flood"}`, 400, "srv-other is not a service"},
		"over the 7 days":   {`{"startsAt":"` + rfc(now.Add(-9*24*time.Hour)) + `","endsAt":"` + rfc(end) + `","actor":"ops","reason":"flood"}`, 400, "at most"},
		"not a JSON object": {`[]`, 400, "bad request body"},
	} {
		rr := call(http.MethodPost, base, tc.body)
		if rr.Code != tc.code || !strings.Contains(rr.Body.String(), tc.says) {
			t.Errorf("%s: %d %s, want %d naming %q", name, rr.Code, rr.Body.String(), tc.code, tc.says)
		}
	}
	if rr := call(http.MethodPost, "/v1/tenants/tea-missing/egress-attack-windows", `{"startsAt":"`+rfc(start)+`","endsAt":"`+rfc(end)+`","actor":"ops","reason":"flood"}`); rr.Code != http.StatusNotFound {
		t.Errorf("unknown workspace = %d %s, want 404", rr.Code, rr.Body.String())
	}

	rr := call(http.MethodPost, base, `{"startsAt":"`+rfc(start)+`","endsAt":"`+rfc(end)+`","serviceIds":["srv-web"],"actor":"ops@bex.co","reason":"Hetzner DDoS notice"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("declare = %d %s", rr.Code, rr.Body.String())
	}
	var declared EgressAttackWindow
	if err := json.Unmarshal(rr.Body.Bytes(), &declared); err != nil || !strings.HasPrefix(declared.ID, "eaw-") || declared.Actor != "ops@bex.co" {
		t.Fatalf("declared = %+v, %v", declared, err)
	}

	if rr := call(http.MethodPatch, base+"/"+declared.ID, `{"endsAt":"`+rfc(now.Add(-time.Hour))+`","serviceIds":["srv-web"],"actor":"ops","reason":"wider"}`); rr.Code != 400 || !strings.Contains(rr.Body.String(), "cannot change serviceIds") {
		t.Errorf("extension changing services = %d %s, want 400", rr.Code, rr.Body.String())
	}
	if rr := call(http.MethodPatch, base+"/"+declared.ID, `{"endsAt":"`+rfc(now.Add(-time.Hour))+`","actor":"oncall","reason":"attack went on"}`); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"action":"extend"`) {
		t.Fatalf("extend = %d %s", rr.Code, rr.Body.String())
	}
	if rr := call(http.MethodDelete, base+"/eaw-missing", `{"actor":"ops","reason":"x"}`); rr.Code != http.StatusNotFound {
		t.Errorf("withdrawing an unknown window = %d, want 404", rr.Code)
	}
	if rr := call(http.MethodDelete, base+"/"+declared.ID, `{"actor":"ops","reason":"customer's load test"}`); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"withdrawnBy":"ops"`) {
		t.Fatalf("withdraw = %d %s", rr.Code, rr.Body.String())
	}

	rr = call(http.MethodGet, base, "")
	var list struct {
		Windows []EgressAttackWindow `json:"windows"`
	}
	if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &list) != nil || len(list.Windows) != 1 {
		t.Fatalf("list = %d %s", rr.Code, rr.Body.String())
	}
	got := list.Windows[0]
	if got.WithdrawnAt == nil || len(got.Changes) != 3 || got.Changes[0].Reason != "Hetzner DDoS notice" || got.Changes[1].Actor != "oncall" || got.Changes[2].Action != "withdraw" {
		t.Errorf("listed window = %+v, want each change with its actor and reason", got)
	}

	unwired := (&API{Store: newMemStore()}).Handler()
	if rr := do(t, unwired, http.MethodGet, base, ""); rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "unavailable") {
		t.Errorf("unwired verb = %d %s", rr.Code, rr.Body.String())
	}
}
