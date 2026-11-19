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

package managedoidc

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/oidcissuer"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	wsA  = "tea-d4ahtvtpdjhs73eclslg"
	evmA = "evm-d4ahtvtpdjhs73eclsm0"
	srvA = "srv-d4ahtvtpdjhs73eclsmg"
	srvB = "srv-d4ahtvtpdjhs73eclsn0"
	bdsA = "bds-d4ahtvtpdjhs73eclsn0"
)

// keys is an in-memory key store: the issuer's key set, without a database.
type keys struct {
	mu   sync.Mutex
	now  func() time.Time
	rows []store.OIDCSigningKey
}

func (k *keys) OIDCSigningKeys(context.Context) ([]store.OIDCSigningKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]store.OIDCSigningKey(nil), k.rows...), nil
}

func (k *keys) BootstrapOIDCSigningKey(_ context.Context, kid string, der []byte) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.rows) > 0 {
		return false, nil
	}
	k.rows = append(k.rows, store.OIDCSigningKey{KID: kid, PrivateKey: der, CreatedAt: k.now(), ActivatesAt: k.now()})
	return true, nil
}

func (k *keys) DeleteOIDCSigningKeys(context.Context, []string) error { return nil }

// clock is the test's time, shared by the issuer and the minter.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fixture is an issuer served on a real listener, so a token the minter
// writes verifies with go-oidc through discovery alone, as AWS STS does.
type fixture struct {
	clock  *clock
	keys   *oidcissuer.KeySet
	issuer *oidcissuer.Issuer
	base   string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{clock: &clock{t: time.Now().Truncate(time.Second)}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.issuer.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	f.base = srv.URL
	f.keys = &oidcissuer.KeySet{Store: &keys{now: f.clock.Now}, Now: f.clock.Now}
	issuer, err := oidcissuer.New(srv.URL, f.keys)
	if err != nil {
		t.Fatal(err)
	}
	f.issuer = issuer
	if err := f.keys.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return f
}

// reload is the issuer's minute reload, so the set never reads as stale
// while the test's clock jumps ahead.
func (f *fixture) reload(t *testing.T) {
	t.Helper()
	if err := f.keys.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

// optedIn is a Pro service whose running release mounts the AWS token.
func optedIn(name, service string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: wsA, UID: types.UID("uid-" + name),
			Labels: map[string]string{
				appv1alpha1.LabelManagedOIDC: appv1alpha1.ManagedOIDCEnabled,
				core.LabelTenant:             wsA,
				core.LabelEnvironment:        evmA,
				core.LabelAppID:              service,
			},
		},
		Status: appv1alpha1.AppStatus{ManagedOIDC: &appv1alpha1.ManagedOIDCStatus{Generation: 3, Providers: []string{"aws"}, PlanAllowed: true}},
	}
}

func tokenSecret(t *testing.T, cl client.Client, app string) (*corev1.Secret, bool) {
	t.Helper()
	var s corev1.Secret
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: wsA, Name: appv1alpha1.ManagedOIDCSecretName(app)}, &s)
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return &s, true
}

func issued(t *testing.T, s *corev1.Secret) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s.Annotations[annotationIssuedAt], 10, 64)
	if err != nil {
		t.Fatalf("issued-at annotation %q: %v", s.Annotations[annotationIssuedAt], err)
	}
	return n
}

