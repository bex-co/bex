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

// HTTP listener decisions are shared by runtime projection and deploy history.
const (
	HTTPSelectionDetected   = "http"
	HTTPSelectionFallback   = "fallback"
	HTTPSelectionRetained   = "retained"
	HTTPSelectionConfigured = "configured"
	ReasonHTTPPortDiscovery = "HTTPPortDiscovery"
)

// MaxImagePorts bounds a private service's TCP port set; it mirrors the
// MaxItems marker on ImageNetworkStatus.Ports, which must stay a literal.
const MaxImagePorts = 75

// IsConfiguredPortMode reports a port mode whose port the owner set
// explicitly: the image's own ports still apply, with this one as the primary.
func IsConfiguredPortMode(mode string) bool {
	return mode == PortModeImageConfiguredV1 || mode == PortModeWebImageConfiguredV1
}

// ReservedImagePorts are the ports bex's own processes listen on inside a
// tenant pod; no service port may be one of them.
func ReservedImagePorts() []int32 { return slices.Clone(reservedImagePorts) }

var reservedImagePorts = []int32{18012, 18013, 18014, 19099}

// IsReservedImagePort keeps explicit primary ports and discovered TCP ports
// under the same image-compatibility policy.
func IsReservedImagePort(port int32) bool {
	return slices.Contains(reservedImagePorts, port)
}

