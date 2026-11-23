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
	"slices"
	"strconv"
	"strings"
)

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
	// PortScanEnvPort is a web service's scan for the one port its PORT
	// variable names, found closed. Render scans only that port once PORT is
	// set, and words it apart (w8/416): "Continuing to scan for open port
	// {port} (from PORT environment variable)".
	PortScanEnvPort
	// PortScanOver ends a scan: a port was found, or the stall stopped saying
	// that no port is open. The rollout moved on ("" is no stall), a check now
	// fails with its port open (a Health Check Path that errors leaves the
	// operator's port hint off), or the stall is about something else.
	PortScanOver
)

// Scanning reports a stall that says the scan found no port to serve.
func (s PortScan) Scanning() bool {
	return s == PortScanNoOpenPort || s == PortScanLocalhostOnly || s == PortScanNoHTTPPort || s == PortScanEnvPort
}

// PortScanFinding is what a running scan found besides no port to serve, as
// the deploy row records it (w8/388); "" is neither.
type PortScanFinding string

const (
	PortScanFoundLocalhost PortScanFinding = "localhost"
	PortScanFoundNoHTTP    PortScanFinding = "no_http"
	// envPortFinding prefixes the port a scan for PORT looks for
	// (PortScanFoundEnvPort).
	envPortFinding = "env_port:"
)

// PortScanFoundEnvPort is the finding of a scan for port, the one a web
// service's PORT variable names (w8/416): "env_port:8080".
func PortScanFoundEnvPort(port int) PortScanFinding {
	return PortScanFinding(envPortFinding + strconv.Itoa(port))
}

// EnvPort is the port a scan for PORT looks for, 0 for any other finding.
func (f PortScanFinding) EnvPort() int {
	rest, ok := strings.CutPrefix(string(f), envPortFinding)
	if !ok {
		return 0
	}
	if port, err := strconv.Atoi(rest); err == nil && port > 0 && port <= 65535 {
		return port
	}
	return 0
}

// ReadPortScan is PortScanOf(stall) with its finding as the deploy row records
// it: a listener only on localhost, open ports answering no HTTP, or the
// closed port PORT names; "" for none. One parse of the stall reads both.
func ReadPortScan(stall string) (PortScan, PortScanFinding) {
	scan, port := readPortScan(stall)
	switch scan {
	case PortScanLocalhostOnly:
		return scan, PortScanFoundLocalhost
	case PortScanNoHTTPPort:
		return scan, PortScanFoundNoHTTP
	case PortScanEnvPort:
		return scan, PortScanFoundEnvPort(port)
	}
	return scan, ""
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
// closed: an image service's primary, whose other ports nobody read. The env
// ones are a private image service's since w8/416.
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
// with no listener read to tell (selectWebImageHTTP). A web service's closed
// PORT is a scan of that port alone (w8/416).
func PortScanOf(stall string) PortScan {
	scan, _ := readPortScan(stall)
	return scan
}

// readPortScan is PortScanOf's reading of stall, with the port a scan for
// PORT looks for (0 for any other scan).
func readPortScan(stall string) (PortScan, int) {
	code, params, ok := ParseDeployReason(stall)
	if !ok {
		return PortScanOver, 0
	}
	switch code {
	case DeployReasonHTTPPortNoneYet:
		return PortScanNoOpenPort, 0
	case DeployReasonHTTPPortLocalhostOnly:
		return PortScanLocalhostOnly, 0
	case DeployReasonHTTPPortNoneHTTP:
		return PortScanNoHTTPPort, 0
	case DeployReasonHTTPPortWaiting, DeployReasonHTTPPortDetecting, DeployReasonHTTPPortFallback:
		return PortScanSilent, 0
	}
	if !slices.Contains(probeStallCodes, code) {
		return PortScanOver, 0
	}
	var hints []string
	envPort := 0
	for _, p := range params {
		if p.Name == "hint" {
			parts, _ := ParseDeployReasonParts(p.Value)
			for _, part := range parts {
				hints = append(hints, part.Code)
				if part.Code == DeployReasonHintEnvPortClosed && envPort == 0 {
					envPort = envPortOf(part)
				}
			}
		}
	}
	anyOf := func(codes []string) bool {
		return slices.ContainsFunc(hints, func(h string) bool { return slices.Contains(codes, h) })
	}
	switch {
	case anyOf(listeningHints):
		return PortScanOver, 0
	case slices.Contains(hints, DeployReasonHintNoOpenPortLocalhost):
		return PortScanLocalhostOnly, 0
	case slices.Contains(hints, DeployReasonHintNoOpenHTTPPort):
		return PortScanNoHTTPPort, 0
	case envPort > 0:
		return PortScanEnvPort, envPort
	case anyOf(noOpenPortHints), slices.Contains(openPortProbeCodes, code):
		return PortScanNoOpenPort, 0
	case anyOf(closedPortHints):
		return PortScanSilent, 0
	}
	return PortScanOver, 0
}

// envPortOf is the port a closed-PORT hint names, 0 for none.
func envPortOf(hint DeployReasonPart) int {
	for _, p := range hint.Params {
		if p.Name == "port" {
			return PortScanFinding(envPortFinding + p.Value).EnvPort()
		}
	}
	return 0
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