// TestAPassWritesTheTokenAWSAcceptsAndReplacesItBeforeThirtyMinutes is the
// definition of done's token: in the App's Secret, under the file name the
// pods read, a bex-signed token that verifies through the workspace's
// discovery with aud sts.amazonaws.com, Render's sub and a 60-minute life;
// replaced at 20 minutes, never later than 30.
func TestAPassWritesTheTokenAWSAcceptsAndReplacesItBeforeThirtyMinutes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(optedIn("web", srvA)).Build()
	m := NewMinter(cl, f.issuer)
	m.Now = f.clock.Now
	if err := m.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	s, ok := tokenSecret(t, cl, "web")
	if !ok {
		t.Fatal("no token Secret after the first pass")
	}
	if len(s.Data) != 1 || len(s.Data["aws-token"]) == 0 {
		t.Fatalf("Secret keys = %v, want exactly aws-token", s.Data)
	}
	if s.Labels[appv1alpha1.LabelProtectedFromTenantMount] != appv1alpha1.ProtectedFromTenantMount {
		t.Error("the token Secret is not protected from another service's mount")
	}
	if len(s.OwnerReferences) != 1 || s.OwnerReferences[0].UID != "uid-web" || s.OwnerReferences[0].Controller != nil {
		t.Errorf("owner references = %+v, want the App as a non-controller owner, so deleting the service deletes its tokens", s.OwnerReferences)
	}

	p, err := oidc.NewProvider(ctx, f.base+"/"+wsA)
	if err != nil {
		t.Fatal(err)
	}
	idt, err := p.Verifier(&oidc.Config{ClientID: "sts.amazonaws.com", SupportedSigningAlgs: []string{oidc.ES256}, Now: f.clock.Now}).
		Verify(ctx, string(s.Data["aws-token"]))
	if err != nil {
		t.Fatalf("the written token does not verify through its workspace's discovery: %v", err)
	}
	wantSub := "workspace:" + wsA + ":environment:" + evmA + ":service:" + srvA
	if idt.Subject != wantSub || idt.Expiry.Sub(idt.IssuedAt) != time.Hour {
		t.Fatalf("token sub %q, lifetime %v; want %q, 1h", idt.Subject, idt.Expiry.Sub(idt.IssuedAt), wantSub)
	}
	first := issued(t, s)

	// A pass before RefreshAfter writes nothing; the one at RefreshAfter
	// replaces the token, well before 30 minutes.
	f.clock.advance(RefreshAfter - time.Second)
	f.reload(t)
	if err := m.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if s, _ := tokenSecret(t, cl, "web"); issued(t, s) != first {
		t.Fatal("a token younger than RefreshAfter was replaced")
	}
	f.clock.advance(time.Second)
	f.reload(t)
	if err := m.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	s, _ = tokenSecret(t, cl, "web")
	if got := issued(t, s); got-first != int64(RefreshAfter/time.Second) || RefreshAfter >= 30*time.Minute {
		t.Fatalf("refreshed at +%ds; want +%ds, under 30 minutes", got-first, int64(RefreshAfter/time.Second))
	}
}

