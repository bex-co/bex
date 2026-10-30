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

package controller

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var publicTLSNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCert(t testing.TB, tmpl *x509.Certificate, parent *testCA) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, parentCert := key, tmpl
	if parent != nil {
		signer, parentCert = parent.key, parent.cert
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, &key.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCA{cert: cert, key: key}
}

func testCATemplate(name string, serial int64) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: publicTLSNow.Add(-time.Hour), NotAfter: publicTLSNow.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
}

func pemOf(certs ...*x509.Certificate) []byte {
	var b bytes.Buffer
	for _, c := range certs {
		_ = pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
	}
	return b.Bytes()
}

// issuedChain is a root -> intermediate -> leaf chain shaped like Let's
// Encrypt's: the Secret's tls.crt carries the leaf and intermediate, never the
// root, so the root must be found in the trust pool.
func issuedChain(t testing.TB, names []string, notAfter time.Time) (root *x509.Certificate, tlsCrt []byte) {
	t.Helper()
	ca := newTestCert(t, testCATemplate("Test Root X1", 1), nil)
	intermediate := newTestCert(t, testCATemplate("Test R10", 2), ca)
	leaf := newTestCert(t, &x509.Certificate{
		SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: names[0]}, DNSNames: names,
		NotBefore: publicTLSNow.Add(-time.Hour), NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, intermediate)
	return ca.cert, pemOf(leaf.cert, intermediate.cert)
}

func publicTLSScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, gvk := range []schema.GroupVersionKind{
		cnpgClusterGVK, cnpgScheduledBackupGVK, cnpgBackupGVK, cnpgPoolerGVK, certManagerCertificateGVK,
	} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}
	return scheme
}

type publicTLSHarness struct {
	t   *testing.T
	ctx context.Context
	cl  client.Client
	r   *DatabaseReconciler
	req reconcile.Request
}

func newPublicTLSHarness(t *testing.T, issuer string, roots *x509.CertPool) *publicTLSHarness {
	t.Helper()
	scheme := publicTLSScheme(t)
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dpg-orders", Namespace: "default", UID: "uid-orders",
			Finalizers: []string{dbFinalizer},
		},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-1gb", Public: true, Pooler: true},
	}
	// CNPG's application Secret lets the pooler projection settle, so each
	// reconcile reaches the status write.
	app := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders-app", Namespace: "default"},
		Data:       map[string][]byte{"username": []byte("u"), "password": []byte("p"), "dbname": []byte("d")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, app).
		WithStatusSubresource(&appv1alpha1.Database{}).Build()
	r := &DatabaseReconciler{
		Client: cl, Scheme: scheme, DBDomain: "db.example",
		PublicCertIssuer: issuer, PublicCertRoots: roots,
		Now: func() time.Time { return publicTLSNow },
	}
	return &publicTLSHarness{
		t: t, ctx: context.Background(), cl: cl, r: r,
		req: reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}},
	}
}

func (h *publicTLSHarness) reconcile() reconcile.Result {
	h.t.Helper()
	res, err := h.r.Reconcile(h.ctx, h.req)
	if err != nil {
		h.t.Fatalf("reconcile: %v", err)
	}
	return res
}

func (h *publicTLSHarness) database() *appv1alpha1.Database {
	h.t.Helper()
	db := &appv1alpha1.Database{}
	if err := h.cl.Get(h.ctx, h.req.NamespacedName, db); err != nil {
		h.t.Fatal(err)
	}
	return db
}

func (h *publicTLSHarness) cluster() *unstructured.Unstructured {
	h.t.Helper()
	c := &unstructured.Unstructured{}
	c.SetGroupVersionKind(cnpgClusterGVK)
	if err := h.cl.Get(h.ctx, h.req.NamespacedName, c); err != nil {
		h.t.Fatal(err)
	}
	return c
}

func (h *publicTLSHarness) certificate() (*unstructured.Unstructured, error) {
	c := &unstructured.Unstructured{}
	c.SetGroupVersionKind(certManagerCertificateGVK)
	err := h.cl.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: "dpg-orders-public-tls"}, c)
	return c, err
}

