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

// conformance_test.go — TestRenderConformance validates bex's live REST
// responses against the complete embedded Render OpenAPI document shared with
// the production request gate.
//
// Validation errors that match a conformanceAllowlist entry (allowlist.go) are
// suppressed with an ADR018 citation; everything else fails the build. A new
// divergence without an allowlist entry turns the CI job red at PR time.
//
// The deliberate pin-refresh workflow, provenance checks, and compatibility
// review are documented in docs/render-artifacts/openapi-request-validation.md.
// For each new response failure, either fix bex or add a narrow ADR018-cited
// conformanceAllowlist entry; the stale-entry test prevents permanent excuses.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// conformEpoch is a fixed timestamp used in all conformance fixtures so the
// tests are deterministic and the rendered timestamps are verifiable by eye.
var conformEpoch = time.Unix(1_700_000_000, 0).UTC()

// --- fake stores for conformance fixtures ------------------------------------

// conformDeployStore is a read-only deploy store backed by a pre-seeded slice.
// Only ListDeploys and GetDeploy are needed for the conformance suite; the rest
// return errors so a missing call is immediately visible.
type conformDeployStore struct {
	byApp map[string][]store.Deploy
}

func (s *conformDeployStore) CreateDeploy(_ context.Context, _, _, _ string, _ int64, _ store.CommitInfo, _ string, _ ...bool) (store.Deploy, error) {
	return store.Deploy{}, errors.New("conformDeployStore: CreateDeploy not expected in conformance tests")
}
func (s *conformDeployStore) LatestDeployCommit(_ context.Context, _ string) (store.CommitInfo, error) {
	return store.CommitInfo{}, errors.New("conformDeployStore: LatestDeployCommit not expected")
}
func (s *conformDeployStore) CreateRestartDeploy(_ context.Context, _, _ string, _ int64, _ store.CommitInfo, _ string) (store.Deploy, error) {
	return store.Deploy{}, errors.New("conformDeployStore: CreateRestartDeploy not expected")
}
func (s *conformDeployStore) CreateRollbackDeploy(_ context.Context, _, _, _ string, _ int64, _ store.CommitInfo, _ string) (store.Deploy, error) {
	return store.Deploy{}, errors.New("conformDeployStore: CreateRollbackDeploy not expected")
}
func (s *conformDeployStore) ListDeploys(_ context.Context, appID string, _ store.DeployFilter) ([]store.Deploy, error) {
	return s.byApp[appID], nil
}
func (s *conformDeployStore) GetDeploy(_ context.Context, appID, deployID string) (store.Deploy, error) {
	for _, d := range s.byApp[appID] {
		if d.ID == deployID {
			return d, nil
		}
	}
	return store.Deploy{}, core.ErrNotFound
}
func (s *conformDeployStore) DeployIDByGeneration(_ context.Context, appID string, generation int64) (string, error) {
	for _, d := range s.byApp[appID] {
		if d.Generation == generation {
			return d.ID, nil
		}
	}
	return "", nil
}
func (s *conformDeployStore) CloseDeploy(_ context.Context, _, _, _ string) (bool, error) {
	return false, errors.New("conformDeployStore: CloseDeploy not expected")
}
func (s *conformDeployStore) SetAppImage(_ context.Context, _, _ string) error {
	return errors.New("conformDeployStore: SetAppImage not expected")
}
func (s *conformDeployStore) InsertServiceEventFact(_ context.Context, _ store.ServiceEventFact) (bool, error) {
	return false, errors.New("conformDeployStore: InsertServiceEventFact not expected")
}

// conformEventStore returns a pre-seeded slice of service event rows.
type conformEventStore struct {
	rows []store.ServiceEventRow
}

func (s *conformEventStore) ListServiceEvents(_ context.Context, _, _ string, _ store.ServiceEventFilter) ([]store.ServiceEventRow, error) {
	return s.rows, nil
}

// ServiceEventWorkspaces records no routing: conformance runs in one workspace.
func (s *conformEventStore) ServiceEventWorkspaces(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *conformEventStore) GetServiceEvent(_ context.Context, workspaceID, eventID string) (store.ServiceEventLookup, error) {
	if workspaceID == core.DefaultTenant {
		for _, row := range s.rows {
			if ids.Derive(ids.Event, row.Key) == eventID {
				return store.ServiceEventLookup{Event: row, ServiceID: "web"}, nil
			}
		}
	}
	return store.ServiceEventLookup{}, store.ErrNotFound
}

