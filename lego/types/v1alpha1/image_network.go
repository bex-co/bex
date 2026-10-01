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

import "fmt"

const (
	// PortModeImageV1 opts a new private service into verified image TCP metadata.
	PortModeImageV1 = "image-v1"
	// PortModeImageConfiguredV1 keeps image TCP ports while honoring spec.port as
	// the explicitly selected primary listener, including an explicit 3000.
	PortModeImageConfiguredV1 = "image-configured-v1"
)

// MaxImagePorts bounds a private service's TCP port set; it mirrors the
// MaxItems marker on ImageNetworkStatus.Ports, which must stay a literal.
const MaxImagePorts = 75

// IsReservedImagePort keeps explicit primary ports and discovered TCP ports
// under the same image-compatibility policy.
func IsReservedImagePort(port int32) bool {
	switch port {
	case 18012, 18013, 19099:
		return true
	default:
		return false
	}
}

// ImageNetworkStatus binds a private port set to an immutable image and release.
type ImageNetworkStatus struct {
	Image    string `json:"image"`
	Revision string `json:"revision"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	PrimaryPort int32 `json:"primaryPort"`
	// +kubebuilder:validation:MaxItems=75
	// +kubebuilder:validation:MinItems=1
	Ports []int32 `json:"ports"`
}

// UsesImagePorts never expands the public routing contract or legacy services.
func (s AppSpec) UsesImagePorts() bool {
	return s.Type == TypePrivateService && (s.PortMode == PortModeImageV1 || s.PortMode == PortModeImageConfiguredV1)
}

// ActiveImageNetwork is the serving release's port set, else the first
// candidate's before anything has served; nil for configured-port services.
func (a *App) ActiveImageNetwork() *ImageNetworkStatus {
	if !a.Spec.UsesImagePorts() {
		return nil
	}
	if a.Status.ServingNetwork != nil {
		return a.Status.ServingNetwork
	}
	return a.Status.ImageNetwork
}

// ServicePort is the published primary port; zero means discovery is pending.
func (a *App) ServicePort() int32 {
	if !a.Spec.UsesImagePorts() {
		return a.Spec.EffectivePort()
	}
	if network := a.ActiveImageNetwork(); network != nil {
		return network.PrimaryPort
	}
	return 0
}

// InternalAddress returns the Render-shaped private-network address sibling
// services connect to — "<slug>:<port>", scheme-less — or "" when the type is
// not addressable. This is the D2 resolvability contract in one place
// (docs/ADR041-service-addresses.md): the operator's slug-named Service answers
// exactly this hostname, and bex-api surfaces exactly this string. It follows
// the serving release rather than a pending image's port choice, and before
// initial discovery no invented hostname:3000 is returned.
func (a *App) InternalAddress() string {
	port := a.ServicePort()
	if !a.Spec.InternallyAddressable() || port == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", a.Spec.PlatformSubdomain(a.Name), port)
}
