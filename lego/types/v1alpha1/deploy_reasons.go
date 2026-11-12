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
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A deploy's in-flight and failure reasons are English sentences the operator
// and bex-api compose for every surface, REST and MCP included, as Render's
// API does. The dashboard words them in its own language instead (w4/566), so
// the sentences it knows live here as templates with a stable code: the
// writer formats one, and bex-api parses the stored text back into its code
// and parameters. A sentence no template matches stays English everywhere.
const (
	DeployReasonHTTPPortWaiting      = "http_port_waiting"
	DeployReasonHTTPPortDetecting    = "http_port_detecting"
	DeployReasonHTTPPortLimit        = "http_port_limit"
	DeployReasonHTTPPortDetected     = "http_port_detected"
	DeployReasonHTTPPortNotAnswering = "http_port_not_answering"
	DeployReasonHTTPPortNoneYet      = "http_port_none_yet"
	DeployReasonHTTPPortFallback     = "http_port_fallback"
	DeployReasonImageNotFound        = "image_not_found"
	DeployReasonImageCredential      = "image_credential_refused"
	DeployReasonImageAccess          = "image_access_refused"
	DeployReasonImageRead            = "image_read_refused"
	DeployReasonBuildTimedOut        = "build_timed_out"
	DeployReasonPreDeployTimedOut    = "pre_deploy_timed_out"
	DeployReasonHealthTimedOut       = "health_timed_out"

	// w4/591: the health-check stall, by its target and whether liveness has
	// restarted the container. The operator writes the waiting half;
	// TerminalStallText restates it as the failed half.
	DeployReasonProbeHTTPWaiting          = "probe_http_waiting"
	DeployReasonProbeHTTPWaitingRestarted = "probe_http_waiting_restarted"
	DeployReasonProbeHTTPFailed           = "probe_http_failed"
	DeployReasonProbeHTTPFailedRestarted  = "probe_http_failed_restarted"
	DeployReasonProbeTCPWaiting           = "probe_tcp_waiting"
	DeployReasonProbeTCPWaitingRestarted  = "probe_tcp_waiting_restarted"
	DeployReasonProbeTCPFailed            = "probe_tcp_failed"
	DeployReasonProbeTCPFailedRestarted   = "probe_tcp_failed_restarted"
	// The rollout deadline's verdict when nothing diagnosed the stall.
	DeployReasonRolloutDeadline = "rollout_deadline_exceeded"

	// w4/591: the operator's pre-deploy verdicts, and bex-api's closes of a
	// build that never started and of a deploy whose App it could not read.
	// build_never_started's {cause} is the operator's wait message, which
	// stays English.
	DeployReasonPreDeployWindowExceeded = "pre_deploy_window_exceeded"
	DeployReasonPreDeployOutOfMemory    = "pre_deploy_out_of_memory"
	DeployReasonPreDeployExited         = "pre_deploy_exited"
	DeployReasonPreDeployFailed         = "pre_deploy_failed"
	DeployReasonPreDeployCouldNotLaunch = "pre_deploy_could_not_launch"
	DeployReasonBuildNeverStarted       = "build_never_started"
	DeployReasonBuildAbandoned          = "build_abandoned"
	DeployReasonDeployAbandoned         = "deploy_abandoned"

	// w9/090: the API server refused one of the operator's own writes (an
	// admission policy, RBAC, a schema check). The customer reads this in
	// place of the raw refusal, which names internal policies and the tenant
	// namespace; {ref} finds the raw text in the operator log.
	DeployReasonPlatformError = "platform_error"

	// w4/591: a canceled deploy's cancelReason.
	DeployReasonCanceledSuspended   = "canceled_suspended"
	DeployReasonSupersededByNewer   = "superseded_by_newer_release"
	DeployReasonSupersededBy        = "superseded_by"
	DeployReasonCanceledByRestart   = "canceled_by_restart"
	DeployReasonCanceledAlongWith   = "canceled_along_with"
	DeployReasonCanceledAlongNewer  = "canceled_along_with_newer"
	DeployReasonRestartServiceGone  = "restart_service_gone"
	DeployReasonRestartWaitTimedOut = "restart_wait_timed_out"
)

// The health-check stall sentence's parts, shared by its eight templates.
const (
	probeStallHead      = "the container is running but its {kind} health check "
	probeStallHTTP      = ": GET {path} on port {port}."
	probeStallTCP       = ": a TCP connect to port {port}."
	probeStallRestarted = " The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow."
	probeStallPathTail  = " Check that the path returns a 2xx or 3xx status on the service's own port."
	probeStallTCPTail   = " No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on that port."
	// The operator appends port hints to the sentence on the passes that scan
	// for them (rolloutStallDiagnosis); {hint} carries them, in English, so
	// the sentence keeps its code with or without one.
	probeStallHint = "{hint}"
)

