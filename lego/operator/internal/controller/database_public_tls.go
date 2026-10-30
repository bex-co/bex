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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Publicly trusted server certificates for public Databases (w2/m245).
//
// Render's external Postgres hosts present Let's Encrypt certificates, so a
// copied external URL connects from any stock client with no downloaded CA.
// CNPG's generated server certificate is signed by a per-cluster CA that rotates
// about every 90 days, which breaks every client that pinned it. When
// BEX_DB_CLUSTER_ISSUER names a cert-manager ClusterIssuer, a public Database
// therefore gets a cert-manager Certificate for all of its public SNI names, and
// once it is issued the CNPG Cluster serves it through
// spec.certificates.serverTLSSecret + serverCASecret. CNPG keeps generating and
// managing its own client CA and streaming_replica certificate (proven on dev-2,
// w2/m245 t001).
//
// Why the in-cluster names need not be in the public certificate: every CNPG
// consumer of the server certificate verifies only the CA, never a hostname —
//   - replication: primary_conninfo / pg_basebackup use sslmode=verify-ca with
//     sslrootcert=<serverCASecret ca.crt> (pkg/management/postgres/conninfo.go);
//   - PgBouncer poolers: server_tls_sslmode=verify-ca against the same CA
//     (pkg/management/pgbouncer/config/config.go); the pooler's client-facing
//     certificate IS the cluster's serverTLSSecret, so the pooler's public SNI
//     name must be (and is) on the certificate;
//   - the CNPG operator's instance-manager status client verifies the chain
//     against the CA and deliberately ignores the DNS name
//     (pkg/certs/tls.go, "user-provided certificates could not have the DNS
//     name for the <cluster>-rw service");
//   - the metrics PodMonitor sets InsecureSkipVerify for the same reason.
//
// bex's own paths need nothing either: pg-sni-proxy never terminates TLS (it
// answers the SSLRequest, peeks the ClientHello's SNI, and splices bytes, so the
// TLS session is end to end between the client and CNPG), and every internal
// connection string (apps, exports, insights, poolers) uses the -rw/-pooler
// Services without verify-full. The serverCASecret therefore holds exactly the
// root(s) that anchor the issued chain — ISRG Root X1/X2 or whichever Let's
// Encrypt root the chain resolves to — not the system bundle, so replication
// still trusts only the issuer that actually signed this database.
//
// Let's Encrypt budget: 50 NEW certificates per registered domain (bex.co) per
// week, shared with every other *.bex.co certificate (App custom domains are
// their own domains; *.onbex.co is a separate registered domain). Renewals are
// exempt. Each public database is ONE certificate whatever its SAN count, but
// any change to its name set — enabling the pooler, adding or removing a read
// replica, or turning public access off and on — orders a new one, and an
// identical name set is further capped at 5 per week. That bounds bex at about
// 50 new public databases (or name-set changes) per week. A *.db.bex.co wildcard
// would lift it but needs a DNS-01 (Cloudflare) token bex does not hold; file it
// as a follow-up with that token's scope (Zone:DNS:Edit on bex.co) if the
// budget ever binds.
const (
	dbPublicTLSSuffix = "-public-tls"
	dbPublicCASuffix  = "-public-ca"
	// cnpgReloadLabel makes CNPG hot-reload PostgreSQL when a user-provided
	// Secret changes. Without it a renewed certificate is not served until a
	// restart (t001: the old serial was still served after 85s).
	cnpgReloadLabel = "cnpg.io/reload"
	// labelDatabaseUID binds the cert-manager-written Secret to this Database's
	// lifetime: only a Secret carrying this Database's UID is ever deleted.
	labelDatabaseUID = "app.bex.co/database-uid"
	// conditionPublicCertificateReady reports the publicly trusted certificate's
	// progress; it never affects the Database's Ready condition, because the
	// database serves with CNPG's own certificate meanwhile.
	conditionPublicCertificateReady = "PublicCertificateReady"
	publicTLSPendingRequeue         = 30 * time.Second
	publicTLSSettleRequeue          = 5 * time.Second
)

