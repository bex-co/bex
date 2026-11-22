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
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w8/385: the stall write times the port scan the deploy log narrates. The
// first no-open-port stall stamps its start, a stall between two scans keeps
// it, the first stall that says the scan is over stamps its end, and nothing
// after that moves either.
func TestPGStallReasonTimesThePortScan(t *testing.T) {
	ctx := context.Background()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("port-scan-%d", time.Now().UnixNano()), PlanHobby, "test-port-scan")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.DeleteTenant(ctx, ten.ID) }()
	app, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "busybox:1.36", Type: "web_service",
		Branch: "main", Port: 10000, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployUpdateInProgress, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("start rollout = (%v, %v)", ok, err)
	}
	scan := func() (since, until *time.Time) {
		t.Helper()
		d, err := st.GetDeploy(ctx, app.ID, app.FirstDeployID)
		if err != nil {
			t.Fatal(err)
		}
		return d.PortScanSince, d.PortScanUntil
	}
	stall := func(reason string) {
		t.Helper()
		if err := st.SetDeployStallReason(ctx, app.FirstDeployID, reason); err != nil {
			t.Fatal(err)
		}
	}

	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortWaiting))
	if since, until := scan(); since != nil || until != nil {
		t.Fatalf("waiting to start: scan = (%v, %v), want none", since, until)
	}
	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet))
	first, until := scan()
	if first == nil || until != nil {
		t.Fatalf("no open port: scan = (%v, %v), want started and running", first, until)
	}
	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortDetecting))
	stall(appv1alpha1.OpenPortProbeStallReason("startup", 0))
	if since, until := scan(); since == nil || !since.Equal(*first) || until != nil {
		t.Fatalf("still scanning: scan = (%v, %v), want (%v, nil)", since, until, first)
	}
	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortDetected,
		appv1alpha1.DeployReasonParam{Name: "port", Value: "8080"}))
	_, end := scan()
	if end == nil || end.Before(*first) {
		t.Fatalf("port found: scan ended at %v, want after %v", end, first)
	}
	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet))
	stall("")
	if since, until := scan(); !since.Equal(*first) || !until.Equal(*end) {
		t.Fatalf("after the scan: scan = (%v, %v), want (%v, %v) kept", since, until, first, end)
	}
}