// TestAServiceWithSeveralProvidersGetsOneTokenPerAudience (w2/m478, ADR116
// D10): a release that mounts AWS and then also Anthropic and OpenAI gets a
// token per provider, each verifying for its own audience only, all for the
// same workspace issuer and subject; a provider added at a deploy is minted
// at once whatever the age of the others, and one dropped at a deploy loses
// only its own file.
func TestAServiceWithSeveralProvidersGetsOneTokenPerAudience(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	app := optedIn("web", srvA)
	cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(app).WithStatusSubresource(app).Build()
	m := NewMinter(cl, f.issuer)
	m.Now = f.clock.Now
	gcpInputs := appv1alpha1.ManagedOIDCGCP{WorkloadIdentityProvider: gcpProvider}
	audienceOf := func(p appv1alpha1.ManagedOIDCProvider) string {
		if p.Name == appv1alpha1.ManagedOIDCGCPName {
			return gcpInputs.Audience()
		}
		return p.Audience
	}
	setProviders := func(names ...string) {
		t.Helper()
		var cur appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &cur); err != nil {
			t.Fatal(err)
		}
		cur.Status.ManagedOIDC.Providers = names
		cur.Status.ManagedOIDC.GCP = &gcpInputs
		cur.Status.ManagedOIDC.Generation++
		if err := cl.Status().Update(ctx, &cur); err != nil {
			t.Fatal(err)
		}
	}
	pass := func() *corev1.Secret {
		t.Helper()
		if err := m.Pass(ctx); err != nil {
			t.Fatal(err)
		}
		s, ok := tokenSecret(t, cl, "web")
		if !ok {
			t.Fatal("no token Secret")
		}
		return s
	}
	keysOf := func(s *corev1.Secret) []string { return slices.Sorted(maps.Keys(s.Data)) }

	pass()
	f.clock.advance(RefreshAfter / 2)
	f.reload(t)
	setProviders("aws", "gcp", "anthropic", "openai")
	s := pass()
	if got, want := keysOf(s), []string{"anthropic-token", "aws-token", "gcp-credentials.json", "gcp-token", "openai-token"}; !slices.Equal(got, want) {
		t.Fatalf("a provider added at a deploy: Secret keys = %v, want %v at once", got, want)
	}

	p, err := oidc.NewProvider(ctx, f.base+"/"+wsA)
	if err != nil {
		t.Fatal(err)
	}
	verify := func(raw []byte, audience string) (*oidc.IDToken, error) {
		return p.Verifier(&oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.ES256}, Now: f.clock.Now}).Verify(ctx, string(raw))
	}
	wantSub := "workspace:" + wsA + ":environment:" + evmA + ":service:" + srvA
	for _, own := range appv1alpha1.ManagedOIDCProviders {
		tok := s.Data[own.File]
		idt, err := verify(tok, audienceOf(own))
		if err != nil {
			t.Fatalf("the %s token does not verify for %s: %v", own.Name, audienceOf(own), err)
		}
		if idt.Subject != wantSub || idt.Issuer != f.base+"/"+wsA {
			t.Errorf("the %s token: iss %q sub %q, want the workspace's issuer and %q", own.Name, idt.Issuer, idt.Subject, wantSub)
		}
		for _, other := range appv1alpha1.ManagedOIDCProviders {
			if other.Name == own.Name {
				continue
			}
			if _, err := verify(tok, audienceOf(other)); err == nil {
				t.Errorf("the %s token verifies where %s's audience %s is expected", own.Name, other.Name, audienceOf(other))
			}
		}
	}

	setProviders("aws", "openai")
	s = pass()
	if got, want := keysOf(s), []string{"aws-token", "openai-token"}; !slices.Equal(got, want) {
		t.Fatalf("a provider dropped at a deploy: Secret keys = %v, want %v", got, want)
	}
	for _, name := range []string{"aws", "openai"} {
		own, _ := appv1alpha1.ManagedOIDCProviderNamed(name)
		if _, err := verify(s.Data[own.File], own.Audience); err != nil {
			t.Errorf("the %s token left after dropping Anthropic does not verify: %v", name, err)
		}
	}
}

