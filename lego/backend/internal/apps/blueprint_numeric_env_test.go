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

import (
	"strings"
	"testing"
)

func numericEnvService(envVars string) string {
	return `services:
  - type: web
    name: web
    runtime: go
    repo: https://github.com/bex-co/bex
    plan: free
    buildCommand: go build -o app .
    startCommand: ./app
    envVars:
` + envVars
}

func compileNumericEnvStack(t *testing.T, manifest string) parsedStack {
	t.Helper()
	compiled, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) != 0 {
		t.Fatalf("compiler refused: %+v", problems)
	}
	stack, err := parseCompiledStack(blueprintParseOverrides{}, compiled, ir)
	if err != nil {
		t.Fatalf("typed stack refused a schema-valid manifest: %v", err)
	}
	return stack
}

// The pinned schema allows envVarFromKeyValue.value to be a number (w8/m53);
// the process receives its JSON text, and quoted strings keep their exact text.
func TestBlueprintEnvValueAcceptsNumbers(t *testing.T) {
	stack := compileNumericEnvStack(t, numericEnvService(`      - key: INT
        value: 47
      - key: FLOAT
        value: 47.5
      - key: EXP
        value: 1e3
      - key: BIG
        value: 9007199254740993
      - key: ZERO
        value: 0
      - key: NEG
        value: -3
      - key: LEADING_ZERO
        value: "007"
      - key: SPACED
        value: " 47 "
      - key: SEEDED
        value: 12
        sync: false
`))
	if len(stack.services) != 1 {
		t.Fatalf("services = %d", len(stack.services))
	}
	svc := stack.services[0]
	got := map[string]string{}
	for _, e := range svc.req.Env {
		got[e.Name] = e.Value
	}
	want := map[string]string{
		"INT": "47", "FLOAT": "47.5", "EXP": "1000", "BIG": "9007199254740993",
		"ZERO": "0", "NEG": "-3", "LEADING_ZERO": "007", "SPACED": " 47 ",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if svc.seedLiterals["SEEDED"] != "12" {
		t.Errorf("sync:false numeric seed = %q, want 12", svc.seedLiterals["SEEDED"])
	}
	if _, inSpec := got["SEEDED"]; inSpec {
		t.Error("sync:false var leaked into spec.Env")
	}
}

func TestBlueprintEnvGroupAndNestedServiceAcceptNumbers(t *testing.T) {
	stack := compileNumericEnvStack(t, `envVarGroups:
  - name: shared
    envVars:
      - key: ANSWER
        value: 47
projects:
  - name: proj
    environments:
      - name: production
        services:
          - type: web
            name: web
            runtime: go
            repo: https://github.com/bex-co/bex
            plan: free
            buildCommand: go build -o app .
            startCommand: ./app
            envVars:
              - key: ANSWER
                value: 42
`)
	if len(stack.envGroups) != 1 || stack.envGroups[0].literals["ANSWER"] != "47" {
		t.Errorf("env group literals = %+v", stack.envGroups)
	}
	if len(stack.services) != 1 || len(stack.services[0].req.Env) != 1 || stack.services[0].req.Env[0].Value != "42" {
		t.Errorf("nested service env = %+v", stack.services)
	}
}

// Non-string, non-number values stay compiler refusals with a source location,
// never a Go decoder message.
func TestBlueprintEnvValueStillRefusesBooleans(t *testing.T) {
	_, _, problems := CompileBlueprintIR(numericEnvService(`      - key: FLAG
        value: true
`))
	if len(problems) == 0 {
		t.Fatal("boolean env value accepted")
	}
	for _, p := range problems {
		if strings.Contains(p.Message, "unmarshal") {
			t.Errorf("decoder internals leaked: %+v", p)
		}
		if p.Line == 0 {
			t.Errorf("refusal has no source location: %+v", p)
		}
	}
}