// markIssued plays cert-manager: Ready for the current generation, and the
// key pair written with the secretTemplate's labels.
func (h *publicTLSHarness) markIssued(tlsCrt []byte) {
	h.t.Helper()
	cert, err := h.certificate()
	if err != nil {
		h.t.Fatal(err)
	}
	_ = unstructured.SetNestedSlice(cert.Object, []any{map[string]any{
		"type": "Ready", "status": "True", "observedGeneration": cert.GetGeneration(),
	}}, "status", "conditions")
	if err := h.cl.Update(h.ctx, cert); err != nil {
		h.t.Fatal(err)
	}
	labels, _, _ := unstructured.NestedStringMap(cert.Object, "spec", "secretTemplate", "labels")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders-public-tls", Namespace: "default", Labels: labels},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{corev1.TLSCertKey: tlsCrt, corev1.TLSPrivateKeyKey: []byte("key")},
	}
	if err := h.cl.Create(h.ctx, secret); err != nil {
		h.t.Fatal(err)
	}
}

// cnpgObserves plays CNPG: its status names the server TLS Secret in use.
func (h *publicTLSHarness) cnpgObserves(serverTLSSecret string) {
	h.t.Helper()
	c := h.cluster()
	_ = unstructured.SetNestedField(c.Object, serverTLSSecret, "status", "certificates", "serverTLSSecret")
	if err := h.cl.Update(h.ctx, c); err != nil {
		h.t.Fatal(err)
	}
}

func clusterCertificates(c *unstructured.Unstructured) map[string]any {
	certs, _, _ := unstructured.NestedMap(c.Object, "spec", "certificates")
	return certs
}

var publicNames = []string{"dpg-orders.db.example", "dpg-orders-pool.db.example"}

// TestPublicDatabaseWithoutIssuerKeepsCNPGCertificate: BEX_DB_CLUSTER_ISSUER
// unset is today's behavior — CNPG's generated certificate with the public
// SANs, no Certificate, no externalCertificate.
func TestPublicDatabaseWithoutIssuerKeepsCNPGCertificate(t *testing.T) {
	h := newPublicTLSHarness(t, "", nil)
	h.reconcile()
	certs := clusterCertificates(h.cluster())
	if _, ok := certs["serverTLSSecret"]; ok || certs["serverAltDNSNames"] == nil {
		t.Fatalf("feature off: cluster certificates = %v, want only CNPG serverAltDNSNames", certs)
	}
	// No cert-manager object at all, so the scheme must not even be consulted.
	if _, err := h.certificate(); !apierrors.IsNotFound(err) {
		t.Fatalf("feature off created a Certificate (err=%v)", err)
	}
	if got := h.database().Status.ExternalCertificate; got != "" {
		t.Fatalf("externalCertificate = %q, want empty", got)
	}
}

// TestPublicDatabaseCertificateLifecycle walks the whole contract: request,
// wait for Ready, wire into CNPG, report PubliclyTrusted only once CNPG serves
// it, and on public access off revert to CNPG's certificate before deleting
// anything CNPG still uses.
func TestPublicDatabaseCertificateLifecycle(t *testing.T) {
	root, tlsCrt := issuedChain(t, publicNames, publicTLSNow.Add(90*24*time.Hour))
	roots := x509.NewCertPool()
	roots.AddCert(root)
	h := newPublicTLSHarness(t, "letsencrypt-prod", roots)

	h.requireRequested()
	h.markIssued(tlsCrt)
	h.requireWired(root)
	h.requireServedAcrossReissue()
	h.requireWithdrawn()
}

