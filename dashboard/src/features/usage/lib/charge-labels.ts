// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
 * The words the charge tree uses for what bex-api meters. A key per resource
 * kind (a category) and per usage kind (a charge line); `charge-labels.test.ts`
 * reads both lists from the Go constants in `lego/backend/internal/store`, so a
 * kind added there without a label here fails the suite instead of reaching a
 * customer as its raw meter name. The unit and the rate on a line come from
 * bex-api (`pricing.unitFor`), not from here.
 */

/** A resource kind's category header in the charge tree. */
export const CATEGORY_LABEL_KEYS: Record<string, string> = {
  service: "usage.categoryServices",
  postgres: "usage.categoryPostgres",
  key_value: "usage.categoryKeyValue",
  object_storage: "usage.categoryObjectStorage",
  custom_domains: "usage.categoryCustomDomains",
  sandbox: "usage.categorySandboxes",
  agent_session: "usage.categoryAgentSessions",
};

/** Meter kinds read as charge names on a bill, not as raw meter identifiers. */
export const CHARGE_LABEL_KEYS: Record<string, string> = {
  instance_seconds: "usage.chargeCompute",
  egress_bytes: "usage.chargeBandwidth",
  build_seconds: "usage.chargeBuild",
  storage_gb_seconds: "usage.chargeStorage",
  sandbox_compute_seconds: "usage.chargeSandboxCompute",
  sandbox_memory_gb_seconds: "usage.chargeSandboxMemory",
  sandbox_snapshot_gb_seconds: "usage.chargeSandboxSnapshotStorage",
  object_storage_gb_seconds: "usage.chargeObjectStorage",
  object_storage_ingress_bytes: "usage.chargeObjectStorageIngress",
  object_storage_egress_bytes: "usage.chargeObjectStorageEgress",
  disk_gb_seconds: "usage.chargeDisk",
  custom_domain_seconds: "usage.chargeCustomDomains",
  agent_model_cost_microusd: "usage.chargeAgentModelCost",
  agent_model_input_tokens: "usage.chargeAgentModelInputTokens",
  agent_model_output_tokens: "usage.chargeAgentModelOutputTokens",
  agent_model_cache_read_tokens: "usage.chargeAgentModelCacheReadTokens",
  agent_model_cache_write_tokens: "usage.chargeAgentModelCacheWriteTokens",
};

/**
 * What bex-api names a workspace's object storage resource: it has none of its
 * own (one resource per workspace, whose id is the workspace's), so
 * `usage/service.go` fills the name with this English placeholder
 * (`objectStorageResourceName`; `charge-labels.test.ts` keeps the two equal).
 */
export const OBJECT_STORAGE_PLACEHOLDER_NAME = "Object storage";

/**
 * What bex-api names a workspace's custom domain line (w7/m429): one per
 * workspace, keyed by the workspace id, with no name of its own, so
 * `usage/service.go` fills the name with this English placeholder
 * (`customDomainsResourceName`; `charge-labels.test.ts` keeps the two equal).
 */
export const CUSTOM_DOMAINS_PLACEHOLDER_NAME = "Custom domains";

/** The placeholder name of each resource a workspace has exactly one of. */
const WORKSPACE_RESOURCE_PLACEHOLDERS: Record<string, string> = {
  object_storage: OBJECT_STORAGE_PLACEHOLDER_NAME,
  custom_domains: CUSTOM_DOMAINS_PLACEHOLDER_NAME,
};

/**
 * Whether a resource is one a workspace has exactly one of and that has no name
 * of its own (its object storage, its custom domain line), so its row reads as
 * the category's translated name and a category holding only it opens straight
 * onto its charge lines.
 */
export function isUnnamedWorkspaceResource(resource: {
  resourceKind: string;
  serviceName: string;
}): boolean {
  const placeholder = WORKSPACE_RESOURCE_PLACEHOLDERS[resource.resourceKind];
  return (
    placeholder !== undefined &&
    (resource.serviceName === "" || resource.serviceName === placeholder)
  );
}
