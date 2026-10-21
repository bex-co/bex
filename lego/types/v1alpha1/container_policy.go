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

package v1alpha1

const (
	// ContainerPolicyStrictV1 is the explicit spelling of the default hardened
	// policy every pre-ADR089 service runs under ("" means the same).
	ContainerPolicyStrictV1 = "strict-v1"
	// ContainerPolicyImageV1 supports ordinary root image initialization and
	// privilege dropping without privileged mode or arbitrary tenant capabilities.
	ContainerPolicyImageV1 = "image-v1"
	// ContainerPolicyRenderV1 adds exactly the capability set Render's runtime
	// grants, measured live (ADR089 § Privilege slice decisions, w6/m149):
	// every image that starts on Render starts on bex unchanged.
	ContainerPolicyRenderV1 = "render-v1"
)

// ContainerPolicyCapabilities is the one table of capabilities a policy adds
// on top of dropping ALL: the operator projects it, and the workload admission
// policy's lists are tested against it. Strict (and "") adds none.
func ContainerPolicyCapabilities(policy string) []string {
	switch policy {
	case ContainerPolicyImageV1:
		return []string{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID"}
	case ContainerPolicyRenderV1:
		return []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID", "SYS_CHROOT"}
	default:
		return nil
	}
}

// ContainerPolicySeccompProfile is the kubelet Localhost seccomp profile path a
// policy runs under (relative to /var/lib/kubelet/seccomp), or "" for
// RuntimeDefault. render-v1's profile is RuntimeDefault plus the
// user-namespace and io_uring syscalls Render's unfiltered runtime permits
// (ADR089 § Seccomp, w6/m150); the seccomp-profiles DaemonSet installs it.
func ContainerPolicySeccompProfile(policy string) string {
	if policy == ContainerPolicyRenderV1 {
		return "profiles/bex-render-v1.json"
	}
	return ""
}