// requireRequested: requested, not yet issued — the Certificate is rendered
// and CNPG keeps its own certificate.
func (h *publicTLSHarness) requireRequested() {
	t := h.t
	t.Helper()
	if res := h.reconcile(); res.RequeueAfter == 0 || res.RequeueAfter > publicTLSPendingRequeue {
		t.Fatalf("pending certificate requeue = %v, want <= %v", res.RequeueAfter, publicTLSPendingRequeue)
	}
	cert, err := h.certificate()
	if err != nil {
		t.Fatalf("no Certificate for a public database: %v", err)
	}
	spec, _, _ := unstructured.NestedMap(cert.Object, "spec")
	want := map[string]any{
		"secretName": "dpg-orders-public-tls",
		"dnsNames":   []any{publicNames[0], publicNames[1]},
		"issuerRef":  map[string]any{"name": "letsencrypt-prod", "kind": "ClusterIssuer"},
		// Without cnpg.io/reload CNPG never serves a renewal (t001).
		"secretTemplate": map[string]any{"labels": map[string]any{"cnpg.io/reload": "", labelDatabaseUID: "uid-orders"}},
	}
	if !reflect.DeepEqual(spec, want) {
		t.Errorf("Certificate spec = %v, want %v", spec, want)
	}
	if !metav1.IsControlledBy(cert, h.database()) {
		t.Error("Certificate is not owned by the Database")
	}
	if certs := clusterCertificates(h.cluster()); certs["serverTLSSecret"] != nil || certs["serverAltDNSNames"] == nil {
		t.Fatalf("cluster wired before the Certificate was Ready: %v", certs)
	}
	h.requireStatus(appv1alpha1.DBExternalCertificatePending, "Issuing")
}

// requireWired: once issued, CNPG gets the server Secret and a CA-only Secret
// holding the anchoring root; clients learn system roots suffice only after
// CNPG reports serving it.
func (h *publicTLSHarness) requireWired(root *x509.Certificate) {
	t := h.t
	t.Helper()
	h.reconcile()
	certs := clusterCertificates(h.cluster())
	want := map[string]any{"serverTLSSecret": "dpg-orders-public-tls", "serverCASecret": "dpg-orders-public-ca"}
	if !reflect.DeepEqual(certs, want) {
		t.Fatalf("cluster certificates = %v, want %v (serverAltDNSNames must go: CNPG rejects it beside serverTLSSecret)", certs, want)
	}
	ca := &corev1.Secret{}
	if err := h.cl.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: "dpg-orders-public-ca"}, ca); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca.Data["ca.crt"], pemOf(root)) || len(ca.Data) != 1 {
		t.Fatalf("CA Secret data = %v, want only ca.crt = the anchoring root", ca.Data)
	}
	if _, ok := ca.Labels["cnpg.io/reload"]; !ok {
		t.Error("CA Secret lacks cnpg.io/reload")
	}
	h.requireStatus(appv1alpha1.DBExternalCertificatePending, "Issued")

	h.cnpgObserves("dpg-orders-public-tls")
	h.reconcile()
	h.requireStatus(appv1alpha1.DBExternalCertificatePubliclyTrusted, "Issued")
}

// requireServedAcrossReissue: re-issuing for a changed name set keeps serving
// the still-valid Secret instead of falling back to CNPG's certificate.
func (h *publicTLSHarness) requireServedAcrossReissue() {
	t := h.t
	t.Helper()
	db := h.database()
	db.Spec.Pooler = false
	if err := h.cl.Update(h.ctx, db); err != nil {
		t.Fatal(err)
	}
	cert, _ := h.certificate()
	_ = unstructured.SetNestedSlice(cert.Object, []any{map[string]any{"type": "Ready", "status": "False"}}, "status", "conditions")
	if err := h.cl.Update(h.ctx, cert); err != nil {
		t.Fatal(err)
	}
	h.reconcile()
	if got := clusterCertificates(h.cluster())["serverTLSSecret"]; got != "dpg-orders-public-tls" {
		t.Fatalf("re-issue unwired the serving certificate: %v", got)
	}
	// The re-issue is for the new name set, while clients keep the Render URL:
	// flipping to Pending would hand them verify-full against a CA they lack.
	cert, _ = h.certificate()
	if names, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames"); !reflect.DeepEqual(names, publicNames[:1]) {
		t.Fatalf("re-issue dnsNames = %v, want %v", names, publicNames[:1])
	}
	h.requireStatus(appv1alpha1.DBExternalCertificatePubliclyTrusted, "Issued")
}

