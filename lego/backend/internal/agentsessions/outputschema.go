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

package agentsessions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Structured output (ADR094 D5): a turn created with an outputSchema ends with
// a result that validates against it, or fails STRUCTURED_OUTPUT_MISSING.
const (
	// maxOutputSchemaBytes bounds the schema a caller attaches to a turn.
	maxOutputSchemaBytes = 64 << 10
	// OutputSchemaDialect is the one JSON Schema dialect a turn schema may
	// declare; a schema with no $schema is read as this dialect.
	outputSchemaDialect = "https://json-schema.org/draft/2020-12/schema"
	// maxResultProblems caps the validation problems reported back to the agent.
	maxResultProblems = 8

	codeOutputSchemaInvalid = "AGENT_SESSION_OUTPUT_SCHEMA_INVALID"
	// FailureCodeStructuredOutputMissing is the failureReasonCode of a turn
	// that required a result and ended without a final valid one.
	FailureCodeStructuredOutputMissing = "STRUCTURED_OUTPUT_MISSING"
	structuredOutputMissingReason      = "The agent ended the turn without submitting the structured result its outputSchema requires."
)

// externalRefs is the loader a turn schema compiles with. Every schema lives in
// memory under one URL, so anything the compiler asks it to load is a reference
// that leaves the schema: a file, a network URL or another dialect's
// metaschema. It never gets to touch the filesystem or the network.
type externalRefs struct{}

func (externalRefs) Load(url string) (any, error) {
	return nil, fmt.Errorf("external reference %s is not allowed", url)
}

const outputSchemaURL = "bex://agent-session/output-schema.json"

// compileOutputSchema compiles raw as a self-contained JSON Schema 2020-12.
func compileOutputSchema(raw []byte) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	object, ok := doc.(map[string]any)
	if !ok {
		return nil, errors.New("must be a JSON object")
	}
	if dialect, present := object["$schema"]; present && strings.TrimSuffix(fmt.Sprint(dialect), "#") != outputSchemaDialect {
		return nil, fmt.Errorf("$schema must be %s", outputSchemaDialect)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(externalRefs{})
	if err := compiler.AddResource(outputSchemaURL, doc); err != nil {
		return nil, err
	}
	return compiler.Compile(outputSchemaURL)
}

// parseTurnOutput validates a request's outputSchema and outputRequired and
// returns what the turn stores. outputRequired defaults to true when a schema
// is set and is meaningless without one.
func parseTurnOutput(schema map[string]any, required *bool) (store.AgentTurnOutput, error) {
	if schema == nil {
		if required != nil && *required {
			return store.AgentTurnOutput{}, core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "outputRequired needs an outputSchema", map[string]any{"field": "outputRequired"})
		}
		return store.AgentTurnOutput{}, nil
	}
	invalid := func(reason string) error {
		return core.NewBadRequestError(codeOutputSchemaInvalid, "outputSchema is not usable: "+reason, map[string]any{"field": "outputSchema"})
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return store.AgentTurnOutput{}, invalid("not valid JSON")
	}
	if len(raw) > maxOutputSchemaBytes {
		return store.AgentTurnOutput{}, invalid(fmt.Sprintf("larger than %d KiB", maxOutputSchemaBytes>>10))
	}
	if bytes.Contains(raw, []byte(`\u0000`)) { // jsonb cannot store NUL
		return store.AgentTurnOutput{}, invalid("must not contain NUL characters")
	}
	if _, err := compileOutputSchema(raw); err != nil {
		return store.AgentTurnOutput{}, invalid(err.Error())
	}
	return store.AgentTurnOutput{Schema: raw, Required: required == nil || *required}, nil
}

// outputArgs reads a GraphQL mutation's outputSchema (JSON text; empty is no
// schema) and outputRequired into the request fields every transport shares.
func outputArgs(args map[string]any) (schema map[string]any, required *bool, err error) {
	if v, ok := args["outputRequired"].(bool); ok {
		required = &v
	}
	text, _ := args["outputSchema"].(string)
	if strings.TrimSpace(text) == "" {
		return nil, required, nil
	}
	invalid := func(reason string) error {
		return core.NewBadRequestError(codeOutputSchemaInvalid, "outputSchema is not usable: "+reason, map[string]any{"field": "outputSchema"})
	}
	if len(text) > maxOutputSchemaBytes*2 { // bound the decode; the marshaled size is checked by parseTurnOutput
		return nil, nil, invalid(fmt.Sprintf("larger than %d KiB", maxOutputSchemaBytes>>10))
	}
	if json.Unmarshal([]byte(text), &schema) != nil || schema == nil {
		return nil, nil, invalid("must be a JSON object")
	}
	return schema, required, nil
}

// checkResult validates result against a turn's stored schema and reports what
// is wrong in words the agent can act on; nil means it conforms.
func checkResult(schemaRaw, result []byte) []string {
	schema, err := compileOutputSchema(schemaRaw)
	if err != nil { // a stored schema was validated at accept time; treat drift as a miss, not a pass
		return []string{"the turn's output schema cannot be compiled"}
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(result))
	if err != nil {
		return []string{"the result is not valid JSON"}
	}
	err = schema.Validate(value)
	if err == nil {
		return nil
	}
	var verr *jsonschema.ValidationError
	if !errors.As(err, &verr) {
		return []string{err.Error()}
	}
	var problems []string
	for _, unit := range verr.BasicOutput().Errors {
		if unit.Error == nil {
			continue
		}
		where := unit.InstanceLocation
		if where == "" {
			where = "(result)"
		}
		problems = append(problems, where+": "+unit.Error.String())
		if len(problems) == maxResultProblems {
			break
		}
	}
	if len(problems) == 0 {
		problems = []string{"the result does not match the output schema"}
	}
	return problems
}

// jsonValue decodes a stored JSON document (a result or a schema) for a view, keeping numbers exact.
func jsonValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil
	}
	return value
}