func databasePublicTLSName(db *appv1alpha1.Database) string { return db.Name + dbPublicTLSSuffix }

// clusterIssuerRef is a cert-manager Certificate's issuerRef to a
// ClusterIssuer, as every operator-managed Certificate spells it. The group is
// left to cert-manager's default: adding it to an existing Certificate's spec
// would be a spec change.
func clusterIssuerRef(name string) map[string]any {
	return map[string]any{"name": name, "kind": "ClusterIssuer"}
}
func databasePublicCAName(db *appv1alpha1.Database) string { return db.Name + dbPublicCASuffix }

// databasePublicTLS is what one reconcile resolved about the public certificate.
type databasePublicTLS struct {
	// wanted: the Database is public and an issuer is configured.
	wanted bool
	// wired: the Cluster spec should name the public TLS + CA Secrets.
	wired   bool
	requeue time.Duration
}

// databasePublicCertificateSpec is the cert-manager Certificate spec for a
// public Database: one certificate covering every public SNI name (primary,
// pooler, read replicas), written to a Secret CNPG reloads on renewal.
func databasePublicCertificateSpec(db *appv1alpha1.Database, names []string, issuer string) map[string]any {
	dnsNames := make([]any, len(names))
	for i, name := range names {
		dnsNames[i] = name
	}
	return map[string]any{
		"secretName": databasePublicTLSName(db),
		"dnsNames":   dnsNames,
		"issuerRef":  clusterIssuerRef(issuer),
		"secretTemplate": map[string]any{"labels": map[string]any{
			cnpgReloadLabel:  "",
			labelDatabaseUID: string(db.UID),
		}},
	}
}

// clusterNamesPublicTLS reports whether the live Cluster's spec or CNPG's
// observed status names this Database's public TLS Secret.
func clusterNamesPublicTLS(db *appv1alpha1.Database, cluster *unstructured.Unstructured, field ...string) bool {
	name, _, _ := unstructured.NestedString(cluster.Object, append(field, "certificates", "serverTLSSecret")...)
	return name == databasePublicTLSName(db)
}

// preparePublicTLS converges the Certificate (and the CA Secret derived from
// what it issued) for a public Database and decides whether the Cluster serves
// it. It never fails the reconcile: on any problem the Database keeps serving
// with whatever certificate the Cluster already uses, and the
// PublicCertificateReady condition says why.
func (r *DatabaseReconciler) preparePublicTLS(ctx context.Context, db *appv1alpha1.Database, cluster *unstructured.Unstructured) databasePublicTLS {
	names := databaseServerAltDNSNames(db, r.DBDomain)
	if len(names) == 0 || r.PublicCertIssuer == "" {
		return databasePublicTLS{}
	}
	state := databasePublicTLS{wanted: true, requeue: publicTLSPendingRequeue}
	// Already serving it: keep serving while the Secret stays valid, even when
	// the Certificate is re-issuing for a changed name set. Falling back to
	// CNPG's certificate would break every connected client mid-renewal.
	liveWired := clusterNamesPublicTLS(db, cluster, "spec")
	keep := func(reason string, err error) databasePublicTLS {
		logf.FromContext(ctx).Info("public certificate not ready", "database", db.Name, "reason", reason, "err", err)
		setPublicCertificateCondition(db, metav1.ConditionFalse, reason, err.Error())
		state.wired = liveWired
		return state
	}

	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	cert.SetName(databasePublicTLSName(db))
	cert.SetNamespace(db.Namespace)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cert, func() error {
		if cert.GetResourceVersion() != "" && !metav1.IsControlledBy(cert, db) {
			return fmt.Errorf("refusing to adopt Certificate %s/%s owned by another object", cert.GetNamespace(), cert.GetName())
		}
		if err := projectUnstructuredSpec(cert, databasePublicCertificateSpec(db, names, r.PublicCertIssuer)); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(db, cert, r.Scheme)
	}); err != nil {
		return keep("CertificateFailed", err)
	}
	if !certificateReady(cert) && !liveWired {
		setPublicCertificateCondition(db, metav1.ConditionFalse, "Issuing",
			"waiting for the publicly trusted certificate to be issued")
		return state
	}

	secret := &corev1.Secret{}
	if err := r.secretClient().Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: databasePublicTLSName(db)}, secret); err != nil {
		return keep("CertificateSecretUnavailable", err)
	}
	roots, err := publicCertificateRoots(secret, names[0], r.PublicCertRoots, r.exportNow())
	if err != nil {
		// An expired or untrusted Secret must not stay served.
		liveWired = false
		return keep("CertificateUntrusted", err)
	}
	if err := r.ensurePublicCASecret(ctx, db, roots); err != nil {
		return keep("CASecretFailed", err)
	}
	state.wired = true
	state.requeue = 0
	setPublicCertificateCondition(db, metav1.ConditionTrue, "Issued", "the external endpoints serve a publicly trusted certificate")
	return state
}

