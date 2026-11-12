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
	"maps"
	"regexp"
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
}

// DeployReasonTemplates returns every code's sentence, {name} marking a
// parameter.
func DeployReasonTemplates() map[string]string {
	return maps.Clone(deployReasonTemplates)
}

// DeployReasonParam is one named value a reason sentence carries.
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

type deployReasonMatcher struct {
	code string
	re   *regexp.Regexp
}

// deployReasonMatchers are the templates as anchored patterns, each parameter
// a named group. Anchored at both ends, a sentence matches one template at most.
var deployReasonMatchers = func() []deployReasonMatcher {
	out := make([]deployReasonMatcher, 0, len(deployReasonTemplates))
	for code, tmpl := range deployReasonTemplates {
		var pattern strings.Builder
		pattern.WriteString("^")
		last := 0
		for _, m := range deployReasonParam.FindAllStringSubmatchIndex(tmpl, -1) {
			pattern.WriteString(regexp.QuoteMeta(tmpl[last:m[0]]))
			pattern.WriteString("(?P<" + tmpl[m[2]:m[3]] + ">.+?)")
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
