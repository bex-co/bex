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

// w8/388: the first finding a running scan reports is recorded with its
// moment, read with the scan from every deploy read, and never moves: a later
// finding, or one after the scan ended, leaves it. A scan whose first stall
// names a finding finds it at its start, from one clock reading.
func TestPGStallReasonRecordsThePortScansFinding(t *testing.T) {
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
	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("port-scan-found-%d", time.Now().UnixNano()), PlanHobby, "test-port-scan-found")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.DeleteTenant(ctx, ten.ID) }()
	rollout := func(name string) (stall func(string), read func() Deploy) {
		t.Helper()
		app, err := st.CreateApp(ctx, App{
			TenantID: ten.ID, Name: name, Image: "busybox:1.36", Type: "web_service",
			Branch: "main", Port: 10000, Replicas: 1, Tier: "starter",
		})
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployUpdateInProgress, RanImage{}, "", "", "", nil); err != nil || !ok {
			t.Fatalf("start rollout = (%v, %v)", ok, err)
		}
		stall = func(reason string) {
			t.Helper()
			if err := st.SetDeployStallReason(ctx, app.FirstDeployID, reason); err != nil {
				t.Fatal(err)
			}
		}
		read = func() Deploy {
			t.Helper()
			d, err := st.GetDeploy(ctx, app.ID, app.FirstDeployID)
			if err != nil {
				t.Fatal(err)
			}
			return d
		}
		return stall, read
	}
	ports := appv1alpha1.DeployReasonParam{Name: "ports", Value: "10000"}
	localhost := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortLocalhostOnly, ports)
	noHTTP := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneHTTP, ports)

	stall, read := rollout("plain-first")
	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet))
	if d := read(); d.PortScanSince == nil || d.PortScanFound != "" || d.PortScanFoundAt != nil {
		t.Fatalf("no open port: scan from %v found %q at %v, want started with no finding", d.PortScanSince, d.PortScanFound, d.PortScanFoundAt)
	}
	stall(localhost)
	first := read()
	if first.PortScanFound != appv1alpha1.PortScanFoundLocalhost || first.PortScanFoundAt == nil || first.PortScanFoundAt.Before(*first.PortScanSince) {
		t.Fatalf("localhost: found %q at %v, want localhost at or after %v", first.PortScanFound, first.PortScanFoundAt, first.PortScanSince)
	}
	stall(noHTTP)
	stall(appv1alpha1.OpenPortProbeStallReason("startup", 0))
	if d := read(); d.PortScanFound != appv1alpha1.PortScanFoundLocalhost || !d.PortScanFoundAt.Equal(*first.PortScanFoundAt) {
		t.Fatalf("a later finding: found %q at %v, want localhost at %v kept", d.PortScanFound, d.PortScanFoundAt, first.PortScanFoundAt)
	}

	stall, read = rollout("found-first")
	stall(noHTTP)
	if d := read(); d.PortScanFound != appv1alpha1.PortScanFoundNoHTTP || d.PortScanSince == nil || d.PortScanFoundAt == nil || !d.PortScanFoundAt.Equal(*d.PortScanSince) {
		t.Fatalf("a scan that starts with a finding: since %v, found %q at %v, want no_http at the scan's start", d.PortScanSince, d.PortScanFound, d.PortScanFoundAt)
	}

	stall, read = rollout("after-the-scan")
	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet))
	stall(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortDetected, appv1alpha1.DeployReasonParam{Name: "port", Value: "8080"}))
	stall(localhost)
	if d := read(); d.PortScanUntil == nil || d.PortScanFound != "" || d.PortScanFoundAt != nil {
		t.Fatalf("a finding after the scan ended: until %v, found %q at %v, want none recorded", d.PortScanUntil, d.PortScanFound, d.PortScanFoundAt)
	}
}
