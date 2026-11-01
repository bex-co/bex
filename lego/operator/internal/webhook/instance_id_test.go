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
	"strings"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/bex-co/bex/lego/types/renderenv"
)

func admitPod(t *testing.T, pod *corev1.Pod) admission.Response {
	t.Helper()
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	return InstanceIDStamper{}.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create, Object: runtime.RawExtension{Raw: raw},
	}})
}

func patched(t *testing.T, pod *corev1.Pod, resp admission.Response) *corev1.Pod {
	t.Helper()
	raw, _ := json.Marshal(pod)
	ops, _ := json.Marshal(resp.Patches)
	decoded, err := jsonpatch.DecodePatch(ops)
	if err != nil {
		t.Fatal(err)
	}
	p, err := decoded.Apply(raw)
	if err != nil {
		t.Fatal(err)
	}
	var out corev1.Pod
	if err := json.Unmarshal(p, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func tenantPod(name, generateName, serviceID string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, GenerateName: generateName, Labels: map[string]string{"app.bex.co/app": "tea-x-web"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	if serviceID != "" {
		pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "PORT", Value: "10000"}, {Name: renderenv.ServiceID, Value: serviceID}}
	}
	return pod
}

// w4/386: a ReplicaSet pod arrives with only generateName; the stamper picks
// the name and stamps the id derived from it, so RENDER_INSTANCE_ID equals the
// instance id every API surface reports for that pod.
func TestInstanceIDStamperNamesAndStampsAGeneratedPod(t *testing.T) {
	pod := tenantPod("", "tea-x-web-5ff6c9466c-", "srv-dbcad3603oos73cqipm0")
	resp := admitPod(t, pod)
	if !resp.Allowed || len(resp.Patches) == 0 {
		t.Fatalf("want an allowed patch, got %+v", resp)
	}
	out := patched(t, pod, resp)
	if !strings.HasPrefix(out.Name, "tea-x-web-5ff6c9466c-") || len(out.Name) != len("tea-x-web-5ff6c9466c-")+5 {
		t.Fatalf("name = %q, want generateName plus 5 characters", out.Name)
	}
	if got, want := out.Annotations[renderenv.InstanceIDAnnotation], renderenv.PublicInstanceID("srv-dbcad3603oos73cqipm0", out.Name); got != want {
		t.Fatalf("instance id annotation = %q, want %q", got, want)
	}
}

func TestInstanceIDStamperUsesAnExplicitName(t *testing.T) {
	pod := tenantPod("predeploy-tea-x-web-093ac3bvvcs9", "", "srv-abc")
	out := patched(t, pod, admitPod(t, pod))
	if out.Name != pod.Name {
		t.Fatalf("explicit name changed to %q", out.Name)
	}
	if out.Annotations[renderenv.InstanceIDAnnotation] != renderenv.PublicInstanceID("srv-abc", pod.Name) {
		t.Fatalf("annotation = %q", out.Annotations[renderenv.InstanceIDAnnotation])
	}
}

func TestInstanceIDStamperLeavesOtherPodsAlone(t *testing.T) {
	for name, pod := range map[string]*corev1.Pod{
		"no service id":  tenantPod("", "x-", ""),
		"no name at all": tenantPod("", "", "srv-abc"),
	} {
		resp := admitPod(t, pod)
		if !resp.Allowed || len(resp.Patches) != 0 {
			t.Errorf("%s: want allowed with no patch, got %+v", name, resp)
		}
	}
	// Already stamped with the right id: nothing to change.
	pod := tenantPod("web-1", "", "srv-abc")
	pod.Annotations = map[string]string{renderenv.InstanceIDAnnotation: renderenv.PublicInstanceID("srv-abc", "web-1")}
	if resp := admitPod(t, pod); !resp.Allowed || len(resp.Patches) != 0 {
		t.Errorf("stamped pod: want no patch, got %+v", resp)
	}
}
