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

import { isDnsLabel } from "@/common/lib/utils/dns-label";
import type {
  DatabaseDetailView,
  DatabaseInstanceTypeView,
} from "@/features/databases/types";

/**
 * The longest read replica name: bex builds the replica's hostname label as
 * `<database id>-ro-<name>`, and every database id is 24 characters, so 35
 * keeps the label within DNS's 63 (ReadReplicaNameMaxLength in
 * lego/backend/internal/postgres/admission.go).
 */
export const READ_REPLICA_NAME_MAX_LENGTH = 35;

/**
 * Why bex-api would refuse a new replica of this name, or null when it
 * accepts it — the dashboard copy of checkReadReplicaNames. Both are tested
 * against lego/backend/internal/postgres/testdata/read-replica-name-vectors.json
 * (w5/m138).
 */
export function readReplicaNameProblem(
  name: string,
  existing: readonly string[],
): "duplicate" | "invalid" | null {
  if (existing.includes(name)) return "duplicate";
  return isDnsLabel(name, READ_REPLICA_NAME_MAX_LENGTH) ? null : "invalid";
}

/** Why another replica cannot be added on this plan and storage, if it can't. */
export type ReadReplicaAddBlock =
  | { kind: "plan" }
  | { kind: "limit"; max: number }
  | { kind: "storage"; min: number; current: number };

/**
 * The plan-level rules bex-api's admission applies to a larger replica set,
 * read from the plan catalog: the plan must allow replicas, have room for one
 * more, and the database must carry the minimum storage. Unknown while the
 * catalog loads (no plan) is never a block — the API stays the authority.
 */
export function readReplicaAddBlock(
  database: Pick<DatabaseDetailView, "diskSizeGB" | "readReplicas">,
  plan: DatabaseInstanceTypeView | undefined,
): ReadReplicaAddBlock | null {
  if (!plan) return null;
  if (plan.maxReadReplicas === 0) return { kind: "plan" };
  if (database.readReplicas.length >= plan.maxReadReplicas) {
    return { kind: "limit", max: plan.maxReadReplicas };
  }
  const current = database.diskSizeGB ?? plan.storageGB;
  if (current < plan.readReplicaMinStorageGB) {
    return { kind: "storage", min: plan.readReplicaMinStorageGB, current };
  }
  return null;
}
