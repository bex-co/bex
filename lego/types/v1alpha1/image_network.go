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

import (
	"fmt"
	"slices"
)

const (
	// PortModeImageV1 opts a new private service into verified image TCP metadata.
	PortModeImageV1 = "image-v1"
	// PortModeImageConfiguredV1 keeps image TCP ports while honoring spec.port as
	// the explicitly selected primary listener, including an explicit 3000.
	PortModeImageConfiguredV1 = "image-configured-v1"
	// PortModeWebImageV1 opts a new web service into serving on its image's
	// exposed port (w4/m243), as Render does for Docker web services.
	PortModeWebImageV1 = "web-image-v1"
	// PortModeWebImageConfiguredV1 is the web variant with an explicitly
	// selected primary port.
	PortModeWebImageConfiguredV1 = "web-image-configured-v1"
)

// MaxImagePorts bounds a private service's TCP port set; it mirrors the
// MaxItems marker on ImageNetworkStatus.Ports, which must stay a literal.
const MaxImagePorts = 75

// IsReservedImagePort keeps explicit primary ports and discovered TCP ports
// under the same image-compatibility policy.
func IsReservedImagePort(port int32) bool {
	switch port {
	case 18012, 18013, 18014, 19099:
		return true
	default:
		return false
	}
}

// ImageNetworkStatus binds a private port set to an immutable image and release.
type ImageNetworkStatus struct {
	Image string `json:"image"`
	// Source is the reference Image was resolved from, such as a prebuilt
	// image's tag, so one release reads the registry once (w4/m243 t008).
	// +optional
	Source   string `json:"source,omitempty"`
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

// UsesWebImagePorts reports a web service serving on its image's exposed
// port (w4/m243). Only services stamped at create carry the mode, so
// existing web services keep their configured-port behavior.
func (s AppSpec) UsesWebImagePorts() bool {
	return s.Type == TypeWebService && (s.PortMode == PortModeWebImageV1 || s.PortMode == PortModeWebImageConfiguredV1)
}

// HasImagePorts reports any service whose port set comes from image
// metadata: a private service's TCP set, or a web service's primary port.
func (s AppSpec) HasImagePorts() bool {
	return s.UsesImagePorts() || s.UsesWebImagePorts()
}

// ActiveImageNetwork is the serving release's port set, else the first
// candidate's before anything has served; nil for configured-port services.
func (a *App) ActiveImageNetwork() *ImageNetworkStatus {
	if !a.Spec.HasImagePorts() {
		return nil
	}
	if a.Status.ServingNetwork != nil {
		return a.Status.ServingNetwork
	}
	return a.Status.ImageNetwork
}

// ServingPort is the port a configured (non-image-port) service listens on:
// the port its own PORT environment variable selected, as the operator
// recorded it, else an explicit non-default spec.port (w5/m149), else the detected
// listener (w8/177, w8/202), else the default.
func (a *App) ServingPort() int32 {
	if a.Status.EnvPort > 0 && !a.Spec.HasImagePorts() {
		return a.Status.EnvPort
	}
	if a.Status.DetectedPort > 0 && (a.Spec.Port <= 0 || a.Spec.Port == DefaultPort) && !a.Spec.HasImagePorts() {
		return a.Status.DetectedPort
	}
	return a.Spec.EffectivePort()
}

// ServicePort is the published primary port; zero means discovery is pending.
func (a *App) ServicePort() int32 {
	if !a.Spec.HasImagePorts() {
		return a.ServingPort()
	}
	// The serving network already carries a PORT env's choice; a candidate's
	// PORT is published only once it is promoted (w4/m243 t007).
	if network := a.ActiveImageNetwork(); network != nil {
		return network.PrimaryPort
	}
	// Before discovery a web service's PORT env is the only answer; the
	// operator zeroes EnvPort for private image-port services.
	return a.Status.EnvPort
}

// OpenPorts is the TCP port set the stable hostnames serve: Render's
// serviceDetails.openPorts (w6/m148). It is nil for types without a network
// address, the effective port for a configured service, and the serving (else
// first candidate) image network's ports for an image-port private service.
// While that service's discovery is pending it is empty, never an invented 3000.
func (a *App) OpenPorts() []int32 {
	if !a.Spec.InternallyAddressable() {
		return nil
	}
	if !a.Spec.HasImagePorts() {
		return []int32{a.ServingPort()}
	}
	network := a.ActiveImageNetwork()
	if network == nil {
		if a.Status.EnvPort > 0 {
			return []int32{a.Status.EnvPort}
		}
		return []int32{}
	}
	// A web service's PORT names the one port it serves; the serving network
	// already carries it as primary.
	if a.Spec.UsesWebImagePorts() && a.Status.EnvPort > 0 {
		return []int32{network.PrimaryPort}
	}
	return slices.Clone(network.Ports)
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
