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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// appSecCtx applies a service's durable policy to every path executing its
// image: Deployment, CronJob, one-off cron run and pre-deploy Job. Platform
// containers and managed datastores continue to use tenantSecCtx directly.
func appSecCtx(spec appv1alpha1.AppSpec) *corev1.SecurityContext {
	security := tenantSecCtx()
	if spec.ContainerPolicy == appv1alpha1.ContainerPolicyImageV1 {
		security.Capabilities.Add = []corev1.Capability{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID"}
	}
	return security
}
