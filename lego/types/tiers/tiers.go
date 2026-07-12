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

// Package tiers is bex's single instance-size catalog: one reviewed YAML
// (tiers.yaml, embedded at build time) holding one ladder per product surface
// — Compute (App/web-service instance types), Postgres (Database instance
// types), and Valkey (KeyValue instance types) — replacing what used to be
// independent hardcoded copies in the operator, the backend store, and the
// docs that could only drift apart.
//
// It mirrors Render's published ladders (render.com/docs/compute-plans and
// their Postgres instance types) and carries no pricing — prices are
// Metronome's; this is a resource catalog only, so the operator (which
// imports this package) never has money to depend on even by accident.
package tiers

import (
	_ "embed"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
)

//go:embed tiers.yaml
var catalogYAML []byte

// ComputeTier is one rung of the App/web-service ladder: a named, fixed pod
// resource allocation. The operator sets a pod's requests == limits from
// CPU/Memory (Guaranteed QoS) — same mechanism for every tier, only the
// numbers differ.
type ComputeTier struct {
	// ID is the App CRD's spec.tier value (and the store's app/tenant "plan"
	// column) — DNS-label-safe, hyphenated (e.g. "pro-plus").
	ID string `json:"id"`
	// RenderPlan is Render's own public-API `plan` spelling (their OpenAPI
	// enum, e.g. "pro_plus") — used only at the public API boundary; never
	// leaks into spec.tier or internal storage.
	RenderPlan string `json:"renderPlan"`
	// CPU and Memory are k8s resource.Quantity strings.
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// PostgresTier is one rung of the managed-Postgres ladder (the Database
// CRD's spec.plan), named in Render's family-size style (basic-256mb, ...).
type PostgresTier struct {
	ID string `json:"id"`
	// CPU and Memory are k8s resource.Quantity strings (per instance).
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	// StorageGB is the plan's volume-size floor — spec.storageGB may grow
	// past it, never shrink below.
	StorageGB int32 `json:"storageGB"`
	// Instances is the CNPG cluster size.
	Instances int32 `json:"instances"`
	// Backup is the durability axis (docs/ADR009-postgresql-management.md §5): when
	// true and the operator's backup store is configured, the plan gets
	// continuous WAL archiving + a daily base backup to object storage, which
	// is what makes point-in-time recovery available. Free plans keep it off
	// (Render's Free is ephemeral); paid plans opt in.
	Backup bool `json:"backup"`
}

// ValkeyTier is one rung of the managed key-value ladder (the KeyValue CRD's
// spec.plan), the Redis-compatible sibling of the Postgres family. MVP tiers are
// single-instance (Instances==1), differing by compute and storage.
type ValkeyTier struct {
	ID string `json:"id"`
	// CPU and Memory are k8s resource.Quantity strings (per instance).
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	// StorageGB is the plan's volume-size floor — spec.storageGB may grow
	// past it, never shrink below.
	StorageGB int32 `json:"storageGB"`
	// Instances is the Valkey StatefulSet size (1 for MVP).
	Instances int32 `json:"instances"`
}

// ComputeCatalog is the parsed, validated compute ladder with O(1) lookups
// by both vocabularies a caller might have in hand.
type ComputeCatalog struct {
	tiers     []ComputeTier
	byID      map[string]ComputeTier
	byRender  map[string]ComputeTier
	defaultID string
}

// PostgresCatalog is the parsed, validated Postgres ladder.
type PostgresCatalog struct {
	tiers     []PostgresTier
	byID      map[string]PostgresTier
	defaultID string
}

// ValkeyCatalog is the parsed, validated key-value (Valkey) ladder.
type ValkeyCatalog struct {
	tiers     []ValkeyTier
	byID      map[string]ValkeyTier
	defaultID string
}

// Compute, Postgres, and Valkey are parsed once at package init from the
// embedded YAML. A malformed catalog panics at init — this is checked-in data
// with its own test (tiers_test.go); it must never fail silently at
// reconcile/request time.
var Compute, Postgres, Valkey = mustLoad(catalogYAML)

// --- Compute ---

// Default returns the tier a plan-less App/tenant defaults to (mirrors the
// backend store's prior normalizeTier("") => "free" behavior).
func (c ComputeCatalog) Default() ComputeTier { return c.byID[c.defaultID] }

// ByID looks up a tier by its spec.tier / store "plan" spelling.
func (c ComputeCatalog) ByID(id string) (ComputeTier, bool) {
	t, ok := c.byID[id]
	return t, ok
}

// ByRenderPlan looks up a tier by Render's public-API `plan` spelling.
func (c ComputeCatalog) ByRenderPlan(plan string) (ComputeTier, bool) {
	t, ok := c.byRender[plan]
	return t, ok
}

// IDs returns every valid spec.tier id, in catalog order — for error messages
// that need to list the valid names (e.g. "tier must be one of: ...").
func (c ComputeCatalog) IDs() []string {
	ids := make([]string, len(c.tiers))
	for i, t := range c.tiers {
		ids[i] = t.ID
	}
	return ids
}

// RenderPlans returns every valid Render `plan` spelling, in catalog order —
// the public-API-facing sibling of IDs (which lists the internal spec.tier
// spelling instead), for error messages the plan-change verb needs.
func (c ComputeCatalog) RenderPlans() []string {
	plans := make([]string, len(c.tiers))
	for i, t := range c.tiers {
		plans[i] = t.RenderPlan
	}
	return plans
}

// Resources returns the pod requests/limits (as parseable Quantity strings)
// for a tier id; ok is false for an empty or unknown id — the caller's
// existing best-effort (no resource constraints) fallback.
func (c ComputeCatalog) Resources(id string) (cpu, memory string, ok bool) {
	t, found := c.byID[id]
	if !found {
		return "", "", false
	}
	return t.CPU, t.Memory, true
}

// --- Postgres ---

// Default returns the plan an empty/unknown Database spec.plan resolves to
// (the operator's prior dbPlans fallback).
func (c PostgresCatalog) Default() PostgresTier { return c.byID[c.defaultID] }

// ByID looks up a Postgres plan by the Database CRD's spec.plan spelling.
func (c PostgresCatalog) ByID(id string) (PostgresTier, bool) {
	t, ok := c.byID[id]
	return t, ok
}

// IDs returns every valid spec.plan id, in catalog order.
func (c PostgresCatalog) IDs() []string {
	ids := make([]string, len(c.tiers))
	for i, t := range c.tiers {
		ids[i] = t.ID
	}
	return ids
}

// --- Valkey ---

// Default returns the plan an empty/unknown KeyValue spec.plan resolves to
// (mirrors the postgres family's free default).
func (c ValkeyCatalog) Default() ValkeyTier { return c.byID[c.defaultID] }

// ByID looks up a Valkey plan by the KeyValue CRD's spec.plan spelling.
func (c ValkeyCatalog) ByID(id string) (ValkeyTier, bool) {
	t, ok := c.byID[id]
	return t, ok
}

// IDs returns every valid spec.plan id, in catalog order.
func (c ValkeyCatalog) IDs() []string {
	ids := make([]string, len(c.tiers))
	for i, t := range c.tiers {
		ids[i] = t.ID
	}
	return ids
}

// --- loading ---

type catalogFile struct {
	Compute struct {
		Default string        `json:"default"`
		Tiers   []ComputeTier `json:"tiers"`
	} `json:"compute"`
	Postgres struct {
		Default string         `json:"default"`
		Tiers   []PostgresTier `json:"tiers"`
	} `json:"postgres"`
	Valkey struct {
		Default string       `json:"default"`
		Tiers   []ValkeyTier `json:"tiers"`
	} `json:"valkey"`
}

func mustLoad(raw []byte) (ComputeCatalog, PostgresCatalog, ValkeyCatalog) {
	c, p, v, err := parse(raw)
	if err != nil {
		panic(fmt.Sprintf("tiers: %v", err))
	}
	return c, p, v
}

func parse(raw []byte) (ComputeCatalog, PostgresCatalog, ValkeyCatalog, error) {
	var f catalogFile
	if err := yaml.UnmarshalStrict(raw, &f); err != nil {
		return ComputeCatalog{}, PostgresCatalog{}, ValkeyCatalog{}, fmt.Errorf("decode: %w", err)
	}
	c, err := parseCompute(f.Compute.Tiers, f.Compute.Default)
	if err != nil {
		return ComputeCatalog{}, PostgresCatalog{}, ValkeyCatalog{}, fmt.Errorf("compute: %w", err)
	}
	p, err := parsePostgres(f.Postgres.Tiers, f.Postgres.Default)
	if err != nil {
		return ComputeCatalog{}, PostgresCatalog{}, ValkeyCatalog{}, fmt.Errorf("postgres: %w", err)
	}
	v, err := parseValkey(f.Valkey.Tiers, f.Valkey.Default)
	if err != nil {
		return ComputeCatalog{}, PostgresCatalog{}, ValkeyCatalog{}, fmt.Errorf("valkey: %w", err)
	}
	return c, p, v, nil
}

func parseCompute(entries []ComputeTier, defaultID string) (ComputeCatalog, error) {
	if len(entries) == 0 {
		return ComputeCatalog{}, fmt.Errorf("no tiers defined")
	}
	byID := make(map[string]ComputeTier, len(entries))
	byRender := make(map[string]ComputeTier, len(entries))
	for _, t := range entries {
		if err := validateEntry(t.ID, t.CPU, t.Memory, byID); err != nil {
			return ComputeCatalog{}, err
		}
		if t.RenderPlan == "" {
			return ComputeCatalog{}, fmt.Errorf("tier %q has an empty renderPlan", t.ID)
		}
		if _, dup := byRender[t.RenderPlan]; dup {
			return ComputeCatalog{}, fmt.Errorf("duplicate renderPlan %q (tier %q)", t.RenderPlan, t.ID)
		}
		byID[t.ID] = t
		byRender[t.RenderPlan] = t
	}
	if err := validateDefault(defaultID, func(id string) bool { _, ok := byID[id]; return ok }); err != nil {
		return ComputeCatalog{}, err
	}
	return ComputeCatalog{tiers: entries, byID: byID, byRender: byRender, defaultID: defaultID}, nil
}

func parsePostgres(entries []PostgresTier, defaultID string) (PostgresCatalog, error) {
	if len(entries) == 0 {
		return PostgresCatalog{}, fmt.Errorf("no tiers defined")
	}
	byID := make(map[string]PostgresTier, len(entries))
	for _, t := range entries {
		if err := validateEntry(t.ID, t.CPU, t.Memory, byID); err != nil {
			return PostgresCatalog{}, err
		}
		if err := validateDatastore(t.ID, t.StorageGB, t.Instances); err != nil {
			return PostgresCatalog{}, err
		}
		byID[t.ID] = t
	}
	if err := validateDefault(defaultID, func(id string) bool { _, ok := byID[id]; return ok }); err != nil {
		return PostgresCatalog{}, err
	}
	return PostgresCatalog{tiers: entries, byID: byID, defaultID: defaultID}, nil
}

func parseValkey(entries []ValkeyTier, defaultID string) (ValkeyCatalog, error) {
	if len(entries) == 0 {
		return ValkeyCatalog{}, fmt.Errorf("no tiers defined")
	}
	byID := make(map[string]ValkeyTier, len(entries))
	for _, t := range entries {
		if err := validateEntry(t.ID, t.CPU, t.Memory, byID); err != nil {
			return ValkeyCatalog{}, err
		}
		if err := validateDatastore(t.ID, t.StorageGB, t.Instances); err != nil {
			return ValkeyCatalog{}, err
		}
		byID[t.ID] = t
	}
	if err := validateDefault(defaultID, func(id string) bool { _, ok := byID[id]; return ok }); err != nil {
		return ValkeyCatalog{}, err
	}
	return ValkeyCatalog{tiers: entries, byID: byID, defaultID: defaultID}, nil
}

// validateDatastore checks the fields a datastore tier (Postgres or Valkey) has
// beyond a plain compute rung: positive storage and instance counts.
func validateDatastore(id string, storageGB, instances int32) error {
	if storageGB <= 0 {
		return fmt.Errorf("tier %q: storageGB must be > 0", id)
	}
	if instances <= 0 {
		return fmt.Errorf("tier %q: instances must be > 0", id)
	}
	return nil
}

// validateEntry checks the fields every family shares: a unique non-empty id
// and parseable cpu/memory quantities. seen is the family's ids so far.
func validateEntry[T any](id, cpu, memory string, seen map[string]T) error {
	if id == "" {
		return fmt.Errorf("tier has an empty id")
	}
	if _, dup := seen[id]; dup {
		return fmt.Errorf("duplicate tier id %q", id)
	}
	if _, err := resource.ParseQuantity(cpu); err != nil {
		return fmt.Errorf("tier %q: bad cpu quantity %q: %w", id, cpu, err)
	}
	if _, err := resource.ParseQuantity(memory); err != nil {
		return fmt.Errorf("tier %q: bad memory quantity %q: %w", id, memory, err)
	}
	return nil
}

func validateDefault(defaultID string, exists func(string) bool) error {
	if defaultID == "" {
		return fmt.Errorf("default is required")
	}
	if !exists(defaultID) {
		return fmt.Errorf("default %q is not a defined tier", defaultID)
	}
	return nil
}
