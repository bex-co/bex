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
)