// TestAGCPReleaseGetsItsCredentialConfigurationBesideItsToken (w2/m479,
// ADR116 D11): a release that mounts GCP gets its token for the provider's
// default audience and, beside it, gcloud's credential configuration naming
// that token file. A deploy that changes the provider or the service account
// rewrites the configuration and re-mints the token for the new audience at
// once; a release recorded without inputs gets nothing minted for an empty
// audience, and the pass says so.
func TestAGCPReleaseGetsItsCredentialConfigurationBesideItsToken(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	app := optedIn("web", srvA)
	app.Status.ManagedOIDC.Providers = []string{"gcp"}
	app.Status.ManagedOIDC.GCP = &appv1alpha1.ManagedOIDCGCP{WorkloadIdentityProvider: gcpProvider}
	cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(app).WithStatusSubresource(app).Build()
	m := NewMinter(cl, f.issuer)
	m.Now = f.clock.Now
	p, err := oidc.NewProvider(ctx, f.base+"/"+wsA)
	if err != nil {
		t.Fatal(err)
	}
	expect := func(gcp appv1alpha1.ManagedOIDCGCP, golden string) {
		t.Helper()
		if err := m.Pass(ctx); err != nil {
			t.Fatal(err)
		}
		s, ok := tokenSecret(t, cl, "web")
		if !ok {
			t.Fatal("no token Secret")
		}
		if got := slices.Sorted(maps.Keys(s.Data)); !slices.Equal(got, []string{"gcp-credentials.json", "gcp-token"}) {
			t.Fatalf("Secret keys = %v, want the token and its credential configuration", got)
		}
		if golden != "" {
			want, err := os.ReadFile("testdata/" + golden)
			if err != nil {
				t.Fatal(err)
			}
			if string(s.Data["gcp-credentials.json"]) != string(want) {
				t.Fatalf("credential configuration:\n%s\nwant gcloud's %s:\n%s", s.Data["gcp-credentials.json"], golden, want)
			}
		}
		if _, err := p.Verifier(&oidc.Config{ClientID: gcp.Audience(), SupportedSigningAlgs: []string{oidc.ES256}, Now: f.clock.Now}).
			Verify(ctx, string(s.Data["gcp-token"])); err != nil {
			t.Fatalf("the GCP token does not verify for %s: %v", gcp.Audience(), err)
		}
	}
	redeploy := func(gcp *appv1alpha1.ManagedOIDCGCP) {
		t.Helper()
		var cur appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &cur); err != nil {
			t.Fatal(err)
		}
		cur.Status.ManagedOIDC.Generation++
		cur.Status.ManagedOIDC.GCP = gcp
		if err := cl.Status().Update(ctx, &cur); err != nil {
			t.Fatal(err)
		}
	}

	expect(appv1alpha1.ManagedOIDCGCP{WorkloadIdentityProvider: gcpProvider}, "gcloud-direct.json")
	impersonate := appv1alpha1.ManagedOIDCGCP{WorkloadIdentityProvider: gcpProvider, ServiceAccountEmail: gcpAccount}
	redeploy(&impersonate)
	expect(impersonate, "gcloud-impersonate.json")
	moved := appv1alpha1.ManagedOIDCGCP{WorkloadIdentityProvider: strings.Replace(gcpProvider, "bex-provider", "other-provider", 1)}
	redeploy(&moved)
	expect(moved, "")

	redeploy(nil)
	before, _ := tokenSecret(t, cl, "web")
	if err := m.Pass(ctx); err == nil {
		t.Fatal("a release that mounts GCP with no recorded inputs passed without an error")
	}
	if after, _ := tokenSecret(t, cl, "web"); string(after.Data["gcp-token"]) != string(before.Data["gcp-token"]) {
		t.Fatal("a release without GCP inputs changed the token the service reads")
	}
}

// TestAPassStopsForADowngradeOrADroppedTrigger: a service that left Pro, or
// whose running release no longer sets AWS_ROLE_ARN or never mounted the
// token, gets no new token; the one it has expires on its own.
func TestAPassStopsForADowngradeOrADroppedTrigger(t *testing.T) {
	ctx := context.Background()
	for name, change := range map[string]func(*appv1alpha1.App){
		"a downgraded workspace": func(a *appv1alpha1.App) { delete(a.Labels, appv1alpha1.LabelManagedOIDC) },
		"a dropped trigger":      func(a *appv1alpha1.App) { a.Status.ManagedOIDC.Providers = nil },
		"no trigger at all":      func(a *appv1alpha1.App) { a.Status.ManagedOIDC = nil },
		"a release the plan refused": func(a *appv1alpha1.App) {
			a.Status.ManagedOIDC.PlanAllowed = false
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			app := optedIn("web", srvA)
			cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(app).WithStatusSubresource(app).Build()
			m := NewMinter(cl, f.issuer)
			m.Now = f.clock.Now
			if err := m.Pass(ctx); err != nil {
				t.Fatal(err)
			}
			before, _ := tokenSecret(t, cl, "web")

			var cur appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &cur); err != nil {
				t.Fatal(err)
			}
			// The labels and the status are written apart, as bex-api and the
			// operator write them; each write returns the stored object.
			change(&cur)
			if err := cl.Update(ctx, &cur); err != nil {
				t.Fatal(err)
			}
			change(&cur)
			if err := cl.Status().Update(ctx, &cur); err != nil {
				t.Fatal(err)
			}
			f.clock.advance(RefreshAfter + time.Minute)
			f.reload(t)
			if err := m.Pass(ctx); err != nil {
				t.Fatal(err)
			}
			after, _ := tokenSecret(t, cl, "web")
			if issued(t, after) != issued(t, before) {
				t.Fatalf("%s still gets new tokens", name)
			}
		})
	}
}

