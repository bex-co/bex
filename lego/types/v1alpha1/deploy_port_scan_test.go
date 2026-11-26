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

import "testing"

// w8/385: which in-flight stall reasons start, keep or end the port scan the
// deploy log narrates, read from the sentences the operator writes.
func TestPortScanOfTheOperatorsStalls(t *testing.T) {
	port := DeployReasonParam{Name: "port", Value: "10000"}
	ports := DeployReasonParam{Name: "ports", Value: "8000, 9000"}
	hint := func(codes ...string) string {
		out := ""
		for _, code := range codes {
			out += FormatDeployReason(code, port, DeployReasonParam{Name: "previous", Value: "3000"},
				DeployReasonParam{Name: "ports", Value: "8000, 9000"})
		}
		return out
	}
	tcp := ProbeStallReason("startup", true, "", "10000", 0)
	get := ProbeStallReason("readiness", false, "/healthz", "10000", 2)
	openPort := OpenPortProbeStallReason("startup", 0)
	for _, c := range []struct {
		name, stall string
		want        PortScan
	}{
		{"no stall", "", PortScanOver},
		{"text no template matches", "Back-off restarting failed container", PortScanOver},
		{"image web: nothing answers yet", FormatDeployReason(DeployReasonHTTPPortNoneYet), PortScanNoOpenPort},
		{"image web: waiting to start", FormatDeployReason(DeployReasonHTTPPortWaiting), PortScanSilent},
		{"image web: between two scans", FormatDeployReason(DeployReasonHTTPPortDetecting), PortScanSilent},
		{"image web: fallback port", FormatDeployReason(DeployReasonHTTPPortFallback, port), PortScanSilent},
		{"image web: a port accepted, beside one that answers", FormatDeployReason(DeployReasonHTTPPortNotAnswering, port), PortScanOver},
		{"image web: only localhost listens", FormatDeployReason(DeployReasonHTTPPortLocalhostOnly, ports), PortScanLocalhostOnly},
		{"image web: open ports answer no HTTP", FormatDeployReason(DeployReasonHTTPPortNoneHTTP, ports), PortScanNoHTTPPort},
		{"image web: detected", FormatDeployReason(DeployReasonHTTPPortDetected, port), PortScanOver},
		{"native: TCP check before the first scan", tcp, PortScanOver},
		{"native: no open port", tcp + hint(DeployReasonHintNoOpenPort, DeployReasonHintBackgroundWorker), PortScanNoOpenPort},
		{"native: no open port, listened before", tcp + hint(DeployReasonHintNoOpenPortBefore, DeployReasonHintBackgroundWorker), PortScanNoOpenPort},
		{"private: no open service port", tcp + hint(DeployReasonHintNoOpenServicePort), PortScanNoOpenPort},
		{"native: only localhost listens", tcp + hint(DeployReasonHintNoOpenPortLocalhost), PortScanLocalhostOnly},
		{"native: open ports answer no HTTP", tcp + hint(DeployReasonHintNoOpenHTTPPort), PortScanNoHTTPPort},
		{"any-open-port check, only localhost listens", openPort + hint(DeployReasonHintNoOpenPortLocalhost), PortScanLocalhostOnly},
		{"path check, open ports answer no HTTP", get + hint(DeployReasonHintNoOpenHTTPPort), PortScanNoHTTPPort},
		{"native: listening elsewhere", tcp + hint(DeployReasonHintListening), PortScanOver},
		{"native: listening port adopted", tcp + hint(DeployReasonHintListeningAdopted), PortScanOver},
		{"native: several listeners", tcp + hint(DeployReasonHintListeningPorts), PortScanOver},
		{"private: listening primary", tcp + hint(DeployReasonHintListeningPrimary), PortScanOver},
		{"path check failing with its port open", get, PortScanOver},
		{"image TCP check: primary closed", tcp + hint(DeployReasonHintImageDefaultPort, DeployReasonHintBackgroundWorker), PortScanSilent},
		{"image TCP check: declared primary closed", tcp + hint(DeployReasonHintImageDeclaredPorts, DeployReasonHintBackgroundWorker,
			DeployReasonHintNoHTTPDiscovered), PortScanSilent},
		{"configured port closed", tcp + hint(DeployReasonHintConfiguredPortOnly, DeployReasonHintBackgroundWorker), PortScanSilent},
		{"configured port closed, the image declares others", tcp + hint(DeployReasonHintConfiguredPortDeclared, DeployReasonHintBackgroundWorker), PortScanSilent},
		// w8/416: Render scans only the port PORT names once it is set.
		{"PORT closed", tcp + hint(DeployReasonHintEnvPortClosed), PortScanEnvPort},
		{"private image: PORT closed", tcp + hint(DeployReasonHintEnvPortOnly, DeployReasonHintBackgroundWorker), PortScanSilent},
		{"private image: PORT closed, the image declares others", tcp + hint(DeployReasonHintEnvPortDeclared, DeployReasonHintBackgroundWorker), PortScanSilent},
		{"PORT closed, restarted", ProbeStallReason("startup", true, "", "10000", 2) + hint(DeployReasonHintEnvPortClosed), PortScanEnvPort},
		{"PORT closed with no port to name", tcp + FormatDeployReason(DeployReasonHintEnvPortClosed, DeployReasonParam{Name: "port", Value: "0"}), PortScanOver},
		{"any-open-port check, no hint", openPort, PortScanNoOpenPort},
		{"any-open-port check, image hints (w8/384's busybox)", openPort + hint(DeployReasonHintImageDefaultPort,
			DeployReasonHintBackgroundWorker, DeployReasonHintNoHTTPDiscovered), PortScanNoOpenPort},
		{"any-open-port check restarted", OpenPortProbeStallReason("readiness", 3), PortScanNoOpenPort},
		{"failed half is not in flight", TerminalStallText(openPort), PortScanOver},
	} {
		if got := PortScanOf(c.stall); got != c.want {
			t.Errorf("%s: PortScanOf(%q) = %d, want %d", c.name, c.stall, got, c.want)
		}
	}
}

