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
	// w4/630: the service container was OOM-killed while starting; {memory}
	// is its instance type's limit, as Render's "Out of memory" names it.
	DeployReasonStartupOutOfMemory = "startup_out_of_memory"

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
	// w5/m356: a check that is neither a GET nor a TCP connect.
	DeployReasonProbeOtherWaiting          = "probe_other_waiting"
	DeployReasonProbeOtherWaitingRestarted = "probe_other_waiting_restarted"
	DeployReasonProbeOtherFailed           = "probe_other_failed"
	DeployReasonProbeOtherFailedRestarted  = "probe_other_failed_restarted"
	// The rollout deadline's verdict when nothing diagnosed the stall.
	DeployReasonRolloutDeadline = "rollout_deadline_exceeded"

	// w4/591: the operator's pre-deploy verdicts, and bex-api's closes of a
	// build that never started and of a deploy whose App it could not read.
	// build_never_started's {cause} is the operator's wait message: its
	// sentences are parts (below).
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
	// w4/652: a Rollback or another deploy that waited, as a Restart does.
	DeployReasonDeployWaitTimedOut = "deploy_wait_timed_out"
)

// w5/m356: the sentences that ride another reason's tail, a probe stall's
// {hint} (rolloutStallDiagnosis' port hints) and a never-started build's
// {cause} (the operator's wait). Each parses only off that tail
// (ParseDeployReasonParts), never as a reason of its own, so a stored sentence
// reads as the same code it always did. A hint starts with the space that
// joins it to the sentence before.
const (
	DeployReasonHintImageDefaultPort       = "hint_image_default_port"
	DeployReasonHintImageDeclaredPorts     = "hint_image_declared_ports"
	DeployReasonHintEnvPortOnly            = "hint_env_port_only"
	DeployReasonHintConfiguredPortOnly     = "hint_configured_port_only"
	DeployReasonHintEnvPortDeclared        = "hint_env_port_declared"
	DeployReasonHintConfiguredPortDeclared = "hint_configured_port_declared"
	DeployReasonHintBackgroundWorker       = "hint_background_worker"
	DeployReasonHintNoHTTPDiscovered       = "hint_no_http_discovered"
	DeployReasonHintListening              = "hint_listening"
	DeployReasonHintListeningAdopted       = "hint_listening_adopted"
	DeployReasonHintListeningPorts         = "hint_listening_ports"
	DeployReasonHintListeningPrimary       = "hint_listening_primary"
	DeployReasonHintNoOpenPort             = "hint_no_open_port"
	DeployReasonHintNoOpenPortBefore       = "hint_no_open_port_before"
	DeployReasonHintNoOpenServicePort      = "hint_no_open_service_port"

	DeployReasonQueuedBuildLimit     = "queued_build_limit"
	DeployReasonQueuedClusterFull    = "queued_cluster_full"
	DeployReasonQueuedSlotTaken      = "queued_slot_taken"
	DeployReasonQueuedDatabase       = "queued_database"
	DeployReasonQueuedDatastore      = "queued_datastore"
	DeployReasonQueuedBuildCred      = "queued_build_credential"
	DeployReasonQueuedPullCred       = "queued_pull_credential"
	DeployReasonQueuedCapacity       = "queued_capacity"
	DeployReasonQueuedCapacityPod    = "queued_capacity_pod"
	DeployReasonQueuedCapacityNoNode = "queued_capacity_no_node"
	DeployReasonQueuedCapacityShort  = "queued_capacity_short"
)

