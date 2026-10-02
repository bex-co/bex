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

package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DatabaseInstanceType is the display-shaped projection of one lego/types/tiers
// Postgres tier — the create dialog's plan picker source, the managed-Postgres
// sibling of apps.InstanceType. Render's dashboard hardcodes its own Postgres
// instance-type list (no public REST/MCP equivalent to mirror byte-for-byte),
// so this is a bex extension, documented as such — never a fourth hardcoded
// copy of the ladder the w1/m8 catalog collapsed. ID is the Database CRD's
// spec.plan spelling (what createDatabase accepts), matching the other fields.
type DatabaseInstanceType struct {
	ID                        string
	Name                      string
	CPU                       string
	Memory                    string
	StorageGB                 int32
	MaxStorageGB              int32
	SupportsDiskAutoscaling   bool
	SupportsConnectionPooling bool
	MaxReadReplicas           int
	ReadReplicaMinStorageGB   int32
	// SupportsHighAvailability is PlanSupportsHighAvailability for this plan.
	SupportsHighAvailability bool
	// MonthlyUSD is the always-on monthly price from the one price sheet
	// (pricing.yaml, which bex.co/pricing and billing also read), "" for an
	// unlisted tier. The plan picker showed none, so a paid datastore was
	// chosen blind (w4/156).
	MonthlyUSD string
}

// InstanceTypes lists every plan in the shared Postgres catalog, in ladder
// order — read from lego/types/tiers, never a hardcoded copy here (the same
// rule apps.InstanceTypes follows for the compute family).
func (s *Service) InstanceTypes(ctx context.Context) ([]DatabaseInstanceType, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	ids := tiers.Postgres.IDs()
	out := make([]DatabaseInstanceType, len(ids))
	for i, id := range ids {
		t, _ := tiers.Postgres.ByID(id)
		monthlyUSD, _ := pricing.Default.InstanceMonthlyUSD(id, store.ResourceKindPostgres)
		out[i] = DatabaseInstanceType{
			MonthlyUSD:                monthlyUSD,
			ID:                        t.ID,
			Name:                      pgTierDisplayName(id),
			CPU:                       t.CPU,
			Memory:                    t.Memory,
			StorageGB:                 t.StorageGB,
			SupportsHighAvailability:  PlanSupportsHighAvailability(t),
			MaxStorageGB:              tiers.Postgres.MaxStorageGB(t.ID),
			SupportsDiskAutoscaling:   t.SupportsDiskAutoscaling(),
			SupportsConnectionPooling: t.SupportsConnectionPooling(),
			MaxReadReplicas:           t.MaxReadReplicas(),
			ReadReplicaMinStorageGB:   tiers.PostgresReadReplicaMinStorageGB,
		}
	}
	return out, nil
}

// sizeToken matches a trailing size token (256mb, 1gb, 2tb) whose unit should
// be upper-cased in the display name rather than title-cased ("256mb" reads as
// "256MB", not "256mb").
var sizeToken = regexp.MustCompile(`^\d+(mb|gb|tb)$`)

// pgTierDisplayName turns a Postgres plan id into its display spelling, e.g.
// "basic-256mb" -> "Basic 256MB", "free" -> "Free" (Render's Postgres
// family-size naming). The compute sibling (apps.tierDisplayName) title-cases
// hyphen-words; this additionally upper-cases the byte-size unit.
func pgTierDisplayName(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		switch {
		case w == "":
			continue
		case sizeToken.MatchString(w):
			words[i] = strings.ToUpper(w)
		default:
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// CheckHighAvailabilityPlan refuses high availability on a plan below 1 CPU
// (w8/m43) — Render's rule ("available for plans with at least 1 CPU"), and
// ADR030 §6's: a $0 plan never runs a second instance. The one refusal REST,
// GraphQL, MCP and Blueprint share. An empty plan is the catalog default; an
// unknown one is left to the caller's plan validation, which names the valid
// plans.
func CheckHighAvailabilityPlan(plan string) error {
	tier, ok := tiers.Postgres.ByID(tiers.Postgres.CanonicalID(plan))
	if !ok {
		if plan != "" {
			return nil
		}
		tier = tiers.Postgres.Default()
	}
	if PlanSupportsHighAvailability(tier) {
		return nil
	}
	return core.NewBadRequestError(
		"POSTGRES_HA_PLAN_UNSUPPORTED",
		fmt.Sprintf("high availability requires a Postgres plan with at least 1 CPU; plan %q has %s CPU", tier.ID, tier.CPU),
		map[string]any{"plan": tier.ID, "cpu": tier.CPU},
	)
}

// PlanSupportsHighAvailability is the catalog predicate. It is a variable only
// so tests of the HA-enable path (here and in the Blueprint package) can run
// while the shipped catalog has no ≥1-CPU Postgres plan; nothing else assigns it.
var PlanSupportsHighAvailability = tiers.PostgresTier.SupportsHighAvailability

// checkHighAvailabilityPatch applies CheckHighAvailabilityPlan to a PATCH or
// plan change against d. Enabling HA checks the resulting plan; a plan change
// while HA stays on is refused with the remedy, never by silently switching
// HA off. Disabling HA is always allowed, and a database already running HA
// on an unsupported plan keeps it through unrelated edits and an idempotent
// re-enable (no silent conversion; w8/m43 t004).
func checkHighAvailabilityPatch(d *appv1alpha1.Database, plan *string, enableHA *bool) error {
	target := d.Spec.Plan
	if plan != nil {
		target = *plan
	}
	switch {
	case enableHA != nil && *enableHA:
		if d.Spec.HighAvailability && plan == nil {
			return nil
		}
		return CheckHighAvailabilityPlan(target)
	case enableHA == nil && plan != nil && d.Spec.HighAvailability:
		if err := CheckHighAvailabilityPlan(target); err != nil {
			return core.NewBadRequestError(
				"POSTGRES_HA_PLAN_UNSUPPORTED",
				err.Error()+"; disable high availability first",
				map[string]any{"plan": tiers.Postgres.CanonicalID(target)},
			)
		}
	}
	return nil
}