// w8/388: the scan's running cases are each a scan, and the row records the
// ones Render names apart from the plain one; w8/416 adds the port PORT names.
func TestPortScanFindings(t *testing.T) {
	port := DeployReasonParam{Name: "port", Value: "8080"}
	ports := DeployReasonParam{Name: "ports", Value: "8000, 9000"}
	tcp := ProbeStallReason("startup", true, "", "8080", 0)
	for _, c := range []struct {
		name, stall string
		scanning    bool
		found       PortScanFinding
	}{
		{"waiting to start", FormatDeployReason(DeployReasonHTTPPortWaiting), false, ""},
		{"no open port", tcp + FormatDeployReason(DeployReasonHintNoOpenPort, port), true, ""},
		{"only localhost listens", tcp + FormatDeployReason(DeployReasonHintNoOpenPortLocalhost, port, ports), true, PortScanFoundLocalhost},
		{"open ports answer no HTTP", tcp + FormatDeployReason(DeployReasonHintNoOpenHTTPPort, port, ports), true, PortScanFoundNoHTTP},
		{"PORT closed", tcp + FormatDeployReason(DeployReasonHintEnvPortClosed, port), true, "env_port:8080"},
		{"private image: PORT closed", tcp + FormatDeployReason(DeployReasonHintEnvPortDeclared, port, ports), false, ""},
		{"no stall", "", false, ""},
	} {
		scan, found := ReadPortScan(c.stall)
		if scan != PortScanOf(c.stall) || scan.Scanning() != c.scanning || found != c.found {
			t.Errorf("%s: ReadPortScan(%q) = (%d, %q), want a scan %v finding %q", c.name, c.stall, scan, found, c.scanning, c.found)
		}
	}
}

// w8/416: a scan for PORT records its port, and only a port reads back.
func TestPortScanFindingNamesThePortPORTSets(t *testing.T) {
	if got := PortScanFoundEnvPort(8080); got != "env_port:8080" || got.EnvPort() != 8080 {
		t.Fatalf("PortScanFoundEnvPort(8080) = %q, reading %d", got, got.EnvPort())
	}
	for _, f := range []PortScanFinding{"", PortScanFoundLocalhost, PortScanFoundNoHTTP, "env_port:", "env_port:x", "env_port:0", "env_port:65536", "port:8080"} {
		if got := f.EnvPort(); got != 0 {
			t.Errorf("%q.EnvPort() = %d, want 0", f, got)
		}
	}
}

// w1/m659: the reserved listener a stall or failure reason names reads back as
// its port, on a stall still scanning for an open port, and only a reserved
// port in that hint does.
func TestReservedPortListenerReadsTheReasonsHint(t *testing.T) {
	hint := func(reserved string) string {
		return FormatDeployReason(DeployReasonHintReservedPort, DeployReasonParam{Name: "port", Value: "10000"}, DeployReasonParam{Name: "reserved", Value: reserved})
	}
	open := OpenPortProbeStallReason("startup", 0)
	http := ProbeStallReason("readiness", false, "/healthz", "10000", 0)
	for _, c := range []struct {
		name, reason string
		want         int32
	}{
		{"an open-port stall", open + hint("18012"), 18012},
		{"a health-check stall", http + hint("19099"), 19099},
		{"after another hint", http + FormatDeployReason(DeployReasonHintNoHTTPDiscovered) + hint("18013"), 18013},
		{"a port that is not reserved", open + hint("8080"), 0},
		{"no reserved hint", open + FormatDeployReason(DeployReasonHintNoOpenPort, DeployReasonParam{Name: "port", Value: "10000"}), 0},
		{"the hint alone, with no reason", hint("18012"), 0},
		{"no reason", "", 0},
	} {
		if got := ReservedPortListener(c.reason); got != c.want {
			t.Errorf("%s: ReservedPortListener(%q) = %d, want %d", c.name, c.reason, got, c.want)
		}
	}
	if scan := PortScanOf(http + hint("18012")); scan != PortScanNoOpenPort {
		t.Fatalf("a health-check stall naming a reserved listener reads as port scan %d, want no open port", scan)
	}
}