// --- fixture helpers ---------------------------------------------------------

// conformApp creates an App CR with a creation timestamp and a store-level app-id
// label so the deploys and events services can retrieve history for it.
func conformApp(name, appID string) *appv1alpha1.App {
	labels := map[string]string{}
	if appID != "" {
		labels[store.LabelAppID] = appID
	}
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(conformEpoch),
			Labels:            labels,
		},
		Spec:   appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 1},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://" + name + ".onbex.co"},
	}
}

// conformAppWithDomains creates an App CR that has custom domains in Spec.Hosts.
func conformAppWithDomains(name string, domains ...string) *appv1alpha1.App {
	a := conformApp(name, "")
	a.Spec.Hosts = domains
	return a
}

// conformDatabase creates a Database CR with a creation timestamp.
func conformDatabase(name string) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(conformEpoch),
		},
		Spec:   appv1alpha1.DatabaseSpec{Plan: "free"},
		Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	}
}

// conformKeyValue creates a KeyValue CR with a creation timestamp.
func conformKeyValue(name string) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(conformEpoch),
		},
		Spec:   appv1alpha1.KeyValueSpec{Plan: "free"},
		Status: appv1alpha1.KeyValueStatus{Phase: appv1alpha1.KVPhaseReady},
	}
}

// conformDatastoreSecret is the connection Secret the connection-info endpoints
// read: CNPG's "<db>-app" for Postgres, the operator's KeyValue-named one for
// Key Value. Without it the handlers stop at core.ErrNotFound and the
// connection-info conformance cases never reach their schema.
func conformDatastoreSecret(name string, data map[string]string) *corev1.Secret {
	b := make(map[string][]byte, len(data))
	for k, v := range data {
		b[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data:       b,
	}
}

// conformDeploy creates a canned deploy record for conformance fixtures.
func conformDeploy(id, appID string) store.Deploy {
	finished := conformEpoch.Add(2 * time.Minute)
	return store.Deploy{
		ID:         id,
		AppID:      appID,
		Trigger:    store.TriggerAPI,
		Image:      "web:v1",
		Status:     store.DeployLive,
		CreatedAt:  conformEpoch,
		FinishedAt: &finished,
	}
}

// --- main conformance test ---------------------------------------------------

// TestRenderConformance validates that bex's live REST responses for the core
// endpoint families match the pinned Render OpenAPI response schemas.
// Deliberate divergences are silenced only via conformanceAllowlist entries in
// conformance_allowlist_test.go, each citing its ADR018 row.
func TestRenderConformance(t *testing.T) {
	spec := loadRenderSpec(t)

	const (
		appName   = "web"
		appID     = "srv-web"
		deployID  = "dep-1"
		dbName    = "pg1"
		kvName    = "kv1"
		customFQN = "api.example.com"
	)

	// Seed a secret KV store (used for env-vars + secret-files conformance).
	secretStore := &fakeAuditKV{m: map[string]map[string]string{
		"services/" + appName + "/env":   {"API_KEY": "val1", "DB_URL": "val2"},
		"services/" + appName + "/files": {"ca.pem": "cert", "db.pem": "cert2"},
	}}
	// One deploy per bex trigger value, so list-deploys proves every one of
	// them serializes inside Render's closed trigger enum (w8/058) — a new
	// store.Trigger* fails here until renderTrigger maps it.
	deploys := []store.Deploy{conformDeploy(deployID, appID)}
	for i, trigger := range store.AllTriggers {
		d := conformDeploy(fmt.Sprintf("dep-t%d", i), appID)
		d.Trigger = trigger
		deploys = append(deploys, d)
	}
	deployStore := &conformDeployStore{byApp: map[string][]store.Deploy{appID: deploys}}
	eventStore := &conformEventStore{rows: []store.ServiceEventRow{
		{
			Key:      deployID + ":" + store.EventPhaseStarted,
			At:       conformEpoch,
			Source:   store.EventSourceDeploy,
			Phase:    store.EventPhaseStarted,
			DeployID: deployID,
			Trigger:  store.TriggerAPI,
		},
	}}
	eventID := ids.Derive(ids.Event, eventStore.rows[0].Key)

	h, _ := serverWith(t,
		&core.Base{
			Client: fakeClient(
				conformApp(appName, appID),
				conformAppWithDomains("web-cd", customFQN),
				conformDatabase(dbName),
				conformKeyValue(kvName),
				conformDatastoreSecret(dbName+"-app", map[string]string{
					"username": "u", "password": "p", "dbname": "d",
					"uri": "postgresql://u:p@" + dbName + "-rw.default:5432/d",
				}),
				conformDatastoreSecret(kvName, map[string]string{
					"uri": "redis://default:p@" + kvName + ".default:6379",
				}),
			),
			Namespace: "default",
			Clock:     func() time.Time { return conformEpoch },
		},
		Deps{
			ResourceMetrics: func(context.Context, string, string) ([]metrics.PodResourceUsage, error) { return nil, nil },
			Secrets:         secretStore,
			DeployStore:     deployStore,
			EventStore:      eventStore,
			APIKeys:         newFakeKeyStore(),
		},
	)

	// check makes a GET request, ensures 200, validates against the Render schema,
	// and filters out allowlisted divergences before failing.
	check := func(t *testing.T, path, operationID string) {
		t.Helper()
		w := do(t, h, "GET", path, testToken, "")
		if w.Code != 200 {
			t.Fatalf("GET %s => %d (want 200): %s", path, w.Code, w.Body.String())
		}
		errs := filterAllowed(operationID, spec.validate(operationID, w.Body.Bytes()))
		if len(errs) > 0 {
			t.Errorf(
				"Render schema violation(s) for %s (operationId=%s).\n"+
					"To silence: add an entry to conformanceAllowlist citing its ADR018 row.\n"+
					"To fix: update the bex response to match Render's schema.\n"+
					"Violations:\n  %s",
				path, operationID, strings.Join(errs, "\n  "),
			)
		}
	}

	t.Run("services/list", func(t *testing.T) {
		check(t, "/v1/services", "list-services")
	})

	t.Run("services/get", func(t *testing.T) {
		check(t, "/v1/services/"+appName, "retrieve-service")
	})

	t.Run("deploys/list", func(t *testing.T) {
		check(t, "/v1/services/"+appName+"/deploys", "list-deploys")
	})

	t.Run("deploys/get", func(t *testing.T) {
		check(t, "/v1/services/"+appName+"/deploys/"+deployID, "retrieve-deploy")
	})

	t.Run("postgres/list", func(t *testing.T) {
		// bex returns a flat []PostgresView; Render expects [{postgres:{},cursor}].
		// The mismatch is in the allowlist (ADR018 §Postgres REST).
		check(t, "/v1/postgres", "list-postgres")
	})

	t.Run("postgres/get", func(t *testing.T) {
		check(t, "/v1/postgres/"+dbName, "retrieve-postgres")
	})

	t.Run("postgres/recovery", func(t *testing.T) {
		check(t, "/v1/postgres/"+dbName+"/recovery", "retrieve-postgres-recovery-info")
	})
	t.Run("postgres/credentials", func(t *testing.T) {
		check(t, "/v1/postgres/"+dbName+"/credentials", "list-postgres-users")
	})

	t.Run("keyvalue/list", func(t *testing.T) {
		check(t, "/v1/key-value", "list-key-value")
	})

	t.Run("keyvalue/get", func(t *testing.T) {
		check(t, "/v1/key-value/"+kvName, "retrieve-key-value")
	})

	// Render's deprecated /redis spelling of the same Key Value (w3/m274):
	// the list envelope is keyed "redis" and connection info names its CLI
	// command redisCLICommand, each validated against the redis* schemas.
	t.Run("redis/list", func(t *testing.T) {
		check(t, "/v1/redis", "list-redis")
	})

	t.Run("redis/get", func(t *testing.T) {
		check(t, "/v1/redis/"+kvName, "retrieve-redis")
	})

	t.Run("redis/connection-info", func(t *testing.T) {
		check(t, "/v1/redis/"+kvName+"/connection-info", "retrieve-redis-connection-info")
	})

	// The write half of the /redis family, on a store it creates: create
	// answers 201 and patch 200 in redisDetail, suspend/resume 202 and delete
	// 204, as the pinned spec documents.
	t.Run("redis/writes", func(t *testing.T) {
		// create and update answer redisDetail, retrieve-redis's own schema, so
		// its allowlist entries are the ones that apply. The fake client never
		// stamps creationTimestamp (the API server does), so a missing
		// createdAt here is the fixture's, not bex's.
		validate := func(operationID string, status int, body []byte) {
			t.Helper()
			errs := slices.DeleteFunc(filterAllowed("retrieve-redis", spec.validateStatus(operationID, status, body)), func(e string) bool {
				return e == `/createdAt: property "createdAt" is missing`
			})
			if len(errs) > 0 {
				t.Errorf("%s diverges from Render's schema:\n  %s", operationID, strings.Join(errs, "\n  "))
			}
		}
		w := do(t, h, "POST", "/v1/redis", testToken, `{"name":"legacy","ownerId":"`+core.DefaultTenant+`","plan":"starter"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("POST /v1/redis = %d (want 201): %s", w.Code, w.Body.String())
		}
		validate("create-redis", http.StatusCreated, w.Body.Bytes())
		var created struct{ ID string }
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == "" {
			t.Fatalf("create-redis body has no id: %s", w.Body.String())
		}
		item := "/v1/redis/" + created.ID

		w = do(t, h, "PATCH", item, testToken, `{"maxmemoryPolicy":"noeviction"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("PATCH %s = %d (want 200): %s", item, w.Code, w.Body.String())
		}
		validate("update-redis", http.StatusOK, w.Body.Bytes())

		for _, verb := range []string{"suspend", "resume"} {
			if w = do(t, h, "POST", item+"/"+verb, testToken, ""); w.Code != http.StatusAccepted {
				t.Fatalf("POST %s/%s = %d (want 202): %s", item, verb, w.Code, w.Body.String())
			}
		}
		if w = do(t, h, "DELETE", item, testToken, ""); w.Code != http.StatusNoContent {
			t.Fatalf("DELETE %s = %d (want 204): %s", item, w.Code, w.Body.String())
		}
	})

	// The connection-info endpoints are separate schemas behind separate
	// operations — outside the postgres/keyValue resource sweep above, which is
	// how w6/m109/t008's missing required externalConnectionString survived
	// t001-t006. Validated here so the whole required-field class is gated, not
	// just the one field that was found live.
	t.Run("postgres/connection-info", func(t *testing.T) {
		check(t, "/v1/postgres/"+dbName+"/connection-info", "retrieve-postgres-connection-info")
	})

	t.Run("keyvalue/connection-info", func(t *testing.T) {
		check(t, "/v1/key-value/"+kvName+"/connection-info", "retrieve-key-value-connection-info")
	})

	t.Run("env-vars/list", func(t *testing.T) {
		check(t, "/v1/services/"+appName+"/env-vars", "get-env-vars-for-service")
	})

	t.Run("secret-files/list", func(t *testing.T) {
		// The paged secret-files list (w9/m5) is Render's env-vars twin: the
		// {secretFile,cursor} envelope with each file's content (w8/182) —
		// validated with no allowlist entry.
		check(t, "/v1/services/"+appName+"/secret-files", "list-secret-files-for-service")
	})

	t.Run("custom-domains/list", func(t *testing.T) {
		check(t, "/v1/services/web-cd/custom-domains", "list-custom-domains")
	})

	t.Run("events/list", func(t *testing.T) {
		check(t, "/v1/services/"+appName+"/events", "list-events")
	})

	t.Run("events/get", func(t *testing.T) {
		check(t, "/v1/events/"+eventID, "retrieve-event")
	})

	// Render's replace-all secret-file PUT (w8/063). Runs last: it replaces the
	// seeded ca.pem/db.pem the list subtest above reads.
	t.Run("secret-files/replace-all", func(t *testing.T) {
		w := do(t, h, "PUT", "/v1/services/"+appName+"/secret-files", testToken,
			`[{"name":"b.txt","content":"two"},{"name":"a.txt","content":"one"}]`)
		if w.Code != http.StatusOK {
			t.Fatalf("PUT secret-files = %d: %s", w.Code, w.Body.String())
		}
		if errs := filterAllowed("update-secret-files-for-service", spec.validate("update-secret-files-for-service", w.Body.Bytes())); len(errs) > 0 {
			t.Errorf("update-secret-files-for-service diverges: %v", errs)
		}
		w = do(t, h, "GET", "/v1/services/"+appName+"/secret-files", testToken, "")
		var listed []struct {
			SecretFile struct{ Name string } `json:"secretFile"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil || len(listed) != 2 ||
			listed[0].SecretFile.Name != "a.txt" || listed[1].SecretFile.Name != "b.txt" {
			t.Fatalf("after replace-all, list = %s, want exactly a.txt and b.txt (seeded files gone)", w.Body.String())
		}
	})
}

// TestConformanceAllowlistEntries verifies that every entry in conformanceAllowlist
// actually suppresses a real validation error from the live conformance suite —
// a removed allowlist entry that was never real (or that bex fixed) would otherwise
// silently pass, giving false confidence that the suite still covers that path.
//
// The test re-runs the conformance check WITHOUT filtering, then confirms each
// allowlist entry's `contains` string appears in the raw error set for its
// operation. An entry that no longer matches means bex now conforms on that
// point and the entry should be removed.
func TestConformanceAllowlistEntries(t *testing.T) {
	spec := loadRenderSpec(t)

	const (
		appName = "web"
		dbName  = "pg1"
		kvName  = "kv1"
	)

	secretStore := &fakeAuditKV{m: map[string]map[string]string{
		"services/" + appName + "/env":   {"K": "V"},
		"services/" + appName + "/files": {"ca.pem": "cert"},
	}}
	deployStore := &conformDeployStore{byApp: map[string][]store.Deploy{
		"srv-web": {conformDeploy("dep-1", "srv-web")},
	}}
	eventStore := &conformEventStore{rows: []store.ServiceEventRow{
		{Key: "dep-1:" + store.EventPhaseStarted, At: conformEpoch, Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerAPI},
	}}
	eventID := ids.Derive(ids.Event, eventStore.rows[0].Key)

	h, _ := serverWith(t,
		&core.Base{
			Client: fakeClient(
				conformApp(appName, "srv-web"),
				conformAppWithDomains("web-cd", "api.example.com"),
				conformDatabase(dbName),
				conformKeyValue(kvName),
			),
			Namespace: "default",
			Clock:     func() time.Time { return conformEpoch },
		},
		Deps{
			ResourceMetrics: func(context.Context, string, string) ([]metrics.PodResourceUsage, error) { return nil, nil },
			Secrets:         secretStore,
			DeployStore:     deployStore,
			EventStore:      eventStore,
			APIKeys:         newFakeKeyStore(),
		},
	)

	// operationPath maps an operationId to the bex REST path under test.
	operationPath := map[string]string{
		"list-services":                 "/v1/services",
		"retrieve-service":              "/v1/services/" + appName,
		"list-postgres":                 "/v1/postgres",
		"retrieve-postgres":             "/v1/postgres/" + dbName,
		"retrieve-key-value":            "/v1/key-value/" + kvName,
		"retrieve-redis":                "/v1/redis/" + kvName,
		"list-key-value":                "/v1/key-value",
		"list-secret-files-for-service": "/v1/services/" + appName + "/secret-files",
		"list-custom-domains":           "/v1/services/web-cd/custom-domains",
		"list-events":                   "/v1/services/" + appName + "/events",
		"retrieve-event":                "/v1/events/" + eventID,
		"list-redis":                    "/v1/redis",
	}

	for opID, divergences := range conformanceAllowlist {
		path, ok := operationPath[opID]
		if !ok {
			t.Errorf("allowlist entry %q has no operationPath mapping — add one to TestConformanceAllowlistEntries", opID)
			continue
		}
		t.Run(opID, func(t *testing.T) {
			w := do(t, h, "GET", path, testToken, "")
			if w.Code != 200 {
				t.Fatalf("GET %s => %d: %s", path, w.Code, w.Body.String())
			}
			rawErrs := spec.validate(opID, w.Body.Bytes())
			for _, div := range divergences {
				found := false
				for _, e := range rawErrs {
					if strings.Contains(e, div.contains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("allowlist entry %q (contains=%q) did not match any raw validation error — bex may now conform on this point; remove the entry.\nRaw errors: %v",
						div.adr018, div.contains, rawErrs)
				}
			}
		})
	}
}
