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
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render issues custom domains' certificates from Let's Encrypt and Google
// Trust Services. bex orders a host's certificate from the first ClusterIssuer
// (ClusterIssuer, BEX_CLUSTER_ISSUER) and, when that CA cannot issue it,
// sends the next attempt to the second (SecondClusterIssuer,
// BEX_SECOND_CLUSTER_ISSUER). cert-manager has no fallback of its own and
// ingress-shim takes one issuer per Ingress, so the operator takes the host's
// Certificate over from ingress-shim to name the other issuer (ADR005 § Two
// certificate authorities, w21/m618).

// +kubebuilder:rbac:groups=acme.cert-manager.io,resources=challenges,verbs=get;list;watch

// acmeChallengeGVK is cert-manager's Challenge. A CAA refusal at validation
// reaches only the Challenge: the Order and the Certificate then read just
// `order is in "invalid" state`.
var acmeChallengeGVK = schema.GroupVersionKind{Group: "acme.cert-manager.io", Version: "v1", Kind: "Challenge"}

// issuerMoveLead is how long before cert-manager's own retry a later move
// lands. A changed issuer makes cert-manager retry at once, so the move must
// come first for that retry to go to the other CA, and the App's 30-second
// pass must not miss the moment.
const issuerMoveLead = 5 * time.Minute

// issuanceFailure is cert-manager's record of a Certificate's failing
// attempts: the count since the last success and when the last one failed.
type issuanceFailure struct {
	message  string
	attempts int64
	at       time.Time
}

// certificateIssuanceFailure reads the failure of cert's latest attempt, when
// it failed.
func certificateIssuanceFailure(cert *unstructured.Unstructured) (issuanceFailure, bool) {
	message, failed := certificateFailedAttempt(cert)
	if !failed {
		return issuanceFailure{}, false
	}
	f := issuanceFailure{message: message, attempts: 1}
	if n, found, _ := unstructured.NestedInt64(cert.Object, "status", "failedIssuanceAttempts"); found && n > 0 {
		f.attempts = n
	}
	if raw, _, _ := unstructured.NestedString(cert.Object, "status", "lastFailureTime"); raw != "" {
		f.at, _ = time.Parse(time.RFC3339, raw)
	}
	return f, true
}

// certManagerRetryDelay is how long cert-manager waits after the n-th failed
// attempt in a row before it retries: an hour, doubling to 32 hours
// (shouldBackoffReissuingOnFailure, cert-manager v1.20).
func certManagerRetryDelay(attempts int64) time.Duration {
	return time.Hour << (min(max(attempts, 1), 6) - 1)
}

// issuerMoveDue decides whether a failing host's next attempt goes to the
// other CA. refused says the CA refused the host itself (CAA or a rate
// limit), so the other CA may well issue at once.
//
//   - After the first failed attempt a refusal moves the host at once: a
//     domain whose CAA names only pki.goog gets its certificate within
//     minutes of Let's Encrypt's refusal.
//   - After a second failed attempt a refusal moves it, and from the third
//     every failure does, but only when cert-manager's own retry is due. The
//     move changes which CA the retry goes to, never how often bex retries:
//     a domain neither CA may issue for alternates on cert-manager's
//     schedule, an hour doubling to 32.
func issuerMoveDue(f issuanceFailure, refused bool, now time.Time) bool {
	switch {
	case f.attempts <= 1:
		return refused
	case f.attempts == 2 && !refused:
		return false
	}
	return !now.Before(f.at.Add(certManagerRetryDelay(f.attempts) - issuerMoveLead))
}

// caRefused reports whether a failed attempt's text says the CA refused the
// host: its CAA records do not allow the CA, or the CA's rate limit was hit.
func caRefused(text string) bool {
	return appv1alpha1.CertificateRefusedOnCAA(text) || acmeRateLimited(text)
}

// otherClusterIssuer is the issuer a host on issuer moves to, or "" when
// issuer is neither of the two or no second issuer is configured.
func (r *AppReconciler) otherClusterIssuer(issuer string) string {
	if r.SecondClusterIssuer == "" || r.SecondClusterIssuer == r.ClusterIssuer {
		return ""
	}
	switch issuer {
	case r.ClusterIssuer:
		return r.SecondClusterIssuer
	case r.SecondClusterIssuer:
		return r.ClusterIssuer
	}
	return ""
}

