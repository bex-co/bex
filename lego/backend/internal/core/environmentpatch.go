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

package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EnvVarPatch and SecretFilePatch are the neutral sparse configuration
// vocabulary shared by service-local environments and Environment Groups.
// Omitted entries are preserved, and a rename carries the existing opaque
// value without returning it to the caller.
type EnvVarPatch struct {
	Key     string `json:"key"`
	FromKey string `json:"fromKey,omitempty"`
	Value   string `json:"value,omitempty"`
	// ValueSet distinguishes an explicit empty literal from an omitted value.
	// It is populated by JSON adapters and explicit internal constructors.
	ValueSet      bool `json:"-"`
	GenerateValue bool `json:"generateValue,omitempty"`
	Delete        bool `json:"delete,omitempty"`
}

func (p *EnvVarPatch) UnmarshalJSON(data []byte) error {
	type wire EnvVarPatch
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*p = EnvVarPatch(decoded)
	_, p.ValueSet = fields["value"]
	return nil
}

// InvalidEnvVarValueInput is the stable literal-or-generate validation error
// shared by service and Environment Group patches and every public adapter.
func InvalidEnvVarValueInput(key string) error {
	return NewBadRequestError(
		"ENVIRONMENT_VALUE_INPUT_INVALID",
		"provide exactly one of value or generateValue",
		map[string]any{"key": key},
	)
}

type SecretFilePatch struct {
	Name     string `json:"name"`
	FromName string `json:"fromName,omitempty"`
	Content  string `json:"content,omitempty"`
	Delete   bool   `json:"delete,omitempty"`
}

// ApplyEnvVarPatch applies validated sparse operations to env in request
// order. The caller owns transactionality; this helper owns the shared
// secrecy-preserving rename/delete/generation semantics.
func ApplyEnvVarPatch(env map[string]string, writes []EnvVarPatch) error {
	ops := make([]mapPatchOp, len(writes))
	for i, write := range writes {
		write := write
		hasValue := write.ValueSet || write.Value != ""
		ops[i] = mapPatchOp{
			key:        write.Key,
			fromKey:    write.FromKey,
			remove:     write.Delete,
			hasPayload: write.GenerateValue || hasValue,
			value: func(key string) (string, error) {
				if write.GenerateValue == hasValue {
					return "", InvalidEnvVarValueInput(key)
				}
				if write.GenerateValue {
					return GenerateValue()
				}
				return write.Value, nil
			},
		}
	}
	return applyMapPatch(env, ops, ValidEnvKey, mapPatchWording{
		noun:            "environment variable",
		renameConflicts: "delete, value, or generateValue",
		deleteConflicts: "a value or generateValue",
	})
}

// ApplySecretFilePatch is ApplyEnvVarPatch's secret-file counterpart.
func ApplySecretFilePatch(files map[string]string, writes []SecretFilePatch) error {
	ops := make([]mapPatchOp, len(writes))
	for i, write := range writes {
		write := write
		ops[i] = mapPatchOp{
			key:        write.Name,
			fromKey:    write.FromName,
			remove:     write.Delete,
			hasPayload: write.Content != "",
			value: func(string) (string, error) {
				return write.Content, nil
			},
		}
	}
	return applyMapPatch(files, ops, ValidSecretFileName, mapPatchWording{
		noun:            "secret file",
		renameConflicts: "delete or content",
		deleteConflicts: "content",
	})
}

type mapPatchOp struct {
	key        string
	fromKey    string
	remove     bool
	hasPayload bool
	value      func(key string) (string, error)
}

type mapPatchWording struct {
	noun            string
	renameConflicts string
	deleteConflicts string
}

func applyRenameOp(m map[string]string, seen map[string]struct{}, op mapPatchOp, key, fromKey string, valid func(string) bool, wording mapPatchWording) error {
	if !valid(fromKey) {
		return fmt.Errorf("%w: invalid source %s name %q", ErrBadRequest, wording.noun, fromKey)
	}
	if op.remove || op.hasPayload {
		return fmt.Errorf("%w: %s rename %q cannot combine with %s", ErrBadRequest, wording.noun, key, wording.renameConflicts)
	}
	if _, duplicate := seen[fromKey]; duplicate && fromKey != key {
		return fmt.Errorf("%w: conflicting %s operation for %q", ErrBadRequest, wording.noun, fromKey)
	}
	value, ok := m[fromKey]
	if !ok {
		return fmt.Errorf("%w: source %s %q", ErrNotFound, wording.noun, fromKey)
	}
	if _, occupied := m[key]; occupied && key != fromKey {
		return fmt.Errorf("%w: %s rename destination %q already exists", ErrBadRequest, wording.noun, key)
	}
	seen[fromKey] = struct{}{}
	delete(m, fromKey)
	m[key] = value
	return nil
}

func applyMapPatch(m map[string]string, ops []mapPatchOp, valid func(string) bool, wording mapPatchWording) error {
	seen := make(map[string]struct{}, len(ops))
	for _, op := range ops {
		key := strings.TrimSpace(op.key)
		if !valid(key) {
			return fmt.Errorf("%w: invalid %s name %q", ErrBadRequest, wording.noun, key)
		}
		fromKey := strings.TrimSpace(op.fromKey)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("%w: duplicate %s operation for %q", ErrBadRequest, wording.noun, key)
		}
		seen[key] = struct{}{}
		if fromKey != "" {
			if err := applyRenameOp(m, seen, op, key, fromKey, valid, wording); err != nil {
				return err
			}
			continue
		}
		if op.remove {
			if op.hasPayload {
				return fmt.Errorf("%w: %s %q cannot combine delete with %s", ErrBadRequest, wording.noun, key, wording.deleteConflicts)
			}
			delete(m, key)
			continue
		}
		value, err := op.value(key)
		if err != nil {
			return err
		}
		m[key] = value
	}
	return nil
}
