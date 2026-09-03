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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// Server defaults: what a projection must write so a converged reconcile is a
// true no-op (w7/m84).
//
// Every projection in this package is the mutate body of a
// controllerutil.CreateOrUpdate: it rebuilds the fields it owns on a freshly
// fetched copy of the stored object, and CreateOrUpdate issues a PUT unless the
// result DeepEquals what it fetched. The API server fills in a long list of
// optional pod-template fields on write, so a projection that rebuilds a
// container (or a whole PodSpec) without them can never equal the stored
// object — and every reconcile of an unchanged App, KeyValue or Database
// re-issues the same PUT, forever, once per object per pass.
//
// Two things measured in w7/m84/t001 shape this fix:
//
//   - The redundant PUT is not a redundant *write*. The API server re-defaults
//     the incoming object, finds the result byte-identical to what is stored,
//     and skips etcd — so resourceVersion never moves and no watch event fires.
//     The self-sustaining requeue loop `.pm/w7/done/028.md` predicted through
//     ResourceVersionChangedPredicate therefore never actually closed. The cost
//     is one full decode/validate/admission round trip per object per reconcile,
//     scaling with tenant count, on the API server's mutating path.
//   - That same fact is why writing these defaults CANNOT roll a pod: the stored
//     object is byte-identical before and after this change, so the pod-template
//     hash the Deployment controller computes from it cannot move.
//
// Approach. Two alternatives were weighed. controllerutil.CreateOrPatch does not
// help: it diffs the same in-memory object before and after the mutate, so a
// mutate that drops a server-defaulted field still registers as a change and
// still sends a request — it changes the verb, not the request count. Server-side
// apply removes the need to know the defaults, but sends a request on every
// reconcile by construction, which is precisely what this milestone is removing.
// So the projections mirror the defaults, exactly as w9/m57 did for the one
// Service field it fixed.
//
// The cost of mirroring is that these values must track the API server. They
// mirror k8s.io/kubernetes/pkg/apis/core/v1/defaults.go, and every one of them
// is re-derived from a live API server, field by named field, in
// server_defaults_envtest_test.go: a Kubernetes release that changes one fails a
// named test instead of silently resuming the churn.
//
// Pod templates are the bulk of it but not all of it — applyServicePortServerDefaults
// below covers the Service side, which is where w9/m57 first met this bug.
const (
	// defaultTerminationGracePeriodSeconds tracks the API module rather than
	// restating 30, so a k8s.io/api bump carries it.
	defaultTerminationGracePeriodSeconds int64 = corev1.DefaultTerminationGracePeriodSeconds
	// defaultProbeSuccessThreshold / FailureThreshold / PeriodSeconds /
	// TimeoutSeconds are the four probe defaults. Zero is not a legal explicit
	// value for any of them, so filling a zero can never overwrite a choice.
	defaultProbeSuccessThreshold int32 = 1
	defaultProbeFailureThreshold int32 = 3
	defaultProbePeriodSeconds    int32 = 10
	defaultProbeTimeoutSeconds   int32 = 1
)

