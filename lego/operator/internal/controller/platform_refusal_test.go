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
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr/funcr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// capturedLog records what the operator logs through a context's logger, V(1)
// included.
type capturedLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *capturedLog) into(ctx context.Context) context.Context {
	return logf.IntoContext(ctx, funcr.New(func(prefix, args string) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.lines = append(l.lines, args)
	}, funcr.Options{Verbosity: 1}))
}

func (l *capturedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

// errors is the lines logged at error level: funcr gives them an "error" key.
func (l *capturedLog) errors() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, line := range l.lines {
		if strings.Contains(line, `"error"=`) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// policyDenialText is the 2026-11-12 refusal's policy half; the API server
// prefixes it with the refused object, `deployments.apps "<name>" is forbidden:`.
const policyDenialText = "ValidatingAdmissionPolicy 'bex-operator-workloads' with binding 'bex-operator-workloads' denied request: " +
	"operator-authored containers outside the execution boundary must be restricted"

func policyDenial(name string) error {
	return apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, name, errors.New(policyDenialText))
}

var platformErrorReference = regexp.MustCompile(`^g\d+-[0-9a-f]{8}$`)

// assertPlatformError checks msg is the fixed platform-error sentence and
// returns its reference.
func assertPlatformError(t *testing.T, msg string) string {
	t.Helper()
	code, params, ok := appv1alpha1.ParseDeployReason(msg)
	if !ok || code != appv1alpha1.DeployReasonPlatformError || len(params) != 1 || params[0].Name != "ref" {
		t.Fatalf("message %q parses as %s %v (ok=%v), want %s with a ref", msg, code, params, ok, appv1alpha1.DeployReasonPlatformError)
	}
	ref := params[0].Value
	if !platformErrorReference.MatchString(ref) {
		t.Fatalf("reference %q is not g<generation>-<8 hex>", ref)
	}
	return ref
}

// assertNoRawRefusal checks the customer's text names nothing internal.
func assertNoRawRefusal(t *testing.T, msg, namespace string) {
	t.Helper()
	for _, internal := range []string{"forbidden", "ValidatingAdmissionPolicy", "bex-operator-workloads", "deployments.apps", "execution boundary", "webhook", ".svc", namespace} {
		if strings.Contains(msg, internal) {
			t.Fatalf("customer message %q names %q", msg, internal)
		}
	}
}

// TestARefusedDeploymentWriteReadsAsAPlatformError (w9/090): the API server
// refusing the operator's Deployment write, as an admission policy did on
// 2026-11-12, used to reach the failed deploy and the Events tab verbatim,
// naming the policy and the tenant namespace. The App's Ready condition, which
// bex-api copies into both, now reads as a platform error with a support
// reference, and the raw text is only in the operator log, under the same
// reference. A first release's create and a later release's update both.
func TestARefusedDeploymentWriteReadsAsAPlatformError(t *testing.T) {
	for _, tc := range []struct {
		name      string
		update    bool
		wantGen   string
		namespace string
	}{
		{name: "first release create", namespace: "tea-refused-create", wantGen: "g1-"},
		{name: "later release update", update: true, namespace: "tea-refused-update", wantGen: "g2-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			armed := !tc.update
			refusals := 0
			refuse := func(obj client.Object) error {
				if _, ok := obj.(*appsv1.Deployment); ok && armed {
					refusals++
					return policyDenial(obj.GetName())
				}
				return nil
			}
			r, cl, nn := lifecycleFixture(t, activeApp(tc.namespace), withInterceptor(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if err := refuse(obj); err != nil {
						return err
					}
					return c.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if err := refuse(obj); err != nil {
						return err
					}
					return c.Update(ctx, obj, opts...)
				},
			}))
			if tc.update {
				serveReleaseOne(t, r, cl, nn)
				deployImageAt(t, cl, nn, "nginx:2", 2)
				armed = true
			}

			var log capturedLog
			var refused error
			for pass := 0; refusals == 0; pass++ {
				if pass == 5 {
					t.Fatal("setup: no pass wrote the Deployment")
				}
				_, refused = r.Reconcile(log.into(context.Background()), reconcile.Request{NamespacedName: nn})
			}
			if !apierrors.IsForbidden(refused) {
				t.Fatalf("the refused pass returned %v, want the raw refusal back so it requeues and is logged", refused)
			}

			live := liveApp(t, cl, nn)
			ready := findReadyCondition(&live)
			if live.Status.Phase != appv1alpha1.PhaseFailed || ready == nil || ready.Reason != "DeployFailed" {
				t.Fatalf("phase %q, Ready %+v; want Failed/DeployFailed", live.Status.Phase, ready)
			}
			ref := assertPlatformError(t, ready.Message)
			assertNoRawRefusal(t, ready.Message, nn.Namespace)
			if !strings.HasPrefix(ref, tc.wantGen) {
				t.Fatalf("reference %q does not name generation %s", ref, tc.wantGen)
			}
			want := "bex could not apply this release because of a platform error, not a problem with your code or settings. Contact support with reference " + ref
			if ready.Message != want {
				t.Fatalf("Ready message = %q, want %q", ready.Message, want)
			}
			logged := log.errors()
			if !strings.Contains(logged, ref) || !strings.Contains(logged, policyDenialText) {
				t.Fatalf("the operator's error log does not carry the raw refusal under %s:\n%s", ref, log.String())
			}

			// The same refusal on the next pass keeps the same reference, and the
			// retry logs it at V(1) only: its status already shows the reference.
			var retry capturedLog
			before := refusals
			_, _ = r.Reconcile(retry.into(context.Background()), reconcile.Request{NamespacedName: nn})
			if refusals == before {
				t.Fatal("setup: the retry did not write the Deployment")
			}
			again := liveApp(t, cl, nn)
			if got := findReadyCondition(&again); got == nil || got.Message != ready.Message {
				t.Fatalf("the retry changed the message: %+v, was %q", got, ready.Message)
			}
			if strings.Contains(retry.errors(), ref) || !strings.Contains(retry.String(), ref) {
				t.Fatalf("the retry logged the refusal at error level again, or not at all:\n%s", retry.String())
			}
		})
	}
}