// TestAServiceThatMovesGetsATokenForItsNewSubjectAtOnce: the sub claim names
// the environment, so a token for the old one is replaced on the next pass
// whatever its age.
func TestAServiceThatMovesGetsATokenForItsNewSubjectAtOnce(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	app := optedIn("web", srvA)
	cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(app).Build()
	m := NewMinter(cl, f.issuer)
	m.Now = f.clock.Now
	if err := m.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	var cur appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &cur); err != nil {
		t.Fatal(err)
	}
	delete(cur.Labels, core.LabelEnvironment)
	if err := cl.Update(ctx, &cur); err != nil {
		t.Fatal(err)
	}
	if err := m.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	s, _ := tokenSecret(t, cl, "web")
	if want := "workspace:" + wsA + ":environment:default:service:" + srvA; s.Annotations[annotationSubject] != want {
		t.Fatalf("subject after the move = %q, want %q", s.Annotations[annotationSubject], want)
	}
}

// TestAFailedMintKeepsTheOldTokenAndCountsTheFailure: an issuer that cannot
// sign (its key set went unread for an hour) leaves the Secret as it was and
// raises the failure counter, which is what pages.
func TestAFailedMintKeepsTheOldTokenAndCountsTheFailure(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(optedIn("web", srvA)).Build()
	m := NewMinter(cl, f.issuer)
	m.Now = f.clock.Now
	m.Metrics = NewMetrics(prometheus.NewRegistry())
	if err := m.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	before, _ := tokenSecret(t, cl, "web")
	f.clock.advance(oidcissuer.MaxStaleness + time.Minute) // no reload: the signer refuses
	if err := m.Pass(ctx); err == nil {
		t.Fatal("a pass whose mint failed reported no error")
	}
	after, _ := tokenSecret(t, cl, "web")
	if issued(t, after) != issued(t, before) || string(after.Data["aws-token"]) != string(before.Data["aws-token"]) {
		t.Fatal("a failed refresh changed the token the service reads")
	}
	if got := testutil.ToFloat64(m.Metrics.failures); got != 1 {
		t.Fatalf("refresh failures = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.Metrics.oldest); got < (oidcissuer.MaxStaleness + time.Minute).Seconds() {
		t.Fatalf("oldest token age = %vs, want the stale token's age", got)
	}
}

// TestTwoReplicasNeverBothWrite runs two minters as two bex-api replicas do:
// separate database pools, one advisory lock. While one replica's pass is
// writing, the other's pass runs and writes nothing, so a token is never
// minted twice or overwritten by a slower replica's older one.
func TestTwoReplicasNeverBothWrite(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	replica := func() *store.PGStore {
		pool, err := pgxpool.New(ctx, uri)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return store.NewPGStore(pool)
	}
	f := newFixture(t)
	base := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(optedIn("web", srvA), optedIn("api", srvB)).Build()

	var writes atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	writer := func(block bool) client.Client {
		return interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				writes.Add(1)
				if block {
					once.Do(func() { close(entered) })
					<-release
				}
				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				writes.Add(1)
				return c.Update(ctx, obj, opts...)
			},
		})
	}
	a := NewMinter(writer(true), f.issuer)
	a.Exclusive, a.Now = replica().RunExclusive, f.clock.Now
	b := NewMinter(writer(false), f.issuer)
	b.Exclusive, b.Now = replica().RunExclusive, f.clock.Now

	done := make(chan error, 1)
	go func() { done <- a.Pass(ctx) }()
	<-entered
	bErr := b.Pass(ctx)
	during := writes.Load()
	// Release A before judging, so a failure ends the test instead of leaving
	// A's pass (and its pooled connection) blocked.
	close(release)
	if bErr != nil {
		t.Fatalf("the second replica's pass: %v", bErr)
	}
	if during != 1 {
		t.Fatalf("writes while replica A holds the pass = %d, want 1 (replica B must write nothing)", during)
	}
	if err := <-done; err != nil {
		t.Fatalf("replica A's pass: %v", err)
	}
	if got := writes.Load(); got != 2 {
		t.Fatalf("writes after replica A's pass = %d, want 2 (one token Secret per service)", got)
	}
	// With A done, B takes the next pass, and finds both tokens fresh.
	if err := b.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if got := writes.Load(); got != 2 {
		t.Fatalf("writes after replica B's pass over fresh tokens = %d, want 2", got)
	}
}