// requireWithdrawn: public access off brings CNPG's certificate back; the
// Certificate goes at once, the Secrets only after CNPG stopped using them.
func (h *publicTLSHarness) requireWithdrawn() {
	t := h.t
	t.Helper()
	db := h.database()
	db.Spec.Public = false
	if err := h.cl.Update(h.ctx, db); err != nil {
		t.Fatal(err)
	}
	h.reconcile()
	if certs := clusterCertificates(h.cluster()); certs != nil {
		t.Fatalf("private database still has cluster certificates %v", certs)
	}
	if _, err := h.certificate(); !apierrors.IsNotFound(err) {
		t.Fatalf("Certificate survived public access off (err=%v)", err)
	}
	h.requireSecrets(true)
	if got := h.database().Status.ExternalCertificate; got != appv1alpha1.DBExternalCertificatePending {
		t.Fatalf("externalCertificate while withdrawing = %q, want Pending", got)
	}
	h.cnpgObserves("dpg-orders-server")
	h.reconcile()
	h.requireSecrets(false)
	db = h.database()
	if db.Status.ExternalCertificate != "" || apimeta.FindStatusCondition(db.Status.Conditions, conditionPublicCertificateReady) != nil {
		t.Fatalf("withdrawn status = %q / %v", db.Status.ExternalCertificate, db.Status.Conditions)
	}
}

func (h *publicTLSHarness) requireStatus(want appv1alpha1.DatabaseExternalCertificate, reason string) {
	h.t.Helper()
	db := h.database()
	if db.Status.ExternalCertificate != want {
		h.t.Fatalf("externalCertificate = %q, want %q", db.Status.ExternalCertificate, want)
	}
	if c := apimeta.FindStatusCondition(db.Status.Conditions, conditionPublicCertificateReady); c == nil || c.Reason != reason {
		h.t.Fatalf("condition = %+v, want reason %s", c, reason)
	}
}

func (h *publicTLSHarness) requireSecrets(present bool) {
	h.t.Helper()
	for _, name := range []string{"dpg-orders-public-tls", "dpg-orders-public-ca"} {
		err := h.cl.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: name}, &corev1.Secret{})
		if present && err != nil {
			h.t.Fatalf("Secret %s deleted while CNPG still serves it: %v", name, err)
		}
		if !present && !apierrors.IsNotFound(err) {
			h.t.Fatalf("Secret %s survived the withdrawal (err=%v)", name, err)
		}
	}
}

// TestUntrustedPublicCertificateIsNotWired: a Secret that does not chain to a
// trusted root (a staging issuer, a stale foreign Secret) never reaches CNPG.
func TestUntrustedPublicCertificateIsNotWired(t *testing.T) {
	_, tlsCrt := issuedChain(t, publicNames, publicTLSNow.Add(90*24*time.Hour))
	otherRoot, _ := issuedChain(t, publicNames, publicTLSNow.Add(time.Hour))
	roots := x509.NewCertPool()
	roots.AddCert(otherRoot)
	h := newPublicTLSHarness(t, "letsencrypt-prod", roots)
	h.reconcile()
	h.markIssued(tlsCrt)
	h.reconcile()
	if certs := clusterCertificates(h.cluster()); certs["serverTLSSecret"] != nil {
		t.Fatalf("untrusted certificate wired into CNPG: %v", certs)
	}
	db := h.database()
	if c := apimeta.FindStatusCondition(db.Status.Conditions, conditionPublicCertificateReady); c == nil || c.Reason != "CertificateUntrusted" {
		t.Fatalf("condition = %+v, want CertificateUntrusted", c)
	}
	if db.Status.ExternalCertificate != appv1alpha1.DBExternalCertificatePending {
		t.Fatalf("externalCertificate = %q, want Pending", db.Status.ExternalCertificate)
	}
}