// applyPodSpecServerDefaults fills in every field of spec the API server would
// have defaulted, leaving anything already set untouched. Idempotent: applying
// it to a stored PodSpec changes nothing.
//
// It is called at the END of a projection, so the projection stays readable —
// it says what bex chooses, and this says what Kubernetes would have chosen.
func applyPodSpecServerDefaults(spec *corev1.PodSpec) {
	if spec.TerminationGracePeriodSeconds == nil {
		spec.TerminationGracePeriodSeconds = new(defaultTerminationGracePeriodSeconds)
	}
	if spec.RestartPolicy == "" {
		// Always is the POD default and is INVALID on a Job/CronJob template, so a
		// Job projection that forgets to set Never gets a 422 here rather than
		// silently churning. Every current one sets it.
		spec.RestartPolicy = corev1.RestartPolicyAlways
	}
	if spec.DNSPolicy == "" {
		spec.DNSPolicy = corev1.DNSClusterFirst
	}
	if spec.SchedulerName == "" {
		spec.SchedulerName = corev1.DefaultSchedulerName
	}
	if spec.SecurityContext == nil {
		spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	for i := range spec.InitContainers {
		applyContainerServerDefaults(&spec.InitContainers[i])
	}
	for i := range spec.Containers {
		applyContainerServerDefaults(&spec.Containers[i])
	}
	for i := range spec.Volumes {
		applyVolumeServerDefaults(&spec.Volumes[i])
	}
}

// applyContainerServerDefaults fills in the container-level defaults.
func applyContainerServerDefaults(c *corev1.Container) {
	if c.TerminationMessagePath == "" {
		c.TerminationMessagePath = corev1.TerminationMessagePathDefault
	}
	if c.TerminationMessagePolicy == "" {
		c.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	}
	if c.ImagePullPolicy == "" {
		c.ImagePullPolicy = serverDefaultPullPolicy(c.Image)
	}
	for i := range c.Ports {
		if c.Ports[i].Protocol == "" {
			c.Ports[i].Protocol = corev1.ProtocolTCP
		}
	}
	applyProbeServerDefaults(c.StartupProbe)
	applyProbeServerDefaults(c.ReadinessProbe)
	applyProbeServerDefaults(c.LivenessProbe)
}

// applyProbeServerDefaults fills in the probe-level defaults, nil-safe.
func applyProbeServerDefaults(p *corev1.Probe) {
	if p == nil {
		return
	}
	if p.SuccessThreshold == 0 {
		p.SuccessThreshold = defaultProbeSuccessThreshold
	}
	if p.FailureThreshold == 0 {
		p.FailureThreshold = defaultProbeFailureThreshold
	}
	if p.PeriodSeconds == 0 {
		p.PeriodSeconds = defaultProbePeriodSeconds
	}
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = defaultProbeTimeoutSeconds
	}
	if p.HTTPGet != nil && p.HTTPGet.Scheme == "" {
		p.HTTPGet.Scheme = corev1.URISchemeHTTP
	}
}

// applyVolumeServerDefaults fills in the file-mode defaults of the volume
// sources whose whole defaulting surface is that one field.
//
// downwardAPI is deliberately absent: the API server also defaults
// fieldRef.apiVersion on each of its items, so filling only the mode would leave
// the projection divergent anyway. Nothing here emits one; a projection that
// starts to must extend this function, and the convergence invariant is what
// says so.
func applyVolumeServerDefaults(v *corev1.Volume) {
	switch {
	case v.Secret != nil && v.Secret.DefaultMode == nil:
		v.Secret.DefaultMode = ptr.To(corev1.SecretVolumeSourceDefaultMode)
	case v.ConfigMap != nil && v.ConfigMap.DefaultMode == nil:
		v.ConfigMap.DefaultMode = ptr.To(corev1.ConfigMapVolumeSourceDefaultMode)
	case v.Projected != nil && v.Projected.DefaultMode == nil:
		v.Projected.DefaultMode = ptr.To(corev1.ProjectedVolumeSourceDefaultMode)
	}
}

// applyServicePortServerDefaults fills in the ServicePort defaults. Small, but
// it is the field w9/m57 fixed by hand for one Service and this milestone found
// unfixed on the next one — three construction sites in the operator, now one
// rule, so a fourth Service projection is convergent by construction.
func applyServicePortServerDefaults(ports []corev1.ServicePort) {
	for i := range ports {
		if ports[i].Protocol == "" {
			ports[i].Protocol = corev1.ProtocolTCP
		}
	}
}

// serverDefaultPullPolicy reproduces the API server's imagePullPolicy default:
// an image referring to the "latest" tag — written out or implied by omitting a
// tag entirely — pulls Always; any other tag, and any digest, pulls
// IfNotPresent.
//
// This is deliberately NOT pullPolicyFor. That function is bex's own policy
// (always re-pull unless the reference is digest-pinned) and applies where a
// projection CHOOSES the value; this one applies where a projection does not,
// and exists only to write down what the server would have written. Using
// bex's policy here would change the stored pod template of every projection
// that currently leaves the field empty.
func serverDefaultPullPolicy(image string) corev1.PullPolicy {
	// A digest reference is never "latest". The API server reaches the same
	// conclusion through the reference parser; so does an unparseable image,
	// which cannot be stored anyway because the field is required.
	if image == "" || strings.Contains(image, "@") {
		return corev1.PullIfNotPresent
	}
	tag := "latest" // an omitted tag IS latest, per the reference parser
	if colon := strings.LastIndexByte(image, ':'); colon > strings.LastIndexByte(image, '/') {
		tag = image[colon+1:]
	}
	if tag == "latest" {
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
}