// TestFailMasksOnlyPlatformRefusals (w9/090): fail records an API server
// refusal of the operator's own request as the platform-error sentence, on
// Ready and on a build failure's Build verdict. Every other failure keeps its
// own message, a quota refusal included: it names the plan limit the customer
// can raise.
func TestFailMasksOnlyPlatformRefusals(t *testing.T) {
	deployments := schema.GroupResource{Group: "apps", Resource: "deployments"}
	webhookDenial := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status: metav1.StatusFailure, Code: http.StatusBadRequest,
		Message: `admission webhook "vpod.bex.co" denied the request: the image is not signed`,
	}}
	quota := apierrors.NewForbidden(schema.GroupResource{Resource: "persistentvolumeclaims"}, "disk-web",
		errors.New("exceeded quota: tenant-quota, requested: requests.storage=10Gi, used: requests.storage=0, limited: requests.storage=5Gi"))
	for _, tc := range []struct {
		name   string
		reason string
		err    error
		masked bool
	}{
		{name: "admission policy", reason: "DeployFailed", err: policyDenial("web"), masked: true},
		{name: "rbac", reason: "ServiceFailed", err: apierrors.NewForbidden(schema.GroupResource{Resource: "services"}, "web",
			errors.New(`User "system:serviceaccount:bex-system:bex-controller-manager" cannot update resource "services"`)), masked: true},
		{name: "invalid", reason: "DeployFailed", err: apierrors.NewInvalid(schema.GroupKind{Group: "apps", Kind: "Deployment"}, "web",
			field.ErrorList{field.Invalid(field.NewPath("spec", "template"), "x", "bad")}), masked: true},
		{name: "webhook denial", reason: "DeployFailed", err: webhookDenial, masked: true},
		{name: "webhook unreachable", reason: "DeployFailed", err: apierrors.NewInternalError(errors.New(`failed calling webhook "vpod.bex.co": ` +
			`failed to call webhook: Post "https://bex-webhook-service.bex-system.svc:443/validate-v1-pod?timeout=10s": connect: connection refused`)), masked: true},
		{name: "wrapped refusal", reason: "NetworkPolicyFailed", err: fmt.Errorf("converge network policy: %w", policyDenial("web")), masked: true},
		{name: "build job refused", reason: appv1alpha1.ReasonBuildFailed, err: policyDenial("build-web"), masked: true},
		{name: "quota", reason: "DeployFailed", err: quota},
		{name: "not found", reason: "DeployFailed", err: apierrors.NewNotFound(deployments, "web")},
		{name: "operator diagnosis", reason: "DeployFailed", err: errors.New("image pull secret missing")},
		{name: "build failure", reason: appv1alpha1.ReasonBuildFailed, err: errors.New("build failed in the install step:\nnpm ERR! missing script: build")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, cl, nn := lifecycleFixture(t, activeApp("tea-fail-mask"))
			app := liveApp(t, cl, nn)
			var log capturedLog
			if _, err := r.fail(log.into(context.Background()), &app, tc.reason, tc.err); !errors.Is(err, tc.err) {
				t.Fatalf("fail returned %v, want the raw error %v", err, tc.err)
			}
			live := liveApp(t, cl, nn)
			messages := []string{findReadyCondition(&live).Message}
			if appv1alpha1.IsBuildFailureReason(tc.reason) {
				build := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild)
				if build == nil {
					t.Fatal("a build failure stored no Build verdict")
				}
				messages = append(messages, build.Message)
			}
			for _, msg := range messages {
				if !tc.masked {
					if msg != tc.err.Error() {
						t.Fatalf("message = %q, want the failure's own %q", msg, tc.err.Error())
					}
					continue
				}
				ref := assertPlatformError(t, msg)
				assertNoRawRefusal(t, msg, nn.Namespace)
				if !strings.Contains(log.errors(), ref) {
					t.Fatalf("the operator's error log does not name %s:\n%s", ref, log.String())
				}
			}
			if tc.masked && messages[0] != messages[len(messages)-1] {
				t.Fatalf("one refusal read with two references: Ready %q, Build %q", messages[0], messages[len(messages)-1])
			}
			if !tc.masked && log.String() != "" {
				t.Fatalf("a failure the customer reads as it is was logged as a refusal:\n%s", log.String())
			}
		})
	}
}

