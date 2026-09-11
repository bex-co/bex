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
	"os"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/jackc/pgx/v5/pgxpool"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type productCertificateClient struct {
	client.Client
	calls int
	err   error
}

func (c *productCertificateClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.calls++
	if c.err != nil {
		return c.err
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func TestPGObserveProductAppHostingAndCertificates(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	workspace, err := st.CreateWorkspace(ctx, "product-sampler-"+ids.New(ids.Owner), store.PlanHobby, "test-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, workspace.ID) })

	for _, tc := range []struct {
		name                            string
		stale, suspended, notReady      bool
		pending, missing, lookupFailure bool
		wantLive, wantTLS               bool
	}{
		{name: "ready", wantLive: true, wantTLS: true},
		{name: "stale-ready", stale: true, wantTLS: true},
		{name: "suspended", suspended: true, wantTLS: true},
		{name: "not-ready", notReady: true, wantTLS: true},
		{name: "unverified", pending: true, wantLive: true},
		{name: "missing-certificate", missing: true, wantLive: true},
		{name: "lookup-unavailable", lookupFailure: true, wantLive: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, err := st.CreateApp(ctx, store.App{TenantID: workspace.ID, Name: tc.name, Image: "nginx", Tier: "free", Port: 80, Replicas: 1})
			if err != nil {
				t.Fatal(err)
			}
			domain, err := st.CreateDomain(ctx, row.ID, tc.name+"."+workspace.ID+".test", false)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.pending {
				if _, err := pool.Exec(ctx, "UPDATE domains SET claim_state='verified',verified_at=now() WHERE id=$1", domain.ID); err != nil {
					t.Fatal(err)
				}
			} else if _, err := pool.Exec(ctx, "UPDATE domains SET claim_state='pending',verified_at=NULL,challenge='test-proof' WHERE id=$1", domain.ID); err != nil {
				t.Fatal(err)
			}
			alias, err := st.CreateDomain(ctx, row.ID, "www."+domain.Host, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, "UPDATE domains SET redirect_for_name=$2,claim_state='verified',verified_at=now() WHERE id=$1", alias.ID, domain.Host); err != nil {
				t.Fatal(err)
			}
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: workspace.ID, Generation: 2}}
			app.Spec.Suspended = tc.suspended
			app.Status.Phase = appv1alpha1.PhaseRunning
			app.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, ObservedGeneration: 2}}
			if tc.stale {
				app.Status.Conditions[0].ObservedGeneration = 1
			}
			if tc.notReady {
				app.Status.Conditions[0].Status = metav1.ConditionFalse
			}
			cl := &productCertificateClient{Client: fakeClient()}
			if !tc.missing {
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretForHost(app, domain.Host), Namespace: workspace.ID},
					Data: map[string][]byte{"tls.crt": []byte("certificate-evidence")}}
				if err := cl.Create(ctx, secret); err != nil {
					t.Fatal(err)
				}
			}
			old := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			if tc.lookupFailure {
				cl.err = errors.New("API unavailable")
				if err := st.RecordProductDomainTLS(ctx, domain.ID, true, old); err != nil {
					t.Fatal(err)
				}
			}
			desired := store.DesiredApp{App: row}
			ObserveProductApp(ctx, st, cl, desired, app)
			var live, wasLive bool
			if err := pool.QueryRow(ctx, "SELECT live,was_live FROM product_hosting_daily WHERE resource_id=$1", row.ID).Scan(&live, &wasLive); err != nil || live != tc.wantLive || wasLive != tc.wantLive {
				t.Fatalf("live=%v wasLive=%v want=%v err=%v", live, wasLive, tc.wantLive, err)
			}
			var observedAt time.Time
			var tlsReady bool
			if err := pool.QueryRow(ctx, "SELECT observed_at,tls_ready FROM product_domain_observations WHERE domain_id=$1", domain.ID).Scan(&observedAt, &tlsReady); err != nil {
				t.Fatal(err)
			}
			if tc.lookupFailure {
				if !tlsReady || !observedAt.Equal(old) {
					t.Fatal("transient error overwrote previous certificate evidence")
				}
			} else if tlsReady != tc.wantTLS || !observedAt.After(old) {
				t.Fatalf("TLS=%v want=%v observed=%s", tlsReady, tc.wantTLS, observedAt)
			}
			wantCalls := 1
			if tc.pending {
				wantCalls = 0
			}
			if cl.calls != wantCalls {
				t.Fatalf("certificate lookups=%d want=%d (aliases/unverified must be skipped)", cl.calls, wantCalls)
			}
			var aliasSamples int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM product_domain_observations WHERE domain_id=$1", alias.ID).Scan(&aliasSamples); err != nil || aliasSamples != 0 {
				t.Fatalf("alias samples=%d err=%v", aliasSamples, err)
			}
			ObserveProductApp(ctx, st, cl, desired, app)
			if cl.calls != wantCalls {
				t.Fatal("unchanged hosting repeated certificate work")
			}
		})
	}
}