// deployReasonTemplates are the sentences, {name} marking a parameter.
var deployReasonTemplates = map[string]string{
	DeployReasonHTTPPortWaiting:      "Waiting for the container to start before detecting its HTTP port",
	DeployReasonHTTPPortDetecting:    "Detecting the container's HTTP port",
	DeployReasonHTTPPortLimit:        "The detected HTTP port exceeds the service's 75-port limit; set PORT explicitly",
	DeployReasonHTTPPortDetected:     "Detected HTTP on port {port}; serving it",
	DeployReasonHTTPPortNotAnswering: "Port {port} accepted a connection but has not answered HTTP yet; continuing to scan while the container starts",
	DeployReasonHTTPPortNoneYet:      "No port answers HTTP yet; continuing to scan while the container starts",
	DeployReasonHTTPPortFallback:     "No port answered HTTP; using fallback port {port}. Set PORT to the HTTP listener if it binds elsewhere",
	DeployReasonImageNotFound:        "the image {image} was not found in its registry: the repository or tag does not exist. Check the image URL and tag, or deploy a digest that exists",
	DeployReasonImageCredential:      "{host} refused this service's registry credential for the image {image} (HTTP {status}). Check the credential's username and token, or remove the credential from this service to pull a public image anonymously",
	DeployReasonImageAccess:          "the registry refused access to the image {image}. If the image is private, add a registry credential for {host} to this service",
	DeployReasonImageRead:            "the registry refused to read the image {image} (HTTP {status}). Check the image URL and tag",
	DeployReasonBuildTimedOut:        "the build did not finish within the build window; check the build logs",
	DeployReasonPreDeployTimedOut:    "the pre-deploy command did not finish within its window; check the pre-deploy logs",
	DeployReasonHealthTimedOut:       "the deploy did not become healthy within the health-gate window; check the service logs",

	DeployReasonProbeHTTPWaiting:          probeStallHead + ProbeStallWaiting + probeStallHTTP + probeStallPathTail + probeStallHint,
	DeployReasonProbeHTTPWaitingRestarted: probeStallHead + ProbeStallWaiting + probeStallHTTP + probeStallRestarted + probeStallPathTail + probeStallHint,
	DeployReasonProbeHTTPFailed:           probeStallHead + ProbeStallFailed + probeStallHTTP + probeStallPathTail + probeStallHint,
	DeployReasonProbeHTTPFailedRestarted:  probeStallHead + ProbeStallFailed + probeStallHTTP + probeStallRestarted + probeStallPathTail + probeStallHint,
	DeployReasonProbeTCPWaiting:           probeStallHead + ProbeStallWaiting + probeStallTCP + probeStallTCPTail + probeStallHint,
	DeployReasonProbeTCPWaitingRestarted:  probeStallHead + ProbeStallWaiting + probeStallTCP + probeStallRestarted + probeStallTCPTail + probeStallHint,
	DeployReasonProbeTCPFailed:            probeStallHead + ProbeStallFailed + probeStallTCP + probeStallTCPTail + probeStallHint,
	DeployReasonProbeTCPFailedRestarted:   probeStallHead + ProbeStallFailed + probeStallTCP + probeStallRestarted + probeStallTCPTail + probeStallHint,
	DeployReasonRolloutDeadline:           "rollout did not become healthy within the progress deadline",

	DeployReasonPreDeployWindowExceeded: "the pre-deploy command did not finish within its {minutes}-minute window; check the pre-deploy logs",
	DeployReasonPreDeployOutOfMemory:    "the pre-deploy command ran out of memory (exit code {code}); check the pre-deploy logs",
	DeployReasonPreDeployExited:         "the pre-deploy command exited with code {code}; check the pre-deploy logs",
	DeployReasonPreDeployFailed:         "the pre-deploy command failed; check the pre-deploy logs",
	DeployReasonPreDeployCouldNotLaunch: "the pre-deploy command could not start: the container runtime could not launch its program; the program never ran, so there are no pre-deploy logs",
	DeployReasonBuildNeverStarted:       "the build never started: {cause}",
	DeployReasonBuildAbandoned:          "the build did not finish within the build window, and this service's App record could not be read to explain why",
	DeployReasonDeployAbandoned:         "the deploy did not finish within its window, and this service's App record could not be read to explain why",
	DeployReasonPlatformError:           "bex could not apply this release because of a platform error, not a problem with your code or settings. Contact support with reference {ref}",

	DeployReasonCanceledSuspended:   "Canceled: the service was suspended",
	DeployReasonSupersededByNewer:   "Superseded by a newer release",
	DeployReasonSupersededBy:        "Superseded by {deploy}",
	DeployReasonCanceledByRestart:   "Canceled by restart {deploy}, which kept the running release: what this deploy was applying was not deployed",
	DeployReasonCanceledAlongWith:   "Canceled along with {deploy}",
	DeployReasonCanceledAlongNewer:  "Canceled along with a newer deploy",
	DeployReasonRestartServiceGone:  "the service no longer exists",
	DeployReasonRestartWaitTimedOut: "the deploy this restart waited for did not finish within {wait}",
}

