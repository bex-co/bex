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

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pitrNow is the fixed clock every w7/m453 test reads.
var pitrNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

const pitrDay = 24 * time.Hour

// pitrAgo is pitrNow minus d, as the API reports it.
func pitrAgo(d time.Duration) string { return pitrNow.Add(-d).Format(time.RFC3339) }

// pitrPlans is the tenants store's shape: the workspace plan and the recovery
// floor its last plan change kept, in one read (core.WorkspacePITRPlanReader).
type pitrPlans struct {
	plan        string
	visibleFrom time.Time
}

func (p pitrPlans) WorkspacePlan(context.Context, string) (string, error) { return p.plan, nil }

func (p pitrPlans) WorkspacePITRPlan(context.Context, string) (string, time.Time, error) {
	return p.plan, p.visibleFrom, nil
}

// pitrSource seeds a paid database of workspace tea-a whose ObjectStore window
// runs from first to last, behind plans (nil: no plan applies), a fixed clock
// and a payment gate that refuses and records every call.
func pitrSource(t *testing.T, plans core.WorkspacePlanReader, first, last string) (*Service, client.Client, *rejectingPaymentGate) {
	t.Helper()
	svc, cl := newServicePostgresResources()
	src := seedDatabaseSpec(t, cl, "dpg-src", appv1alpha1.DatabaseSpec{Name: "src", Plan: "basic-1gb", Version: "16"}, true)
	src.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelWorkspace: "tea-a"}
	if err := cl.Update(context.Background(), src); err != nil {
		t.Fatalf("label source: %v", err)
	}
	seedBarmanRecoveryWindow(t, cl, "dpg-src", first, last)
	svc.Plans = plans
	svc.Clock = func() time.Time { return pitrNow }
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	return svc, cl, gate
}

// recoveryInfoOnEverySurface reads dpg-src's recovery info on REST (Render's
// recoveryStatus/startsAt), GraphQL and MCP.
func recoveryInfoOnEverySurface(t *testing.T, svc *Service) (rest map[string]any, gql map[string]any, mcpOut map[string]any) {
	t.Helper()
	res := serveREST(svc, http.MethodGet, "/v1/postgres/dpg-src/recovery", "")
	if res.Code != http.StatusOK {
		t.Fatalf("REST recovery info: %d %s", res.Code, res.Body)
	}
	if err := json.Unmarshal(res.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ databaseRecoveryInfo(id:"dpg-src") { enabled earliestRecoveryTime latestRecoveryTime retention { plan days upgradePlan upgradeDays } } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL recovery info: %v", result.Errors)
	}
	// Through JSON, as a client reads it (graphql-go answers Go ints).
	encoded, _ := json.Marshal(result.Data.(map[string]any)["databaseRecoveryInfo"])
	if err := json.Unmarshal(encoded, &gql); err != nil {
		t.Fatal(err)
	}
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	mcpOut = call("get_postgres_recovery_info", map[string]any{"postgresId": "dpg-src"})
	return rest, gql, mcpOut
}

