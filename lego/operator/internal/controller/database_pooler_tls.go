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
	"crypto/sha256"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// poolerCertificateAnnotation carries a digest of the certificate the
// Pooler's PgBouncer serves on its Pod template (w8/245). PgBouncer reads the
// Cluster's serverTLSSecret at start and, unlike PostgreSQL (cnpg.io/reload),
// never reloads it: enabling the pooler on a public database re-issues the
// certificate with the pooler's name, and every renewal replaces it, yet the
// running PgBouncer kept serving the old one, so `verify-full` clients of the
// pooled URL failed until it was recreated. A new digest rolls it.
const poolerCertificateAnnotation = "app.bex.co/served-certificate"

// servedCertificateDigest is a short digest of the certificate the Cluster
// serves (spec.certificates.serverTLSSecret), or "" when it serves CNPG's own
// or the Secret is not written yet.
func (r *DatabaseReconciler) servedCertificateDigest(ctx context.Context, namespace string, cluster *unstructured.Unstructured) (string, error) {
	name, _, _ := unstructured.NestedString(cluster.Object, "spec", "certificates", "serverTLSSecret")
	if name == "" {
		return "", nil
	}
	secret, ok, err := r.optionalSecret(ctx, namespace, name)
	if err != nil || !ok || len(secret.Data[corev1.TLSCertKey]) == 0 {
		return "", err
	}
	sum := sha256.Sum256(secret.Data[corev1.TLSCertKey])
	return hex.EncodeToString(sum[:8]), nil
}

// stampPoolerCertificate writes digest onto the desired Pooler template. When
// the priority rule left the template out (an existing Pooler whose Cluster
// has not adopted the class), the live template is kept as it is, so only a
// certificate change rolls the Pooler.
func stampPoolerCertificate(spec map[string]any, live *unstructured.Unstructured, digest string) {
	template, ok := spec["template"].(map[string]any)
	if !ok {
		template, _, _ = unstructured.NestedMap(live.Object, "spec", "template")
		if template == nil {
			template = map[string]any{}
		}
		spec["template"] = template
	}
	_ = unstructured.SetNestedField(template, digest, "metadata", "annotations", poolerCertificateAnnotation)
}