// settleCertificateIssuers moves each host of ing whose certificate the CA it
// is ordered from cannot issue to the other ClusterIssuer (issuerMoveDue).
// Only an Ingress that ingress-shim issues for from the first issuer takes
// part, and only with a second issuer configured: otherwise it reads nothing.
// A wildcard's DNS-01 issuer has no second (w4/m326).
func (r *AppReconciler) settleCertificateIssuers(ctx context.Context, ing *networkingv1.Ingress) error {
	if r.otherClusterIssuer(r.ClusterIssuer) == "" || ing.Annotations[certManagerClusterIssuerAnnotation] != r.ClusterIssuer {
		return nil
	}
	app := metav1.GetControllerOf(ing)
	if app == nil {
		return nil
	}
	for _, entry := range ing.Spec.TLS {
		if entry.SecretName == "" || slices.ContainsFunc(entry.Hosts, appv1alpha1.IsWildcardHost) {
			continue
		}
		if err := r.settleCertificateIssuer(ctx, ing, *app, entry); err != nil {
			return err
		}
	}
	return nil
}

// settleCertificateIssuer moves one TLS entry's Certificate to the other
// issuer when issuerMoveDue says so. The move is one update: the Certificate
// becomes the App's instead of the Ingress's and names the other issuer.
// ingress-shim leaves a Certificate it does not control alone, so it neither
// puts the issuer back nor deletes it, and the Secret, the Ingress and its
// routes stay as they are. A Certificate already taken over stays the App's,
// whichever issuer it names; one whose host has changed (the `<app>-tls` of a
// new first host) is deleted, and ingress-shim orders the new host afresh.
func (r *AppReconciler) settleCertificateIssuer(ctx context.Context, ing *networkingv1.Ingress, app metav1.OwnerReference, entry networkingv1.IngressTLS) error {
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	if err := r.Get(ctx, client.ObjectKey{Namespace: ing.Namespace, Name: entry.SecretName}, cert); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil // ingress-shim has not ordered it yet
		}
		return fmt.Errorf("read Certificate %s/%s: %w", ing.Namespace, entry.SecretName, err)
	}
	if takenOver(cert, app) {
		if dnsNames, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames"); !slices.Equal(dnsNames, entry.Hosts) {
			return deleteTolerantly(ctx, r.Client, cert)
		}
	} else if !metav1.IsControlledBy(cert, ing) {
		return nil
	}
	issuer, _, _ := unstructured.NestedString(cert.Object, "spec", "issuerRef", "name")
	other := r.otherClusterIssuer(issuer)
	failure, failed := certificateIssuanceFailure(cert)
	if other == "" || !failed {
		return nil
	}
	// From the third attempt every failure counts, so only an earlier one
	// needs its cause, which a CAA refusal at validation leaves on the
	// Challenge alone.
	refused := failure.attempts < 3 && (caRefused(failure.message) || r.challengeRefusedOnCAA(ctx, ing.Namespace, entry.Hosts, issuer))
	if !issuerMoveDue(failure, refused, time.Now()) {
		return nil
	}
	app.BlockOwnerDeletion = new(true)
	cert.SetOwnerReferences([]metav1.OwnerReference{app})
	if err := unstructured.SetNestedField(cert.Object, clusterIssuerRef(other), "spec", "issuerRef"); err != nil {
		return err
	}
	logf.FromContext(ctx).Info("moving a host's certificate to the other ClusterIssuer",
		"certificate", entry.SecretName, "from", issuer, "to", other, "failedAttempts", failure.attempts, "refused", refused)
	if err := r.Update(ctx, cert); err != nil {
		return fmt.Errorf("move Certificate %s/%s to ClusterIssuer %s: %w", ing.Namespace, entry.SecretName, other, err)
	}
	return nil
}

// takenOver reports whether the operator took cert over for the App that
// controls its Ingress.
func takenOver(cert *unstructured.Unstructured, app metav1.OwnerReference) bool {
	controller := metav1.GetControllerOf(cert)
	return controller != nil && controller.UID == app.UID
}

// challengeRefusedOnCAA reports whether a Challenge issuer made for one of
// hosts says the CA refused it on CAA. Every failure yields false: the cause
// only makes a move sooner.
func (r *AppReconciler) challengeRefusedOnCAA(ctx context.Context, namespace string, hosts []string, issuer string) bool {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(acmeChallengeGVK.GroupVersion().WithKind(acmeChallengeGVK.Kind + "List"))
	if err := r.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return false
	}
	for _, item := range list.Items {
		dnsName, _, _ := unstructured.NestedString(item.Object, "spec", "dnsName")
		challengeIssuer, _, _ := unstructured.NestedString(item.Object, "spec", "issuerRef", "name")
		reason, _, _ := unstructured.NestedString(item.Object, "status", "reason")
		if challengeIssuer == issuer && slices.ContainsFunc(hosts, func(h string) bool { return strings.EqualFold(h, dnsName) }) &&
			appv1alpha1.CertificateRefusedOnCAA(reason) {
			return true
		}
	}
	return false
}