func setPublicCertificateCondition(db *appv1alpha1.Database, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: conditionPublicCertificateReady, Status: status, Reason: reason,
		Message: message, ObservedGeneration: db.Generation,
	})
}

// certificateReady reports a cert-manager Certificate's Ready=True for its
// current generation, so a Ready left over from a previous name set is ignored.
func certificateReady(cert *unstructured.Unstructured) bool {
	conditions, _, _ := unstructured.NestedSlice(cert.Object, "status", "conditions")
	for _, raw := range conditions {
		c, ok := raw.(map[string]any)
		if !ok || c["type"] != "Ready" {
			continue
		}
		if c["status"] != "True" {
			return false
		}
		observed, found, _ := unstructured.NestedInt64(c, "observedGeneration")
		return !found || observed == cert.GetGeneration()
	}
	return false
}

// publicCertificateRoots verifies the issued kubernetes.io/tls Secret against
// the trusted roots (nil = the system pool) for host and returns the PEM of
// every root that anchors it, deduplicated and in a stable order. That bundle
// is what CNPG's verify-ca replication and pooler connections trust.
func publicCertificateRoots(secret *corev1.Secret, host string, roots *x509.CertPool, now time.Time) ([]byte, error) {
	if len(secret.Data[corev1.TLSPrivateKeyKey]) == 0 {
		return nil, fmt.Errorf("secret %s has no private key", secret.Name)
	}
	var chain []*x509.Certificate
	rest := secret.Data[corev1.TLSCertKey]
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("secret %s: %w", secret.Name, err)
		}
		chain = append(chain, cert)
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("secret %s has no certificate", secret.Name)
	}
	intermediates := x509.NewCertPool()
	for _, cert := range chain[1:] {
		intermediates.AddCert(cert)
	}
	chains, err := chain[0].Verify(x509.VerifyOptions{
		DNSName: host, Roots: roots, Intermediates: intermediates, CurrentTime: now,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("secret %s is not publicly trusted for %s: %w", secret.Name, host, err)
	}
	var anchors []*x509.Certificate
	for _, c := range chains {
		root := c[len(c)-1]
		if !slices.ContainsFunc(anchors, func(a *x509.Certificate) bool { return a.Equal(root) }) {
			anchors = append(anchors, root)
		}
	}
	slices.SortFunc(anchors, func(a, b *x509.Certificate) int { return bytes.Compare(a.Raw, b.Raw) })
	var out bytes.Buffer
	for _, a := range anchors {
		if err := pem.Encode(&out, &pem.Block{Type: "CERTIFICATE", Bytes: a.Raw}); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// ensurePublicCASecret writes the CA-only Secret CNPG's serverCASecret names.
// It carries only ca.crt (no key), which CNPG accepts when serverTLSSecret is
// set, and the reload label so a root change reaches running instances.
func (r *DatabaseReconciler) ensurePublicCASecret(ctx context.Context, db *appv1alpha1.Database, roots []byte) error {
	ca := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: databasePublicCAName(db), Namespace: db.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.secretClient(), ca, func() error {
		if ca.ResourceVersion != "" && !metav1.IsControlledBy(ca, db) {
			return fmt.Errorf("refusing to adopt Secret %s/%s owned by another object", ca.Namespace, ca.Name)
		}
		metav1.SetMetaDataLabel(&ca.ObjectMeta, cnpgReloadLabel, "")
		metav1.SetMetaDataLabel(&ca.ObjectMeta, labelDatabaseUID, string(db.UID))
		ca.Data = map[string][]byte{"ca.crt": roots}
		return controllerutil.SetControllerReference(db, ca, r.Scheme)
	})
	return err
}

// settlePublicTLS runs after the Cluster spec was written. It stamps
// status.externalCertificate from what CNPG actually serves, and when the
// public certificate is no longer wanted it withdraws it: the Certificate goes
// at once, its Secrets only after CNPG has stopped referencing them, so CNPG is
// never pointed at a missing Secret.
func (r *DatabaseReconciler) settlePublicTLS(ctx context.Context, db *appv1alpha1.Database, cluster *unstructured.Unstructured, state databasePublicTLS) (time.Duration, error) {
	if state.wanted {
		db.Status.ExternalCertificate = appv1alpha1.DBExternalCertificatePending
		if state.wired && clusterNamesPublicTLS(db, cluster, "status") {
			db.Status.ExternalCertificate = appv1alpha1.DBExternalCertificatePubliclyTrusted
		} else if state.wired {
			// CNPG has not observed the new spec yet; its Cluster status
			// update re-triggers this reconcile.
			return publicTLSSettleRequeue, nil
		}
		return state.requeue, nil
	}
	if db.Status.ExternalCertificate == "" {
		return 0, nil
	}
	// Withdrawing: the endpoints may still serve the public certificate, so
	// clients are pointed back at CNPG's own CA from now on.
	db.Status.ExternalCertificate = appv1alpha1.DBExternalCertificatePending
	meta.RemoveStatusCondition(&db.Status.Conditions, conditionPublicCertificateReady)
	if err := r.deletePublicCertificate(ctx, db); err != nil {
		return 0, err
	}
	if clusterNamesPublicTLS(db, cluster, "status") {
		return publicTLSSettleRequeue, nil
	}
	if err := r.deletePublicTLSSecrets(ctx, db); err != nil {
		return 0, err
	}
	db.Status.ExternalCertificate = ""
	return 0, nil
}

// deletePublicCertificate deletes this Database's own Certificate, never a
// same-named one controlled by something else.
func (r *DatabaseReconciler) deletePublicCertificate(ctx context.Context, db *appv1alpha1.Database) error {
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	if err := r.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: databasePublicTLSName(db)}, cert); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	if !metav1.IsControlledBy(cert, db) {
		return nil
	}
	return deleteTolerantly(ctx, r.Client, cert)
}

// deletePublicTLSSecrets deletes the issued key pair and the derived CA bundle,
// each only when it provably belongs to this Database's lifetime: cert-manager
// writes no owner reference, so the key pair is matched by the UID label its
// secretTemplate stamps; the CA bundle is controller-owned.
func (r *DatabaseReconciler) deletePublicTLSSecrets(ctx context.Context, db *appv1alpha1.Database) error {
	for _, name := range []string{databasePublicTLSName(db), databasePublicCAName(db)} {
		secret := &corev1.Secret{}
		if err := r.secretClient().Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: name}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if db.UID == "" || secret.Labels[labelDatabaseUID] != string(db.UID) {
			continue
		}
		if err := deleteTolerantly(ctx, r.secretClient(), secret); err != nil {
			return err
		}
	}
	return nil
}
