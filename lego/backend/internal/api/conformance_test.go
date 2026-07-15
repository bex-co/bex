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
// responses against the pinned Render OpenAPI schemas in
// testdata/render-openapi.json.
//
// Validation errors that match a conformanceAllowlist entry (allowlist.go) are
// suppressed with an ADR018 citation; everything else fails the build. A new
// divergence without an allowlist entry turns the CI job red at PR time.
//
// Pin-update workflow:
//  1. Replace testdata/render-openapi.json with a fresh copy from
//     render-public-api-1.json (full spec or the response-schema subset for the
//     operationIds below).
//  2. Run: cd lego/backend && go test ./internal/api/... -run TestRenderConformance -v
//  3. For each new failure, decide keep-or-fix:
//     - Keep (bex intentionally diverges): add an entry to conformanceAllowlist
//       citing its ADR018 row.
//     - Fix: update bex's response to match Render's schema, remove the entry.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

func (s *conformDeployStore) CreateDeploy(_ context.Context, _, _, _ string, _ int64, _ store.CommitInfo) (store.Deploy, error) {
	return store.Deploy{}, errors.New("conformDeployStore: CreateDeploy not expected in conformance tests")
}
func (s *conformDeployStore) CreateRollbackDeploy(_ context.Context, _, _, _ string, _ store.CommitInfo) (store.Deploy, error) {
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
func (s *conformDeployStore) CloseDeploy(_ context.Context, _, _, _ string) (bool, error) {
	return false, errors.New("conformDeployStore: CloseDeploy not expected")
}
func (s *conformDeployStore) SetAppImage(_ context.Context, _, _ string) error {
	return errors.New("conformDeployStore: SetAppImage not expected")
}

// conformEventStore returns a pre-seeded slice of service event rows.
type conformEventStore struct {
	rows []store.ServiceEventRow
}

func (s *conformEventStore) ListServiceEvents(_ context.Context, _, _, _ string, _ store.ServiceEventFilter) ([]store.ServiceEventRow, error) {
	return s.rows, nil
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

	// Seed a secret KV store (used for env-vars conformance).
	secretStore := &fakeAuditKV{m: map[string]map[string]string{
		"services/" + appName + "/env": {"API_KEY": "val1", "DB_URL": "val2"},
	}}
	deployStore := &conformDeployStore{byApp: map[string][]store.Deploy{
		appID: {conformDeploy(deployID, appID)},
	}}
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

	h, _ := serverWith(t,
		&core.Base{
			Client: fakeClient(
				conformApp(appName, appID),
				conformAppWithDomains("web-cd", customFQN),
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
		check(t, "/v1/postgres", "list-postgres-databases")
	})

	t.Run("postgres/get", func(t *testing.T) {
		check(t, "/v1/postgres/"+dbName, "retrieve-postgres")
	})

	t.Run("keyvalue/list", func(t *testing.T) {
		// bex returns a flat []KeyValueView; Render expects [{redis:{},cursor}].
		// The mismatch is in the allowlist (ADR018 §Key Value REST).
		check(t, "/v1/key-value", "list-redis")
	})

	t.Run("keyvalue/get", func(t *testing.T) {
		check(t, "/v1/key-value/"+kvName, "retrieve-redis")
	})

	t.Run("env-vars/list", func(t *testing.T) {
		check(t, "/v1/services/"+appName+"/env-vars", "retrieve-env-vars-for-service")
	})

	t.Run("custom-domains/list", func(t *testing.T) {
		check(t, "/v1/services/web-cd/custom-domains", "list-custom-domains")
	})

	t.Run("events/list", func(t *testing.T) {
		check(t, "/v1/services/"+appName+"/events", "list-events")
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
		"services/" + appName + "/env": {"K": "V"},
	}}
	deployStore := &conformDeployStore{byApp: map[string][]store.Deploy{
		"srv-web": {conformDeploy("dep-1", "srv-web")},
	}}
	eventStore := &conformEventStore{rows: []store.ServiceEventRow{
		{Key: "dep-1:" + store.EventPhaseStarted, At: conformEpoch, Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerAPI},
	}}

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(conformApp(appName, "srv-web"), conformDatabase(dbName), conformKeyValue(kvName)),
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
		"list-postgres-databases": "/v1/postgres",
		"list-redis":              "/v1/key-value",
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