// servePubliclyTrusted drives a fresh harness to PubliclyTrusted: requested,
// issued, wired, and observed by CNPG.
func (h *publicTLSHarness) servePubliclyTrusted(tlsCrt []byte) {
	h.t.Helper()
	h.reconcile()
	h.markIssued(tlsCrt)
	h.reconcile()
	h.cnpgObserves("dpg-orders-public-tls")
	h.reconcile()
	h.requireStatus(appv1alpha1.DBExternalCertificatePubliclyTrusted, "Issued")
}

func publicTLSRoots(root *x509.Certificate) *x509.CertPool {
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return roots
}

// TestExpiredServedPublicCertificateFallsBackToCNPG: a served certificate that
// renewal failed to replace must not stay served past its expiry. The Cluster
// goes back to CNPG's certificate and status leaves PubliclyTrusted at once, so
// bex-api hands out the verify-full contract again instead of a Render URL that
// would now fail verification.
func TestExpiredServedPublicCertificateFallsBackToCNPG(t *testing.T) {
	notAfter := publicTLSNow.Add(90 * 24 * time.Hour)
	root, tlsCrt := issuedChain(t, publicNames, notAfter)
	h := newPublicTLSHarness(t, "letsencrypt-prod", publicTLSRoots(root))
	h.servePubliclyTrusted(tlsCrt)

	h.r.Now = func() time.Time { return notAfter.Add(time.Minute) }
	h.reconcile()
	certs := clusterCertificates(h.cluster())
	if certs["serverTLSSecret"] != nil || certs["serverAltDNSNames"] == nil {
		t.Fatalf("expired certificate still wired: %v", certs)
	}
	// CNPG's status still names the old Secret until it rolls over, but the
	// spec no longer does, so nothing may claim it is publicly trusted.
	h.requireStatus(appv1alpha1.DBExternalCertificatePending, "CertificateUntrusted")
	// Still public: the Certificate and its Secrets stay for the renewal.
	if _, err := h.certificate(); err != nil {
		t.Fatalf("Certificate deleted while the database is still public: %v", err)
	}
	h.requireSecrets(true)
}

// TestDeletingPublicDatabaseRemovesCertificateAndSecrets: the key pair carries
// no owner reference, so deletion must remove it explicitly once the Cluster
// is gone, together with the Certificate and the CA bundle.
func TestDeletingPublicDatabaseRemovesCertificateAndSecrets(t *testing.T) {
	root, tlsCrt := issuedChain(t, publicNames, publicTLSNow.Add(90*24*time.Hour))
	h := newPublicTLSHarness(t, "letsencrypt-prod", publicTLSRoots(root))
	h.servePubliclyTrusted(tlsCrt)

	if err := h.cl.Delete(h.ctx, h.database()); err != nil {
		t.Fatal(err)
	}
	h.reconcile() // deletes the Cluster
	// The Cluster still existed on that pass, so nothing it may serve went.
	h.requireSecrets(true)
	h.reconcile() // Cluster gone: certificate cleanup, finalizer released
	if _, err := h.certificate(); !apierrors.IsNotFound(err) {
		t.Fatalf("Certificate survived the database (err=%v)", err)
	}
	h.requireSecrets(false)
	if err := h.cl.Get(h.ctx, h.req.NamespacedName, &appv1alpha1.Database{}); !apierrors.IsNotFound(err) {
		t.Fatalf("Database finalizer not released (err=%v)", err)
	}
}