// The health-check stall sentence's parts, shared by its templates.
const (
	probeStallHead      = "the container is running but its {kind} health check "
	probeStallHTTP      = ": GET {path} on port {port}."
	probeStallTCP       = ": a TCP connect to port {port}."
	probeStallOther     = ": the configured health check."
	probeStallRestarted = " The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow."
	probeStallPathTail  = " Check that the path returns a 2xx or 3xx status on the service's own port."
	probeStallTCPTail   = " No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on that port."
	// The operator appends port hints to the sentence (rolloutStallDiagnosis);
	// {hint} carries them, so the sentence keeps its code with or without
	// one, and ParseDeployReasonParts reads their sentences.
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
	DeployReasonStartupOutOfMemory:   "the instance ran out of memory during startup ({memory} limit on its instance type; exit code {code}) — reduce memory use or choose a larger instance type",

	DeployReasonProbeHTTPWaiting:          probeStallHead + ProbeStallWaiting + probeStallHTTP + probeStallPathTail + probeStallHint,
	DeployReasonProbeHTTPWaitingRestarted: probeStallHead + ProbeStallWaiting + probeStallHTTP + probeStallRestarted + probeStallPathTail + probeStallHint,
	DeployReasonProbeHTTPFailed:           probeStallHead + ProbeStallFailed + probeStallHTTP + probeStallPathTail + probeStallHint,
	DeployReasonProbeHTTPFailedRestarted:  probeStallHead + ProbeStallFailed + probeStallHTTP + probeStallRestarted + probeStallPathTail + probeStallHint,
	DeployReasonProbeTCPWaiting:           probeStallHead + ProbeStallWaiting + probeStallTCP + probeStallTCPTail + probeStallHint,
	DeployReasonProbeTCPWaitingRestarted:  probeStallHead + ProbeStallWaiting + probeStallTCP + probeStallRestarted + probeStallTCPTail + probeStallHint,
	DeployReasonProbeTCPFailed:            probeStallHead + ProbeStallFailed + probeStallTCP + probeStallTCPTail + probeStallHint,
	DeployReasonProbeTCPFailedRestarted:   probeStallHead + ProbeStallFailed + probeStallTCP + probeStallRestarted + probeStallTCPTail + probeStallHint,
	// Any other check keeps the path advice its sentence has always ended on.
	DeployReasonProbeOtherWaiting:          probeStallHead + ProbeStallWaiting + probeStallOther + probeStallPathTail + probeStallHint,
	DeployReasonProbeOtherWaitingRestarted: probeStallHead + ProbeStallWaiting + probeStallOther + probeStallRestarted + probeStallPathTail + probeStallHint,
	DeployReasonProbeOtherFailed:           probeStallHead + ProbeStallFailed + probeStallOther + probeStallPathTail + probeStallHint,
	DeployReasonProbeOtherFailedRestarted:  probeStallHead + ProbeStallFailed + probeStallOther + probeStallRestarted + probeStallPathTail + probeStallHint,
	DeployReasonRolloutDeadline:            "rollout did not become healthy within the progress deadline",

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
	DeployReasonDeployWaitTimedOut:  "the deploy this one waited for did not finish within {wait}",
}

// The parts' shared wording.
const (
	hintNothingAccepted = " Nothing accepted a TCP connection on port {port}, "
	hintSetByEnv        = "set by the service's PORT environment variable; "
	hintSetByConfigured = "set by the service's configured port; "
	hintNoOtherPort     = "the image declares no other TCP port. Bind the server to 0.0.0.0 on port {port}, or change the service's port to the one it listens on."
	hintDeclaresPorts   = "the image declares TCP ports ({ports})."
	hintNoOpenPort      = " No open port was detected on the instance: the server must bind host 0.0.0.0 on "
	queuedCapacity      = "waiting for build capacity: "
	queuedNoBuildNode   = "no build node has room for it yet"
)