// w7/m453: the window every surface offers starts at the later of the
// ObjectStore's first recoverability point and the workspace plan's floor:
// now-3d on Hobby, now-7d on Pro, the kept floor after a Hobby -> Pro upgrade,
// a young database's own first point, and the ObjectStore's point alone where
// no plan applies. GraphQL and MCP carry the plan's window as a hint; REST
// keeps Render's two fields.
func TestRecoveryInfoStartsAtThePlanFloorOnEverySurface(t *testing.T) {
	keptAt := pitrNow.Add(-pitrDay) // a Hobby -> Pro upgrade a day ago keeps T-3d
	hobbyHint := map[string]any{"plan": "hobby", "days": float64(3), "upgradePlan": "pro", "upgradeDays": float64(7)}
	proHint := map[string]any{"plan": "pro", "days": float64(7), "upgradePlan": nil, "upgradeDays": nil}
	for _, tc := range []struct {
		name        string
		plans       core.WorkspacePlanReader
		first, want string
		hint        map[string]any
	}{
		{"hobby", pitrPlans{plan: "hobby"}, pitrAgo(30 * pitrDay), pitrAgo(3 * pitrDay), hobbyHint},
		{"pro", pitrPlans{plan: "pro"}, pitrAgo(30 * pitrDay), pitrAgo(7 * pitrDay), proHint},
		{"scale", pitrPlans{plan: "scale"}, pitrAgo(30 * pitrDay), pitrAgo(7 * pitrDay),
			map[string]any{"plan": "scale", "days": float64(7), "upgradePlan": nil, "upgradeDays": nil}},
		{"pro after a hobby upgrade", pitrPlans{plan: "pro", visibleFrom: keptAt.Add(-3 * pitrDay)}, pitrAgo(30 * pitrDay), pitrAgo(4 * pitrDay), proHint},
		{"young database on hobby", pitrPlans{plan: "hobby"}, pitrAgo(pitrDay), pitrAgo(pitrDay), hobbyHint},
		{"no plan applies", nil, pitrAgo(30 * pitrDay), pitrAgo(30 * pitrDay), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := pitrSource(t, tc.plans, tc.first, pitrAgo(9*time.Hour))
			rest, gql, mcpOut := recoveryInfoOnEverySurface(t, svc)
			if rest["recoveryStatus"] != "AVAILABLE" || rest["startsAt"] != tc.want {
				t.Errorf("REST = %v, want AVAILABLE from %s", rest, tc.want)
			}
			if len(rest) != 2 {
				t.Errorf("REST carries %v; Render's contract is recoveryStatus and startsAt alone", rest)
			}
			if gql["earliestRecoveryTime"] != tc.want || mcpOut["earliestRecoveryTime"] != tc.want {
				t.Errorf("GraphQL earliest = %v, MCP earliest = %v; want %s on both", gql["earliestRecoveryTime"], mcpOut["earliestRecoveryTime"], tc.want)
			}
			// The latest bound stays the last base backup (ADR009).
			if gql["latestRecoveryTime"] != pitrAgo(9*time.Hour) || mcpOut["latestRecoveryTime"] != pitrAgo(9*time.Hour) {
				t.Errorf("latest = %v / %v, want the last base backup", gql["latestRecoveryTime"], mcpOut["latestRecoveryTime"])
			}
			if tc.hint == nil {
				if gql["retention"] != nil || mcpOut["retention"] != nil {
					t.Errorf("retention = %v / %v, want none where no plan applies", gql["retention"], mcpOut["retention"])
				}
				return
			}
			if !reflect.DeepEqual(gql["retention"], tc.hint) {
				t.Errorf("GraphQL retention = %v, want %v", gql["retention"], tc.hint)
			}
			mcpHint := map[string]any{"upgradePlan": nil, "upgradeDays": nil}
			for k, v := range mcpOut["retention"].(map[string]any) {
				mcpHint[k] = v
			}
			if !reflect.DeepEqual(mcpHint, tc.hint) {
				t.Errorf("MCP retention = %v, want %v", mcpOut["retention"], tc.hint)
			}
		})
	}
}

