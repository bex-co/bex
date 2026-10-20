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

package apps

import "testing"

// w5/m143: inputs Render's own Deploy-to-Render templates use but its
// published schema forbids — unquoted booleans as env values, capitalized
// plan names, a bare `envVars:` — validate, as on Render.
func TestBlueprintAcceptsRenderTemplateSpellings(t *testing.T) {
	stack := compileNumericEnvStack(t, `services:
  - type: web
    name: web
    runtime: go
    repo: https://github.com/bex-co/bex
    plan: Starter
    buildCommand: go build -o app .
    startCommand: ./app
    envVars:
      - key: DISABLE_SIGNUP
        value: true
      - key: TELEMETRY
        value: false
      - key: QUOTED
        value: "true"
  - type: worker
    name: worker
    runtime: go
    repo: https://github.com/bex-co/bex
    plan: free
    buildCommand: go build -o app .
    startCommand: ./app
    envVars:
`)
	if len(stack.services) != 2 {
		t.Fatalf("services = %d", len(stack.services))
	}
	web := stack.services[0]
	got := map[string]string{}
	for _, e := range web.req.Env {
		got[e.Name] = e.Value
	}
	for k, v := range map[string]string{"DISABLE_SIGNUP": "true", "TELEMETRY": "false", "QUOTED": "true"} {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if web.req.Plan != "starter" {
		t.Errorf("plan = %q, want starter", web.req.Plan)
	}
	if len(stack.services[1].req.Env) != 0 {
		t.Errorf("bare envVars = %+v, want none", stack.services[1].req.Env)
	}
}

// A boolean outside an env value keeps its type: a setting typed boolean
// still validates as one, and the schema still refuses a boolean where it
// wants a string.
func TestBlueprintBooleanCoercionIsScopedToEnvValues(t *testing.T) {
	_, _, problems := CompileBlueprintIR(`services:
  - type: web
    name: web
    runtime: go
    repo: https://github.com/bex-co/bex
    plan: free
    buildCommand: go build -o app .
    startCommand: true
`)
	if len(problems) == 0 {
		t.Fatal("a boolean startCommand should stay a schema refusal")
	}
}

// Render's templates annotate env vars with deploy-time prompt metadata its
// published schema does not list (displayName, description, sensitive,
// required); Render accepts and ignores it, and so does bex. The same key
// elsewhere is still a schema refusal.
func TestBlueprintIgnoresRenderTemplateEnvMetadata(t *testing.T) {
	stack := compileNumericEnvStack(t, numericEnvService(`      - key: OPENAI_API_KEY
        sync: false
        displayName: OpenAI API key
        description: Used for completions
        sensitive: true
        required: true
      - key: MODE
        value: prod
        description: Run mode
`))
	got := map[string]string{}
	for _, e := range stack.services[0].req.Env {
		got[e.Name] = e.Value
	}
	if got["MODE"] != "prod" {
		t.Errorf("env = %+v, want MODE=prod", got)
	}
	if _, _, problems := CompileBlueprintIR(`services:
  - type: web
    name: web
    runtime: go
    repo: https://github.com/bex-co/bex
    plan: free
    description: not a service field
    buildCommand: go build -o app .
    startCommand: ./app
`); len(problems) == 0 {
		t.Fatal("description outside an env var should stay a schema refusal")
	}
}
