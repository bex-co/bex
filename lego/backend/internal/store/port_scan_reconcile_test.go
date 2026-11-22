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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/types/deploybudgets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/385: the reconciler's pass over an open row times the port scan from the
// operator's Ready diagnosis, through the same write as the stall reason.
func TestReconcilerTimesThePortScanFromTheReadyDiagnosis(t *testing.T) {
	ctx := context.Background()
	m := newMemStore()
	m.deploys["dep-1"] = Deploy{ID: "dep-1", Generation: 2, Status: DeployUpdateInProgress, CreatedAt: time.Now()}
	r := &Reconciler{Store: m, DeployGateTimeout: deploybudgets.DeployGateTimeout}
	app := &appv1alpha1.App{}
	app.Generation = 2
	app.Status.ReleaseGeneration = 2
	app.Status.Phase = appv1alpha1.PhaseDeploying
	pass := func(reason, message string) Deploy {
		t.Helper()
		app.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: reason, Message: message, ObservedGeneration: 2}}
		r.recordDeploy(ctx, DesiredApp{}, m.deploys["dep-1"], app)
		return m.deploys["dep-1"]
	}

	if d := pass(appv1alpha1.ReasonHTTPPortDiscovery, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortWaiting)); d.PortScanSince != nil {
		t.Fatalf("waiting to start: scan started at %v", d.PortScanSince)
	}
	d := pass(appv1alpha1.ReasonHTTPPortDiscovery, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet))
	if d.PortScanSince == nil || d.PortScanUntil != nil {
		t.Fatalf("no open port: scan = (%v, %v), want started", d.PortScanSince, d.PortScanUntil)
	}
	since := *d.PortScanSince
	noOpenPort := appv1alpha1.OpenPortProbeStallReason("startup", 0) +
		appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintBackgroundWorker)
	if d := pass("HealthCheckFailing", noOpenPort); !d.PortScanSince.Equal(since) || d.PortScanUntil != nil {
		t.Fatalf("still scanning: scan = (%v, %v), want (%v, nil)", d.PortScanSince, d.PortScanUntil, since)
	}
	if d := pass("RolloutProgressing", ""); d.PortScanUntil == nil {
		t.Fatalf("check passed: scan = (%v, nil), want ended", d.PortScanSince)
	}
}