// deployReasonPartTemplates are the tail sentences, {name} marking a
// parameter.
var deployReasonPartTemplates = map[string]string{
	DeployReasonHintImageDefaultPort:       hintNothingAccepted + "the default because the image declares no TCP ports. The server must bind host 0.0.0.0 on the port in PORT ({port}).",
	DeployReasonHintImageDeclaredPorts:     hintNothingAccepted + "the primary port selected from the image's declared TCP ports ({ports}). Bind the server to 0.0.0.0 on one of those ports, or set the service's port to the one it listens on.",
	DeployReasonHintEnvPortOnly:            hintNothingAccepted + hintSetByEnv + hintNoOtherPort,
	DeployReasonHintConfiguredPortOnly:     hintNothingAccepted + hintSetByConfigured + hintNoOtherPort,
	DeployReasonHintEnvPortDeclared:        hintNothingAccepted + hintSetByEnv + hintDeclaresPorts + " Change the PORT environment variable to the port it listens on, or remove it to use the image's port.",
	DeployReasonHintConfiguredPortDeclared: hintNothingAccepted + hintSetByConfigured + hintDeclaresPorts + " Change the service's port in Settings to the one it listens on, or remove it to use the image's port.",
	DeployReasonHintBackgroundWorker:       " If the service does not need to receive traffic on any port, create a background worker instead.",
	DeployReasonHintNoHTTPDiscovered:       " No port answered HTTP during startup discovery.",
	DeployReasonHintListening:              " The process is listening on port {port}.",
	DeployReasonHintListeningAdopted:       " The process is listening on port {port}, which the service now uses.",
	DeployReasonHintListeningPorts:         " The process is listening on ports {ports}, not on port {port}; set the service's port (or its PORT variable) to the one that serves HTTP.",
	DeployReasonHintListeningPrimary:       " The process is listening on port {port}, which the service now uses as its primary port.",
	DeployReasonHintNoOpenPort:             hintNoOpenPort + "the port in PORT ({port}).",
	DeployReasonHintNoOpenPortBefore:       hintNoOpenPort + "the port in PORT ({port}); it listened on port {previous} before.",
	DeployReasonHintNoOpenServicePort:      hintNoOpenPort + "the service's port ({port}).",

	DeployReasonQueuedBuildLimit:  "{noun} has {active}/{limit} concurrent builds active; waiting for a slot",
	DeployReasonQueuedClusterFull: "the cluster's build capacity is full; waiting for a slot",
	DeployReasonQueuedSlotTaken:   "a concurrent dispatch took the last build slot; waiting for a slot",
	DeployReasonQueuedDatabase:    "Waiting for linked database {database} to become available before building",
	DeployReasonQueuedDatastore:   "Waiting for a linked datastore's connection details before building",
	DeployReasonQueuedBuildCred:   "Waiting for the registry to accept this app's build credential",
	DeployReasonQueuedPullCred:    "Waiting for the registry to accept this app's pull credential",
	// The operator writes the capacity wait with the build's own {detail};
	// the details bex words parse as their own, more specific templates, and
	// any other (an older row's raw scheduler verdict) stays English.
	DeployReasonQueuedCapacity:       queuedCapacity + "{detail}",
	DeployReasonQueuedCapacityPod:    queuedCapacity + "build job dispatched; waiting for its build pod to be created",
	DeployReasonQueuedCapacityNoNode: queuedCapacity + queuedNoBuildNode,
	DeployReasonQueuedCapacityShort:  queuedCapacity + queuedNoBuildNode + " (not enough {resources} free)",
}

// DeployReasonTemplates returns every code's sentence, parts included, {name}
// marking a parameter.
func DeployReasonTemplates() map[string]string {
	all := maps.Clone(deployReasonTemplates)
	maps.Copy(all, deployReasonPartTemplates)
	return all
}

