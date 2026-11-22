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

import "slices"

// PortScan is what a deploy's in-flight stall reason says about the port scan
// Render narrates in its deploy log ("==> No open ports detected, continuing
// to scan...", w8/385).
type PortScan int

const (
	// PortScanSilent says nothing either way: HTTP discovery is waiting or
	// between two reads, or a check stalls because the one port it connects
	// to is closed, which says nothing about the instance's other ports.
	PortScanSilent PortScan = iota
	// PortScanNoOpenPort is a scan that found no open port on the instance.
	PortScanNoOpenPort
	// PortScanLocalhostOnly is a scan that found no open port on the instance's
	// address but some on localhost, which Render names (w8/388): "No open
	// ports detected on 0.0.0.0" and "Detected open ports on localhost".
	PortScanLocalhostOnly
	// PortScanNoHTTPPort is a web service's scan that found open ports, none
	// answering HTTP: Render's "No open HTTP ports detected on 0.0.0.0".
	PortScanNoHTTPPort
	// PortScanOver ends a scan: a port was found, or the stall stopped saying
	// that no port is open. The rollout moved on ("" is no stall), a check now
	// fails with its port open (a Health Check Path that errors leaves the
	// operator's port hint off), or the stall is about something else.
	PortScanOver
)

// Scanning reports a stall that says the scan found no port to serve.
func (s PortScan) Scanning() bool {
	return s == PortScanNoOpenPort || s == PortScanLocalhostOnly || s == PortScanNoHTTPPort
}

// PortScanFinding is what a running scan found besides no port to serve, as
// the deploy row records it (w8/388); "" is neither.
type PortScanFinding string

const (
	PortScanFoundLocalhost PortScanFinding = "localhost"
	PortScanFoundNoHTTP    PortScanFinding = "no_http"
)

// Found is s's finding as the deploy row records it.
func (s PortScan) Found() PortScanFinding {
	switch s {
	case PortScanLocalhostOnly:
		return PortScanFoundLocalhost
	case PortScanNoHTTPPort:
		return PortScanFoundNoHTTP
	}
	return ""
}

// noOpenPortHints are the stall hints that say the scan found no open port.
var noOpenPortHints = []string{
	DeployReasonHintNoOpenPort, DeployReasonHintNoOpenPortBefore, DeployReasonHintNoOpenServicePort,
}

// listeningHints are the stall hints that name a port the process listens on.
var listeningHints = []string{
	DeployReasonHintListening, DeployReasonHintListeningAdopted, DeployReasonHintListeningPorts, DeployReasonHintListeningPrimary,
}

// closedPortHints are the stall hints that say only that the checked port is
// closed: an image service's primary, whose other ports nobody read.
var closedPortHints = []string{
	DeployReasonHintImageDefaultPort, DeployReasonHintImageDeclaredPorts, DeployReasonHintEnvPortOnly,
	DeployReasonHintConfiguredPortOnly, DeployReasonHintEnvPortDeclared, DeployReasonHintConfiguredPortDeclared,
}

// PortScanOf reads stall, a deploy's in-flight stall reason, for the port
// scan. Render's default check, a TCP connect to any open port, stalls only
// while no port is open, so its stall counts as no open port whatever its
// hints. Any other check counts only with the operator's no-open-port hint
// (port_detect.go), which a pass between two scans replays. The operator's
// localhost and no-HTTP findings name their case whatever the check. HTTP
// discovery waiting on a port that accepted without answering ends the scan,
// as any found port does: it writes that only beside a port that answers, or
// with no listener read to tell (selectWebImageHTTP).
func PortScanOf(stall string) PortScan {
	code, params, ok := ParseDeployReason(stall)
	if !ok {
		return PortScanOver
	}
	switch code {
	case DeployReasonHTTPPortNoneYet:
		return PortScanNoOpenPort
	case DeployReasonHTTPPortLocalhostOnly:
		return PortScanLocalhostOnly
	case DeployReasonHTTPPortNoneHTTP:
		return PortScanNoHTTPPort
	case DeployReasonHTTPPortWaiting, DeployReasonHTTPPortDetecting, DeployReasonHTTPPortFallback:
		return PortScanSilent
	}
	if !slices.Contains(probeStallCodes, code) {
		return PortScanOver
	}
	var hints []string
	for _, p := range params {
		if p.Name == "hint" {
			parts, _ := ParseDeployReasonParts(p.Value)
			for _, part := range parts {
				hints = append(hints, part.Code)
			}
		}
	}
	anyOf := func(codes []string) bool {
		return slices.ContainsFunc(hints, func(h string) bool { return slices.Contains(codes, h) })
	}
	switch {
	case anyOf(listeningHints):
		return PortScanOver
	case slices.Contains(hints, DeployReasonHintNoOpenPortLocalhost):
		return PortScanLocalhostOnly
	case slices.Contains(hints, DeployReasonHintNoOpenHTTPPort):
		return PortScanNoHTTPPort
	case anyOf(noOpenPortHints), slices.Contains(openPortProbeCodes, code):
		return PortScanNoOpenPort
	case anyOf(closedPortHints):
		return PortScanSilent
	}
	return PortScanOver
}

// openPortProbeCodes are the in-flight stalls of Render's default check.
var openPortProbeCodes = []string{DeployReasonProbeOpenPortWaiting, DeployReasonProbeOpenPortWaitingRestarted}

// probeStallCodes are the in-flight health-check stalls, the ones a {hint}
// rides on.
var probeStallCodes = append([]string{
	DeployReasonProbeHTTPWaiting, DeployReasonProbeHTTPWaitingRestarted,
	DeployReasonProbeTCPWaiting, DeployReasonProbeTCPWaitingRestarted,
	DeployReasonProbeOtherWaiting, DeployReasonProbeOtherWaitingRestarted,
}, openPortProbeCodes...)
