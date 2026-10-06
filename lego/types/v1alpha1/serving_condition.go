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

// ConditionServing reports a live observation of the active revision's pods
// during a Deployment rollout. Ready continues to describe rollout progress.
//
// True (ReasonPriorReleaseServing): at least one active-revision pod is
// ready. False (ReasonServingRevisionUnavailable): the pods were listed and
// none is ready, so the condition's transition is when the serving revision
// stopped, which bex-api dates a crash under an open deploy by (w5/083).
// Unknown (ReasonServingUnobserved): not observed, or nothing to observe.
const ConditionServing = "Serving"

const (
	ReasonServingRevisionUnavailable = "ServingRevisionUnavailable"
	ReasonServingUnobserved          = "ServingUnobserved"
)