// DeployReasonTemplates returns every code's sentence, {name} marking a
// parameter.
func DeployReasonTemplates() map[string]string {
	return maps.Clone(deployReasonTemplates)
}

// DeployReasonParam is one named value a reason sentence carries. It is no
// part of a CRD, so it gets no deepcopy.
// +kubebuilder:object:generate=false
type DeployReasonParam struct {
	Name, Value string
}

var deployReasonParam = regexp.MustCompile(`\{([a-z]+)\}`)

// FormatDeployReason is code's sentence with params filled in, in order of
// their names; a missing param reads as empty. Unknown codes read as "".
func FormatDeployReason(code string, params ...DeployReasonParam) string {
	values := make(map[string]string, len(params))
	for _, p := range params {
		values[p.Name] = p.Value
	}
	return deployReasonParam.ReplaceAllStringFunc(deployReasonTemplates[code], func(m string) string {
		return values[m[1:len(m)-1]]
	})
}

// ProbeStallReason is the in-flight health-check stall for a GET of path on
// port, or with tcp a TCP connect to port; kind is the stalling probe,
// "startup" or "readiness", and restarts the liveness restarts so far.
func ProbeStallReason(kind string, tcp bool, path, port string, restarts int32) string {
	code := DeployReasonProbeHTTPWaiting
	switch {
	case tcp && restarts > 0:
		code = DeployReasonProbeTCPWaitingRestarted
	case tcp:
		code = DeployReasonProbeTCPWaiting
	case restarts > 0:
		code = DeployReasonProbeHTTPWaitingRestarted
	}
	return FormatDeployReason(code, DeployReasonParam{Name: "kind", Value: kind}, DeployReasonParam{Name: "path", Value: path},
		DeployReasonParam{Name: "port", Value: port}, DeployReasonParam{Name: "restarts", Value: strconv.Itoa(int(restarts))})
}

type deployReasonMatcher struct {
	code string
	re   *regexp.Regexp
}

// deployReasonParamPattern is what a parameter may match: a hint is empty or
// a whole appended sentence, a path or a cause is free text, and every other
// value (an id, a port, a code, an image, a host) is one token, so it cannot
// swallow the text of a sentence it is part of.
func deployReasonParamPattern(name string) string {
	switch name {
	case "hint":
		return "(?P<hint>(?: .+)?)"
	case "path", "cause":
		return "(?P<" + name + ">.+?)"
	}
	return "(?P<" + name + `>\S+?)`
}

// deployReasonMatchers are the templates as anchored patterns, each parameter
// a named group. A parameter can stretch over text that another template
// fixes ("Superseded by {deploy}" over "Superseded by a newer release"), so
// the template with the most fixed text is tried first: it is the more
// specific one.
var deployReasonMatchers = func() []deployReasonMatcher {
	codes := slices.Collect(maps.Keys(deployReasonTemplates))
	fixed := func(code string) int {
		return len(deployReasonParam.ReplaceAllString(deployReasonTemplates[code], ""))
	}
	slices.SortFunc(codes, func(a, b string) int {
		return cmp.Or(fixed(b)-fixed(a), strings.Compare(a, b))
	})
	out := make([]deployReasonMatcher, 0, len(codes))
	for _, code := range codes {
		tmpl := deployReasonTemplates[code]
		var pattern strings.Builder
		pattern.WriteString("^")
		last := 0
		for _, m := range deployReasonParam.FindAllStringSubmatchIndex(tmpl, -1) {
			pattern.WriteString(regexp.QuoteMeta(tmpl[last:m[0]]))
			pattern.WriteString(deployReasonParamPattern(tmpl[m[2]:m[3]]))
			last = m[1]
		}
		pattern.WriteString(regexp.QuoteMeta(tmpl[last:]) + "$")
		out = append(out, deployReasonMatcher{code, regexp.MustCompile(pattern.String())})
	}
	return out
}()

// ParseDeployReason names the template a stored sentence came from, with its
// parameters. ok is false for a sentence no template matches.
func ParseDeployReason(text string) (code string, params []DeployReasonParam, ok bool) {
	for _, m := range deployReasonMatchers {
		match := m.re.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		for i, name := range m.re.SubexpNames() {
			if name != "" {
				params = append(params, DeployReasonParam{Name: name, Value: match[i]})
			}
		}
		return m.code, params, true
	}
	return "", nil, false
}
