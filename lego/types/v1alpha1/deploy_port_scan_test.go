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
		{"image web: a port accepted", FormatDeployReason(DeployReasonHTTPPortNotAnswering, port), PortScanOver},
		{"image web: detected", FormatDeployReason(DeployReasonHTTPPortDetected, port), PortScanOver},
		{"native: TCP check before the first scan", tcp, PortScanOver},
		{"native: no open port", tcp + hint(DeployReasonHintNoOpenPort, DeployReasonHintBackgroundWorker), PortScanNoOpenPort},
		{"native: no open port, listened before", tcp + hint(DeployReasonHintNoOpenPortBefore, DeployReasonHintBackgroundWorker), PortScanNoOpenPort},
		{"private: no open service port", tcp + hint(DeployReasonHintNoOpenServicePort), PortScanNoOpenPort},
		{"native: listening elsewhere", tcp + hint(DeployReasonHintListening), PortScanOver},
		{"native: listening port adopted", tcp + hint(DeployReasonHintListeningAdopted), PortScanOver},
		{"native: several listeners", tcp + hint(DeployReasonHintListeningPorts), PortScanOver},
		{"private: listening primary", tcp + hint(DeployReasonHintListeningPrimary), PortScanOver},
		{"path check failing with its port open", get, PortScanOver},
		{"image TCP check: primary closed", tcp + hint(DeployReasonHintImageDefaultPort, DeployReasonHintBackgroundWorker), PortScanSilent},
		{"image TCP check: declared primary closed", tcp + hint(DeployReasonHintImageDeclaredPorts, DeployReasonHintBackgroundWorker,
			DeployReasonHintNoHTTPDiscovered), PortScanSilent},
		{"configured port closed", tcp + hint(DeployReasonHintConfiguredPortOnly, DeployReasonHintBackgroundWorker), PortScanSilent},
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
