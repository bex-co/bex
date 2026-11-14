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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/bex-co/bex/lego/types/renderenv"
)

// InstanceIDPath is where the API server sends tenant pod creations for
// InstanceIDStamper.
const InstanceIDPath = "/mutate-v1-pod-instance-id"

// InstanceIDStamper is a mutating admission webhook that stamps a tenant pod
// with its public instance id, so RENDER_INSTANCE_ID (read from the annotation
// through the downward API) equals the id GET /v1/services/{id}/instances, logs
// and metrics report for it (w4/386). The id is derived from the pod name, which
// a ReplicaSet or Job leaves to generateName; the API server fills a missing
// name only after mutating admission and keeps one a webhook set, so the stamper
// picks the name itself when it is still empty. The service id is the pod's own
// literal RENDER_SERVICE_ID, which web, cron and pre-deploy pods all carry.
type InstanceIDStamper struct{}

// Handle implements admission.Handler.
func (InstanceIDStamper) Handle(_ context.Context, req admission.Request) admission.Response {
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("decode pod: %w", err))
	}
	serviceID := podServiceID(&pod)
	if serviceID == "" {
		return admission.Allowed("no RENDER_SERVICE_ID")
	}
	if pod.Name == "" {
		if pod.GenerateName == "" {
			return admission.Allowed("pod has no name to derive from")
		}
		// The same 5-character suffix the API server's generator appends.
		pod.Name = pod.GenerateName + utilrand.String(5)
	}
	want := renderenv.PublicInstanceID(serviceID, pod.Name)
	// The process sees the same id as its hostname, as a Render instance's
	// hostname is named after its service (w4/629): frameworks name an
	// instance by it. A hostname the pod already sets is kept, and so is the
	// pod name's when the id is no DNS label.
	nameHost := pod.Spec.Hostname == "" && len(validation.IsDNS1123Label(want)) == 0
	if pod.Annotations[renderenv.InstanceIDAnnotation] == want && !nameHost {
		return admission.Allowed("instance id already stamped")
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[renderenv.InstanceIDAnnotation] = want
	if nameHost {
		pod.Spec.Hostname = want
	}
	stamped, err := json.Marshal(&pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("encode pod: %w", err))
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, stamped)
}

// podServiceID is the literal RENDER_SERVICE_ID the operator sets on the pod's
// containers, or "" when none carries one.
func podServiceID(pod *corev1.Pod) string {
	for _, containers := range [][]corev1.Container{pod.Spec.InitContainers, pod.Spec.Containers} {
		for _, c := range containers {
			for _, e := range c.Env {
				if e.Name == renderenv.ServiceID && e.ValueFrom == nil && e.Value != "" {
					return e.Value
				}
			}
		}
	}
	return ""
}

// RegisterInstanceID serves InstanceIDStamper on the manager's webhook server.
func RegisterInstanceID(mgr manager.Manager) {
	mgr.GetWebhookServer().Register(InstanceIDPath, &ctrlwebhook.Admission{Handler: InstanceIDStamper{}})
}