// TestANativeBuildGetsItsOwnTokensWhileItBuilds (w2/m480, ADR116 D12): a
// native build's decision is minted into <app>-oidc-build, stamped with the
// build's release generation, beside and apart from the running release's
// <app>-oidc; a newer build is minted afresh whatever the age of the last
// one's tokens, the tokens are refreshed while the App builds, and once it
// stops building (the build ended, or was canceled and left its decision)
// they are no longer minted.
func TestANativeBuildGetsItsOwnTokensWhileItBuilds(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	app := optedIn("web", srvA)
	app.Status.Phase = appv1alpha1.PhaseBuilding
	gcpInputs := appv1alpha1.ManagedOIDCGCP{WorkloadIdentityProvider: gcpProvider}
	app.Status.BuildManagedOIDC = &appv1alpha1.ManagedOIDCStatus{Generation: 4, Providers: []string{"aws", "gcp"}, PlanAllowed: true, GCP: &gcpInputs}
	cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(app).WithStatusSubresource(app).Build()
	m := NewMinter(cl, f.issuer)
	m.Now = f.clock.Now
	buildSecret := func() (*corev1.Secret, bool) {
		t.Helper()
		var s corev1.Secret
		err := cl.Get(ctx, client.ObjectKey{Namespace: wsA, Name: appv1alpha1.ManagedOIDCBuildSecretName("web")}, &s)
		if apierrors.IsNotFound(err) {
			return nil, false
		}
		if err != nil {
			t.Fatal(err)
		}
		return &s, true
	}
	pass := func() {
		t.Helper()
		f.reload(t)
		if err := m.Pass(ctx); err != nil {
			t.Fatal(err)
		}
	}
	setStatus := func(mutate func(*appv1alpha1.AppStatus)) {
		t.Helper()
		var cur appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &cur); err != nil {
			t.Fatal(err)
		}
		mutate(&cur.Status)
		if err := cl.Status().Update(ctx, &cur); err != nil {
			t.Fatal(err)
		}
	}

	pass()
	s, ok := buildSecret()
	if !ok {
		t.Fatal("no build token Secret while the App builds")
	}
	if got, want := slices.Sorted(maps.Keys(s.Data)), []string{"aws-token", "gcp-credentials.json", "gcp-token"}; !slices.Equal(got, want) {
		t.Fatalf("build Secret keys = %v, want %v", got, want)
	}
	if s.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration] != "4" {
		t.Fatalf("build Secret generation = %q, want the build's release 4", s.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration])
	}
	if s.Labels[appv1alpha1.LabelProtectedFromTenantMount] != appv1alpha1.ProtectedFromTenantMount || len(s.OwnerReferences) != 1 || s.OwnerReferences[0].UID != "uid-web" {
		t.Fatalf("build Secret labels %v owners %+v: want protected and owned by the App", s.Labels, s.OwnerReferences)
	}
	runtime, ok := tokenSecret(t, cl, "web")
	if !ok || len(runtime.Data) != 1 || len(runtime.Data["aws-token"]) == 0 {
		t.Fatalf("the running release's Secret = %v; want its own aws-token alone, untouched by the build", runtime)
	}
	if _, has := runtime.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration]; has {
		t.Fatal("the running release's Secret carries a build generation")
	}
	p, err := oidc.NewProvider(ctx, f.base+"/"+wsA)
	if err != nil {
		t.Fatal(err)
	}
	for file, audience := range map[string]string{"aws-token": "sts.amazonaws.com", "gcp-token": gcpInputs.Audience()} {
		idt, err := p.Verifier(&oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.ES256}, Now: f.clock.Now}).Verify(ctx, string(s.Data[file]))
		if err != nil || idt.Subject != "workspace:"+wsA+":environment:"+evmA+":service:"+srvA {
			t.Fatalf("the build's %s: %v (sub %v); want it verified for %s with the service's sub", file, err, idt, audience)
		}
	}
	if string(s.Data["gcp-credentials.json"]) != string(gcpCredentialConfiguration(gcpInputs, "/var/lib/render/oidc/gcp-token")) {
		t.Fatalf("the build's credential configuration = %s", s.Data["gcp-credentials.json"])
	}
	first := issued(t, s)

	// A newer build takes nothing of the last one's tokens, however young.
	f.clock.advance(time.Minute)
	setStatus(func(st *appv1alpha1.AppStatus) { st.BuildManagedOIDC.Generation = 5 })
	pass()
	if s, _ = buildSecret(); s.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration] != "5" || issued(t, s) == first {
		t.Fatalf("a newer build reused the last build's tokens: generation %q issued %d", s.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration], issued(t, s))
	}
	second := issued(t, s)

	// Refreshed while the build waits or runs.
	f.clock.advance(RefreshAfter)
	pass()
	if s, _ = buildSecret(); issued(t, s) == second {
		t.Fatal("a building App's build tokens were not refreshed at RefreshAfter")
	}
	third := issued(t, s)

	// The App no longer builds: the decision it left is minted no more.
	setStatus(func(st *appv1alpha1.AppStatus) { st.Phase = appv1alpha1.PhaseRunning })
	f.clock.advance(RefreshAfter)
	pass()
	if s, _ = buildSecret(); issued(t, s) != third {
		t.Fatal("a decision left once the App stopped building was refreshed")
	}

	// A Dockerfile build, or a native one without a trigger, records no
	// decision that mounts anything: nothing is written for it.
	other := optedIn("dock", srvB)
	other.Status.ManagedOIDC = nil
	other.Status.Phase = appv1alpha1.PhaseBuilding
	other.Status.BuildManagedOIDC = &appv1alpha1.ManagedOIDCStatus{Generation: 2, PlanAllowed: true}
	if err := cl.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	hobby := optedIn("hobby", srvB)
	hobby.Status.ManagedOIDC = nil
	hobby.Status.Phase = appv1alpha1.PhaseBuilding
	hobby.Status.BuildManagedOIDC = &appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"aws"}}
	if err := cl.Create(ctx, hobby); err != nil {
		t.Fatal(err)
	}
	// A Build Source carrier has a bds- id (not a service id); even if
	// labeled with a build decision, Pass skips it without failing or minting.
	carrier := optedIn("carrier", bdsA)
	carrier.Spec.Type = appv1alpha1.TypeBuildSource
	carrier.Status.ManagedOIDC = nil
	carrier.Status.Phase = appv1alpha1.PhaseBuilding
	carrier.Status.BuildManagedOIDC = &appv1alpha1.ManagedOIDCStatus{Generation: 2, Providers: []string{"aws"}, PlanAllowed: true}
	if err := cl.Create(ctx, carrier); err != nil {
		t.Fatal(err)
	}
	pass()
	for _, name := range []string{"dock", "hobby", "carrier"} {
		var s corev1.Secret
		for _, secret := range []string{appv1alpha1.ManagedOIDCBuildSecretName(name), appv1alpha1.ManagedOIDCSecretName(name)} {
			if err := cl.Get(ctx, client.ObjectKey{Namespace: wsA, Name: secret}, &s); !apierrors.IsNotFound(err) {
				t.Fatalf("%s: %s exists (%v); want no token for a build that mounts none", name, secret, err)
			}
		}
	}
}
