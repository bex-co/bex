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
	"errors"
	"fmt"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// admissionDenial matches an admission webhook's or admission policy's denial,
// or a webhook the API server could not reach, in an API status message. A
// webhook's denial carries the webhook's own status code, often 400, a policy
// may answer with a reason other than Forbidden or Invalid, and an unreachable
// webhook is an InternalError naming the webhook's Service, so none of them is
// caught by its status reason alone.
var admissionDenial = regexp.MustCompile(`admission webhook "[^"]*" denied the request|AdmissionPolicy '[^']*'.* denied request|failed calling webhook`)

// platformRefusal reports whether err is the API server refusing one of the
// operator's own requests (w9/090): an admission policy or webhook denial,
// RBAC (Forbidden), or a schema check (Invalid). The customer cannot act on any
// of them, and their text names internal policies, the tenant namespace and
// the operator's own objects. A ResourceQuota refusal is the exception: it
// names the workspace plan's limit, which the customer can act on.
func platformRefusal(err error) bool {
	if err == nil || isQuotaExceeded(err) {
		return false
	}
	if apierrors.IsForbidden(err) || apierrors.IsInvalid(err) {
		return true
	}
	var status apierrors.APIStatus
	return errors.As(err, &status) && admissionDenial.MatchString(status.Status().Message)
}

// recordedPlatformRefusal is platformRefusal for a refusal another controller
// recorded as text: a ReplicaSet's FailedCreate, which the Deployment copies
// into its ReplicaFailure condition, reads `pods "<name>" is forbidden: …`.
func recordedPlatformRefusal(msg string) bool {
	if strings.Contains(msg, "exceeded quota") {
		return false
	}
	return strings.Contains(msg, " is forbidden: ") || strings.Contains(msg, " is invalid: ") || admissionDenial.MatchString(msg)
}

// platformRefusalReference is the short reference the customer quotes to
// support: the release's generation and a hash of the App's UID with the raw
// refusal. The release's, not metadata.generation, so a settings save between
// two retries of one refusal keeps the reference; the condition then does not
// churn, and support finds the raw text in the operator log under it.
func platformRefusalReference(app *appv1alpha1.App, raw string) string {
	gen := releaseGeneration(app)
	if gen == 0 {
		gen = app.Generation
	}
	sum := sha256.Sum256([]byte(string(app.UID) + "\x00" + raw))
	return fmt.Sprintf("g%d-%s", gen, hex.EncodeToString(sum[:4]))
}

// logPlatformRefusal logs the raw refusal under ref: at error level when the
// App's status does not show ref yet, and at V(1) on a retry whose status
// already does, so each backoff retry of one refusal is not another error line.
func logPlatformRefusal(ctx context.Context, app *appv1alpha1.App, reason, ref string, refusal error) {
	log := logf.FromContext(ctx).WithValues("reference", ref, "app", app.Name, "namespace", app.Namespace,
		"generation", app.Generation, "releaseGeneration", releaseGeneration(app), "reason", reason)
	if statusShows(app, ref) {
		log.V(1).Info("the API server refused the operator's request again; the customer already sees this reference", "refusal", refusal.Error())
		return
	}
	log.Error(refusal, "the API server refused the operator's request; the customer sees a platform error with this reference")
}

// statusShows reports whether a condition or the pre-deploy verdict on app's
// status already carries ref.
func statusShows(app *appv1alpha1.App, ref string) bool {
	for _, c := range app.Status.Conditions {
		if strings.Contains(c.Message, ref) {
			return true
		}
	}
	return app.Status.PreDeploy != nil && strings.Contains(app.Status.PreDeploy.Message, ref)
}

// customerFailure is err's text as the App's status may show it to the
// customer, and whether that is the platform-error sentence. A platform
// refusal becomes the fixed sentence with its reference, and the raw refusal is
// logged under the same reference; any other error keeps its own text, so an
// image pull, quota, health check, crash or build failure reads as before.
func customerFailure(ctx context.Context, app *appv1alpha1.App, reason string, err error) (string, bool) {
	if !platformRefusal(err) {
		return err.Error(), false
	}
	ref := platformRefusalReference(app, err.Error())
	logPlatformRefusal(ctx, app, reason, ref, err)
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPlatformError,
		appv1alpha1.DeployReasonParam{Name: "ref", Value: ref}), true
}

// recordedRefusalReference is customerFailure for a refusal recorded as text
// (recordedPlatformRefusal): the reference to show in its place, and whether
// msg was one.
func recordedRefusalReference(ctx context.Context, app *appv1alpha1.App, reason, msg string) (string, bool) {
	if !recordedPlatformRefusal(msg) {
		return "", false
	}
	ref := platformRefusalReference(app, msg)
	logPlatformRefusal(ctx, app, reason, ref, errors.New(msg))
	return ref, true
}
