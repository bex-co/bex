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
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObserveProductApp is a best-effort sampler, called only for the reconciler's
// own managed Apps. A missing/old Ready condition is not current hosting proof.
// TLS uses the same certificate evidence as the product domain view; it does
// not probe customer URLs or claim that their DNS points at us.
func ObserveProductApp(ctx context.Context, st *store.PGStore, cl client.Client, desired store.DesiredApp, app *appv1alpha1.App) {
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	at := time.Now().UTC()
	ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	live := app.Status.Phase == appv1alpha1.PhaseRunning && !app.Spec.Suspended && ready != nil && ready.Status == metav1.ConditionTrue && ready.ObservedGeneration == app.Generation
	sampled, err := st.RecordProductHosting(bounded, desired.TenantID, desired.ID, effectiveType(app.Spec.Type), live, at)
	if err != nil {
		log.Printf("product analytics: hosting sample: %v", err)
		return
	}
	if !sampled {
		return
	}
	domains, err := st.ListDomainClaims(bounded, desired.ID)
	if err != nil {
		log.Printf("product analytics: domain sample: %v", err)
		return
	}
	for _, domain := range domains {
		if domain.RedirectForName != "" {
			continue
		}
		tlsReady := false
		if domain.ClaimState == "verified" {
			var err error
			tlsReady, err = domainCertificateReady(bounded, cl, app, domain.Host)
			if err != nil {
				log.Printf("product analytics: certificate sample unavailable")
				continue
			}
		}
		if err := st.RecordProductDomainTLS(bounded, domain.ID, tlsReady, at); err != nil {
			log.Printf("product analytics: certificate observation: %v", err)
		}
	}
}