// TestARefusedPreDeployStartReadsAsAPlatformError (w9/090): a pre-deploy step
// bex could not start records its verdict as the deploy's failure reason, so a
// refusal of the operator's own write there reads as the platform error too.
func TestARefusedPreDeployStartReadsAsAPlatformError(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-predeploy", UID: "uid-1", Generation: 4},
		Status:     appv1alpha1.AppStatus{ReleaseGeneration: 3},
	}
	refused := apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "networkpolicies"}, "web-execution",
		errors.New("ValidatingAdmissionPolicy 'bex-operator-networkpolicies' with binding 'bex-operator-networkpolicies' denied request: no"))
	msg := preDeployNotStarted(context.Background(), app, refused)
	if ref := assertPlatformError(t, msg); !strings.HasPrefix(ref, "g3-") {
		t.Fatalf("reference %q does not name release generation 3", ref)
	}
	assertNoRawRefusal(t, msg, app.Namespace)
}

// TestARefusalKeepsItsReferenceAcrossASettingsSave (w9/090): the reference
// names the release, not metadata.generation, so a save that bumps the
// generation between two retries of one refusal shows the customer the same
// reference, on the Build verdict and on Ready alike.
func TestARefusalKeepsItsReferenceAcrossASettingsSave(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, activeApp("tea-refusal-save"))
	refused := policyDenial("build-web")
	generations := []int64{5, 6}
	refs := make([]string, 0, len(generations))
	for _, generation := range generations {
		app := liveApp(t, cl, nn)
		app.Generation, app.Status.ReleaseGeneration = generation, 4
		if _, err := r.fail(context.Background(), &app, appv1alpha1.ReasonBuildFailed, refused); !errors.Is(err, refused) {
			t.Fatalf("fail returned %v, want the refusal", err)
		}
		live := liveApp(t, cl, nn)
		build := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild)
		ready := findReadyCondition(&live)
		if build == nil || ready == nil || build.Message != ready.Message {
			t.Fatalf("Build %+v and Ready %+v do not read the same reference", build, ready)
		}
		refs = append(refs, assertPlatformError(t, build.Message))
	}
	if refs[0] != refs[1] || !strings.HasPrefix(refs[0], "g4-") {
		t.Fatalf("references %v, want one g4- reference across the save", refs)
	}
}

// TestServedReleasePodsRefusedByAdmissionReadAsAPlatformError (w9/090): when
// the served release's ReplicaSet cannot create its pods, Ready names the
// cause while the newer release waits. A quota names itself; an admission
// denial or an unreachable webhook reads as a platform error with a reference.
func TestServedReleasePodsRefusedByAdmissionReadAsAPlatformError(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-served-refused", UID: "uid-2", Generation: 3},
		Status:     appv1alpha1.AppStatus{ReleaseGeneration: 3},
	}
	served := func(failure string) *appsv1.Deployment {
		return &appsv1.Deployment{Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
			Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue, Reason: "FailedCreate", Message: failure,
		}}}}
	}
	const quota = `pods "web-5d8f9-" is forbidden: exceeded quota: tenant-quota, requested: pods=1, used: pods=20, limited: pods=20`
	for _, failure := range []string{
		`admission webhook "vpod.bex.co" denied the request: the image signature does not verify`,
		`pods "web-5d8f9-" is forbidden: ValidatingAdmissionPolicy 'bex-operator-pods' with binding 'bex-operator-pods' denied request: no`,
		`Internal error occurred: failed calling webhook "vpod.bex.co": failed to call webhook: Post "https://bex-webhook-service.bex-system.svc:443/validate-v1-pod": EOF`,
	} {
		var log capturedLog
		ready := startingServedCondition(log.into(context.Background()), app, served(failure))
		if ready.Reason != appv1alpha1.ReasonServedReleaseCannotStart || !strings.Contains(ready.Message, "because of a platform error") {
			t.Fatalf("Ready = %+v, want %s worded as a platform error", ready, appv1alpha1.ReasonServedReleaseCannotStart)
		}
		assertNoRawRefusal(t, ready.Message, app.Namespace)
		ref := platformRefusalReference(app, failure)
		if !strings.Contains(ready.Message, "reference "+ref) || !strings.Contains(log.errors(), ref) {
			t.Fatalf("Ready %q or the error log does not name %s:\n%s", ready.Message, ref, log.String())
		}
	}
	if ready := startingServedCondition(context.Background(), app, served(quota)); !strings.Contains(ready.Message, quota) {
		t.Fatalf("a quota refusal lost its own wording: %q", ready.Message)
	}
}