// DeployReasonPartTemplates returns the tail sentences' templates.
func DeployReasonPartTemplates() map[string]string {
	return maps.Clone(deployReasonPartTemplates)
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
	tmpl, ok := deployReasonTemplates[code]
	if !ok {
		tmpl = deployReasonPartTemplates[code]
	}
	return deployReasonParam.ReplaceAllStringFunc(tmpl, func(m string) string {
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

// OtherProbeStallReason is ProbeStallReason for a check that is neither a GET
// nor a TCP connect.
func OtherProbeStallReason(kind string, restarts int32) string {
	code := DeployReasonProbeOtherWaiting
	if restarts > 0 {
		code = DeployReasonProbeOtherWaitingRestarted
	}
	return FormatDeployReason(code, DeployReasonParam{Name: "kind", Value: kind},
		DeployReasonParam{Name: "restarts", Value: strconv.Itoa(int(restarts))})
}

type deployReasonMatcher struct {
	code string
	re   *regexp.Regexp
}

// deployReasonParamPattern is what a parameter may match: a hint is empty or
// a whole appended sentence, a path or a cause is free text, a detail is the
// rest of its part, a list of ports or resources is comma-separated, and every
// other value (an id, a port, a code, an image, a host) is one token, so it
// cannot swallow the text of a sentence it is part of.
func deployReasonParamPattern(name string) string {
	switch name {
	case "hint":
		return "(?P<hint>(?: .+)?)"
	case "path", "cause":
		return "(?P<" + name + ">.+?)"
	case "detail":
		return "(?P<detail>.+)"
	case "ports":
		return `(?P<ports>\d+(?:, \d+)*)`
	case "resources":
		return `(?P<resources>[a-z-]+(?:, [a-z-]+)*)`
	}
	return "(?P<" + name + `>\S+?)`
}

// deployReasonMatchersFor are templates as anchored patterns, each parameter
// a named group. A parameter can stretch over text that another template
// fixes ("Superseded by {deploy}" over "Superseded by a newer release"), so
// the template with the most fixed text is tried first: it is the more
// specific one. A whole reason is anchored at both ends. A part is anchored
// at its start and ends where a word does, its text in the first group.
func deployReasonMatchersFor(templates map[string]string, part bool) []deployReasonMatcher {
	codes := slices.Collect(maps.Keys(templates))
	fixed := func(code string) int {
		return len(deployReasonParam.ReplaceAllString(templates[code], ""))
	}
	slices.SortFunc(codes, func(a, b string) int {
		return cmp.Or(fixed(b)-fixed(a), strings.Compare(a, b))
	})
	out := make([]deployReasonMatcher, 0, len(codes))
	for _, code := range codes {
		tmpl := templates[code]
		var pattern strings.Builder
		last := 0
		for _, m := range deployReasonParam.FindAllStringSubmatchIndex(tmpl, -1) {
			pattern.WriteString(regexp.QuoteMeta(tmpl[last:m[0]]))
			pattern.WriteString(deployReasonParamPattern(tmpl[m[2]:m[3]]))
			last = m[1]
		}
		pattern.WriteString(regexp.QuoteMeta(tmpl[last:]))
		re := "^" + pattern.String() + "$"
		if part {
			re = "^(" + pattern.String() + ")(?: |$)"
		}
		out = append(out, deployReasonMatcher{code, regexp.MustCompile(re)})
	}
	return out
}

var (
	deployReasonMatchers     = deployReasonMatchersFor(deployReasonTemplates, false)
	deployReasonPartMatchers = deployReasonMatchersFor(deployReasonPartTemplates, true)
)

// params are the named groups of match, each once: a parameter a template
// repeats must read the same everywhere, or the text is not that template's.
func (m deployReasonMatcher) params(match []string) ([]DeployReasonParam, bool) {
	var params []DeployReasonParam
	seen := map[string]string{}
	for i, name := range m.re.SubexpNames() {
		if name == "" {
			continue
		}
		if value, ok := seen[name]; ok {
			if value != match[i] {
				return nil, false
			}
			continue
		}
		seen[name] = match[i]
		params = append(params, DeployReasonParam{Name: name, Value: match[i]})
	}
	return params, true
}

// ParseDeployReason names the template a stored sentence came from, with its
// parameters. ok is false for a sentence no template matches.
func ParseDeployReason(text string) (code string, params []DeployReasonParam, ok bool) {
	for _, m := range deployReasonMatchers {
		match := m.re.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		if params, ok := m.params(match); ok {
			return m.code, params, true
		}
	}
	return "", nil, false
}

// DeployReasonPart is one templated sentence of a reason's tail. It is no
// part of a CRD, so it gets no deepcopy.
// +kubebuilder:object:generate=false
type DeployReasonPart struct {
	Code   string
	Params []DeployReasonParam
	// Text is the sentence as the tail has it.
	Text string
}

// ParseDeployReasonParts reads tail, a probe stall's {hint} or a never-started
// build's {cause}, as the part sentences it is made of, in order. From the
// first text no part template matches, the tail is the remainder, unchanged.
func ParseDeployReasonParts(tail string) (parts []DeployReasonPart, remainder string) {
	for tail != "" {
		part, ok := parseDeployReasonPart(tail)
		if !ok {
			break
		}
		parts = append(parts, part)
		tail = tail[len(part.Text):]
	}
	return parts, tail
}

// parseDeployReasonPart is the part tail starts with; ok is false when no part
// template matches.
func parseDeployReasonPart(tail string) (part DeployReasonPart, ok bool) {
	for _, m := range deployReasonPartMatchers {
		if match := m.re.FindStringSubmatch(tail); match != nil {
			if params, ok := m.params(match); ok {
				return DeployReasonPart{Code: m.code, Params: params, Text: match[1]}, true
			}
		}
	}
	return DeployReasonPart{}, false
}