// w7/m453: when the last base backup is older than the plan's floor (nightly
// backups stalled for longer than the window), no restore point inside the
// plan's window can be vouched for: recovery info reports no window
// (BACKUP_NOT_READY) and recover refuses before any billing gate or create.
// The same ObjectStore on Pro, whose floor is older, stays available.
func TestRecoveryWithBackupsStalledPastThePlanFloor(t *testing.T) {
	svc, cl, gate := pitrSource(t, pitrPlans{plan: "hobby"}, pitrAgo(30*pitrDay), pitrAgo(4*pitrDay))
	rest, gql, mcpOut := recoveryInfoOnEverySurface(t, svc)
	if rest["recoveryStatus"] != "BACKUP_NOT_READY" || rest["startsAt"] != nil {
		t.Errorf("REST = %v, want BACKUP_NOT_READY with no start", rest)
	}
	// GraphQL answers an empty string for no window, as it always has.
	if gql["earliestRecoveryTime"] != "" || gql["latestRecoveryTime"] != "" || mcpOut["earliestRecoveryTime"] != nil {
		t.Errorf("GraphQL = %v, MCP = %v; want no window", gql, mcpOut)
	}
	_, err := svc.Recover(context.Background(), "dpg-src", RecoverRequest{Name: "restored"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("recover with stalled backups => %v, want ErrBadRequest", err)
	}
	if len(gate.calls) != 0 || countDatabases(t, cl) != 1 {
		t.Fatalf("a refused recovery reached the payment gate (%v) or created a database (%d)", gate.calls, countDatabases(t, cl))
	}

	pro, _, _ := pitrSource(t, pitrPlans{plan: "pro"}, pitrAgo(30*pitrDay), pitrAgo(4*pitrDay))
	if rest, _, _ := recoveryInfoOnEverySurface(t, pro); rest["recoveryStatus"] != "AVAILABLE" || rest["startsAt"] != pitrAgo(7*pitrDay) {
		t.Errorf("the same backups on Pro = %v, want AVAILABLE from now-7d", rest)
	}
}

// recoverOnSurface asks for a recovery of dpg-src to target on one surface and
// answers the refusal's code and its params (REST params, GraphQL extensions;
// MCP carries the code alone).
func recoverOnSurface(t *testing.T, svc *Service, surface, target string) (code string, params map[string]any) {
	t.Helper()
	switch surface {
	case "REST":
		res := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-src/recovery", `{"restoreName":"restored","restoreTime":"`+target+`"}`)
		var body struct {
			Code   string         `json:"code"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		if res.Code != http.StatusBadRequest && body.Code != "" {
			t.Fatalf("REST refusal %s answered %d", body.Code, res.Code)
		}
		return body.Code, body.Params
	case "GraphQL":
		schema, err := pgGQLSchema(svc)
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			VariableValues: map[string]any{"t": target},
			RequestString:  `mutation($t:String) { recoverDatabase(id:"dpg-src", name:"restored", targetTime:$t) { id } }`})
		if len(res.Errors) != 1 {
			t.Fatalf("GraphQL recover errors = %v, want one", res.Errors)
		}
		code, _ := res.Errors[0].Extensions["code"].(string)
		return code, res.Errors[0].Extensions
	case "MCP":
		session, cleanup := pgMCPSession(t, svc)
		defer cleanup()
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "recover_postgres",
			Arguments: map[string]any{"postgresId": "dpg-src", "name": "restored", "targetTime": target}})
		if err != nil {
			t.Fatal(err)
		}
		return admissionMCPErrorCode(t, res), nil
	}
	t.Fatalf("unknown surface %q", surface)
	return "", nil
}

// w7/m453: a target before the plan's floored start is refused with
// RECOVERY_TARGET_BEFORE_WINDOW naming that start, and one within ten minutes
// of now with RECOVERY_TARGET_TOO_RECENT, on REST, GraphQL and MCP alike;
// neither reaches the payment gate or creates a database. A target inside the
// window, and no target at all (the latest point), go on to the billing gate.
func TestRecoverRefusesATargetOutsideThePlanWindowOnEverySurface(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			svc, cl, gate := pitrSource(t, pitrPlans{plan: "hobby"}, pitrAgo(30*pitrDay), pitrAgo(9*time.Hour))

			code, params := recoverOnSurface(t, svc, surface, pitrAgo(4*pitrDay))
			if code != "RECOVERY_TARGET_BEFORE_WINDOW" {
				t.Errorf("4 days ago => %q, want RECOVERY_TARGET_BEFORE_WINDOW", code)
			}
			if params != nil && params["earliestRecoveryTime"] != pitrAgo(3*pitrDay) {
				t.Errorf("4 days ago names %v, want the floored start %s", params["earliestRecoveryTime"], pitrAgo(3*pitrDay))
			}
			code, params = recoverOnSurface(t, svc, surface, pitrAgo(5*time.Minute))
			if code != "RECOVERY_TARGET_TOO_RECENT" {
				t.Errorf("5 minutes ago => %q, want RECOVERY_TARGET_TOO_RECENT", code)
			}
			if params != nil && params["latestTargetTime"] != pitrAgo(10*time.Minute) {
				t.Errorf("5 minutes ago names %v, want %s", params["latestTargetTime"], pitrAgo(10*time.Minute))
			}
			if len(gate.calls) != 0 || countDatabases(t, cl) != 1 {
				t.Fatalf("a refused target reached the payment gate (%v) or created a database (%d)", gate.calls, countDatabases(t, cl))
			}
		})
	}

	svc, _, gate := pitrSource(t, pitrPlans{plan: "hobby"}, pitrAgo(30*pitrDay), pitrAgo(9*time.Hour))
	for _, target := range []string{pitrAgo(3 * pitrDay), pitrAgo(2 * pitrDay), pitrAgo(10 * time.Minute), ""} {
		if _, err := svc.Recover(context.Background(), "dpg-src", RecoverRequest{Name: "restored", TargetTime: target}); !errors.Is(err, core.ErrPaymentRequired) {
			t.Errorf("target %q => %v, want it to pass the window and reach the payment gate", target, err)
		}
	}
	if len(gate.calls) != 4 {
		t.Errorf("payment gate calls = %v, want one per target inside the window", gate.calls)
	}
}

// The plan floor is reported to the second, rounded up, so a target at the
// reported start is never outside the plan's window by a fraction of a second.
func TestThePlanFloorRoundsUpToTheSecond(t *testing.T) {
	svc, _, _ := pitrSource(t, pitrPlans{plan: "hobby"}, pitrAgo(30*pitrDay), pitrAgo(9*time.Hour))
	svc.Clock = func() time.Time { return pitrNow.Add(400 * time.Millisecond) }
	info, err := svc.RecoveryInfo(context.Background(), "dpg-src")
	if err != nil {
		t.Fatal(err)
	}
	if want := pitrNow.Add(time.Second - 3*pitrDay).Format(time.RFC3339); info.EarliestRecoveryTime != want {
		t.Fatalf("earliest = %s, want %s", info.EarliestRecoveryTime, want)
	}
}