// ImageNetworkStatus binds a private port set to an immutable image and release.
type ImageNetworkStatus struct {
	Image string `json:"image"`
	// ConfiguredPort records the configuration used to resolve this network:
	// zero means no configured port, a positive value is the explicit PORT or
	// service port, and nil is legacy data whose provenance must be refreshed.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	ConfiguredPort *int32 `json:"configuredPort,omitempty"`
	// HTTPSelection is the completed web listener decision. Empty means the
	// first candidate still needs runtime discovery; private networks ignore it.
	// +optional
	// +kubebuilder:validation:Enum=http;fallback;retained;configured
	HTTPSelection string `json:"httpSelection,omitempty"`
	// NoDeclaredPorts distinguishes a runtime/default fallback from OCI EXPOSE.
	// +optional
	NoDeclaredPorts bool `json:"noDeclaredPorts,omitempty"`
	// ProcessPort is the PORT this network's release gives its process:
	// Render's 10000 for a web service whose image chooses its own HTTP port,
	// whatever port it declares or discovery selects (w4/615). Zero means the
	// primary: a configured or private network, or a web network resolved
	// before w4/615 (ProcessPORT).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	ProcessPort int32 `json:"processPort,omitempty"`

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

// ProcessPORT is the PORT the network's release gives its process:
// ProcessPort, else the primary.
func (n *ImageNetworkStatus) ProcessPORT() int32 {
	if n.ProcessPort > 0 {
		return n.ProcessPort
	}
	return n.PrimaryPort
}

// PrivateHTTPPort is the private-network port on which every web service
// answers with its primary HTTP server, whatever port that server binds
// (render.com/docs/private-network; w3/m273).
const PrivateHTTPPort int32 = 10000

// RuntimePortsStatus is what the operator's listener scan of a web or private
// service's running instance found (w3/m273, w3/m276; ADR089 D3 phase 2).
type RuntimePortsStatus struct {
	// Revision is the pod revision whose Ready instance was scanned.
	Revision string `json:"revision"`
	// Ports are the TCP ports that instance accepted connections on.
	// +optional
	// +kubebuilder:validation:MaxItems=75
	Ports []int32 `json:"ports,omitempty"`
	// Retained are the previous revision's listeners that Revision does not
	// serve, kept published until the rollout replacing it completes.
	// +optional
	// +kubebuilder:validation:MaxItems=75
	Retained []int32 `json:"retained,omitempty"`
}

// RuntimeOnlyPorts is the discovered listeners of a web or private service
// that the published set lacks, ascending: reserved ports are left out, a web
// service's 10000 (a route to its primary HTTP server, whatever binds it) too,
// and the result stops where published plus discovered would pass Render's 75
// open ports. Other types publish none (a stale record from a type change
// included).
func (a *App) RuntimeOnlyPorts(published []int32) []int32 {
	found := a.Status.RuntimePorts
	if !a.Spec.InternallyAddressable() || found == nil {
		return nil
	}
	web := a.Spec.Type != TypePrivateService
	var out []int32
	for _, port := range found.listening() {
		if len(published)+len(out) >= MaxImagePorts {
			break
		}
		if port < 1 || port > 65535 || IsReservedImagePort(port) || web && port == PrivateHTTPPort || slices.Contains(published, port) {
			continue
		}
		out = append(out, port)
	}
	return out
}

// listening is every port an instance of either revision accepts, ascending.
func (s *RuntimePortsStatus) listening() []int32 {
	ports := slices.Concat(s.Ports, s.Retained)
	slices.Sort(ports)
	return slices.Compact(ports)
}

// scannedListeners is what the instances serving the listed image network
// bind, once a scan has run for its revision or for the candidate rolling out
// beside it (whose record retains the serving instances' ports). ok is false
// before then: an unscanned release lists what its image declares.
func (a *App) scannedListeners() (ports []int32, ok bool) {
	found, network := a.Status.RuntimePorts, a.ActiveImageNetwork()
	if found == nil || network == nil {
		return nil, false
	}
	if found.Revision != network.Revision && (a.Status.ImageNetwork == nil || found.Revision != a.Status.ImageNetwork.Revision) {
		return nil, false
	}
	return found.listening(), true
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

// ConfiguredImagePort is an image-port service's configured primary: its own
// PORT env (Status.EnvPort), which wins, else spec.port in a configured mode.
// ok is false while the image chooses (w4/m243). A private service's PORT
// selects its primary as a web service's does (w3/m273 t010): Render serves a
// private service on the port it binds, whatever its image declares.
func (a *App) ConfiguredImagePort() (port int32, ok bool) {
	if a.Spec.HasImagePorts() && a.Status.EnvPort > 0 {
		return a.Status.EnvPort, true
	}
	if a.Spec.HasImagePorts() && IsConfiguredPortMode(a.Spec.PortMode) {
		return a.Spec.Port, true
	}
	return 0, false
}

// ServingPort is the port a configured (non-image-port) service listens on:
// the port its own PORT environment variable selected, as the operator
// recorded it, else an explicit non-default spec.port (w5/m149), else the detected
// listener (w8/177, w8/202), else the default.
func (a *App) ServingPort() int32 {
	// Image-port services publish through the serving image network instead of
	// any recorded PORT or detected listener.
	if a.Spec.HasImagePorts() {
		return a.Spec.EffectivePort()
	}
	if a.Status.EnvPort > 0 {
		return a.Status.EnvPort
	}
	if a.Status.DetectedPort > 0 && (a.Spec.Port <= 0 || a.Spec.Port == DefaultPort) {
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
	// Before discovery the service's PORT env is the only answer.
	return a.Status.EnvPort
}

// OpenPorts is the TCP port set the stable hostnames serve: Render's
// serviceDetails.openPorts (w6/m148). It is nil for types without a network
// address, the effective port for a configured service, and the serving (else
// first candidate) image network's ports for an image-port service. While that
// service's discovery is pending it is empty, never an invented 3000. A web or
// private service adds the undeclared listeners found on its instances
// (w3/m273, w3/m276), and once its release has been scanned an image-declared
// port nothing listens on is left out (the Services still route it while the
// image declares it); the primary always stays. A web service's port-10000
// alias is a route, not a listener, and is not listed.
func (a *App) OpenPorts() []int32 {
	ports := a.ownPorts()
	if len(ports) == 0 {
		return ports
	}
	extra := a.RuntimeOnlyPorts(ports)
	if listening, ok := a.scannedListeners(); ok {
		primary := a.ServicePort()
		ports = slices.DeleteFunc(ports, func(p int32) bool { return p != primary && !slices.Contains(listening, p) })
	}
	if len(extra) > 0 {
		ports = slices.Concat(ports, extra)
		slices.Sort(ports)
	}
	return ports
}

// ownPorts is OpenPorts before runtime discovery: what the service's
// configuration or image declares.
func (a *App) ownPorts() []int32 {
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
	// already carries it as primary. A private service's PORT names only its
	// primary: the network carries it beside every other port the image
	// declares, and all of them stay open (w3/m273 t010).
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