// TestPublicTLSLeavesForeignObjectsAlone: a same-named Certificate or Secret
// that is not this Database's (another controller's, or a previous lifetime's
// with a different UID) is never adopted, wired into CNPG, or deleted.
func TestPublicTLSLeavesForeignObjectsAlone(t *testing.T) {
	_, tlsCrt := issuedChain(t, publicNames, publicTLSNow.Add(90*24*time.Hour))
	h := newPublicTLSHarness(t, "letsencrypt-prod", nil)
	foreignCert := &unstructured.Unstructured{}
	foreignCert.SetGroupVersionKind(certManagerCertificateGVK)
	foreignCert.SetName("dpg-orders-public-tls")
	foreignCert.SetNamespace("default")
	_ = unstructured.SetNestedField(foreignCert.Object, "someone-elses-secret", "spec", "secretName")
	foreignSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders-public-tls", Namespace: "default", Labels: map[string]string{labelDatabaseUID: "uid-previous-lifetime"}},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{corev1.TLSCertKey: tlsCrt, corev1.TLSPrivateKeyKey: []byte("key")},
	}
	for _, o := range []client.Object{foreignCert, foreignSecret} {
		if err := h.cl.Create(h.ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	h.reconcile()
	if certs := clusterCertificates(h.cluster()); certs["serverTLSSecret"] != nil {
		t.Fatalf("foreign certificate wired into CNPG: %v", certs)
	}
	h.requireStatus(appv1alpha1.DBExternalCertificatePending, "CertificateFailed")
	cert, err := h.certificate()
	if err != nil {
		t.Fatal(err)
	}
	if secretName, _, _ := unstructured.NestedString(cert.Object, "spec", "secretName"); secretName != "someone-elses-secret" || len(cert.GetOwnerReferences()) != 0 {
		t.Fatalf("foreign Certificate was adopted: secretName=%q owners=%v", secretName, cert.GetOwnerReferences())
	}

	db := h.database()
	db.Spec.Public = false
	if err := h.cl.Update(h.ctx, db); err != nil {
		t.Fatal(err)
	}
	h.reconcile()
	if _, err := h.certificate(); err != nil {
		t.Fatalf("withdrawal deleted a foreign Certificate: %v", err)
	}
	if err := h.cl.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: "dpg-orders-public-tls"}, &corev1.Secret{}); err != nil {
		t.Fatalf("withdrawal deleted a Secret from another lifetime: %v", err)
	}
	if got := h.database().Status.ExternalCertificate; got != "" {
		t.Fatalf("externalCertificate after withdrawal = %q, want empty", got)
	}
}

func TestPublicCertificateRoots(t *testing.T) {
	root, tlsCrt := issuedChain(t, publicNames, publicTLSNow.Add(time.Hour))
	roots := x509.NewCertPool()
	roots.AddCert(root)
	secret := &corev1.Secret{Data: map[string][]byte{corev1.TLSCertKey: tlsCrt, corev1.TLSPrivateKeyKey: []byte("k")}}
	got, err := publicCertificateRoots(secret, publicNames[0], roots, publicTLSNow)
	if err != nil || !bytes.Equal(got, pemOf(root)) {
		t.Fatalf("roots = %q, %v; want the anchoring root only", got, err)
	}
	if _, err := publicCertificateRoots(secret, publicNames[0], roots, publicTLSNow.Add(2*time.Hour)); err == nil {
		t.Error("an expired certificate verified")
	}
	if _, err := publicCertificateRoots(secret, "dpg-other.db.example", roots, publicTLSNow); err == nil {
		t.Error("a certificate for other names verified")
	}
	noKey := &corev1.Secret{Data: map[string][]byte{corev1.TLSCertKey: tlsCrt}}
	if _, err := publicCertificateRoots(noKey, publicNames[0], roots, publicTLSNow); err == nil {
		t.Error("a Secret without a private key verified")
	}
}

func TestCertificateReadyIgnoresStaleGeneration(t *testing.T) {
	cert := &unstructured.Unstructured{Object: map[string]any{}}
	cert.SetGeneration(3)
	set := func(status string, generation int64) {
		_ = unstructured.SetNestedSlice(cert.Object, []any{map[string]any{
			"type": "Ready", "status": status, "observedGeneration": generation,
		}}, "status", "conditions")
	}
	set("True", 3)
	if !certificateReady(cert) {
		t.Error("Ready=True for the current generation not ready")
	}
	set("True", 2)
	if certificateReady(cert) {
		t.Error("Ready=True from a previous name set counted as ready")
	}
	set("False", 3)
	if certificateReady(cert) {
		t.Error("Ready=False counted as ready")
	}
}
