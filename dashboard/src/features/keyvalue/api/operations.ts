// Hand-written TypedDocumentNodes for the Key Value Networking surface
// (w7/m5: external-endpoint IP allowlist). These MIRROR the operations in
// keyvalue.graphql — the canonical source `yarn codegen` reads once it can
// introspect a live bex-api. Until then (codegen needs a session token, see
// codegen.ts), the panel/hook import their typed documents from here so the
// feature type-checks and tests run offline — the same bridge
// databases/api/operations.ts uses. When codegen is next run, migrate these
// imports to `@/graphql/definitions` and delete this file.

import gql from "graphql-tag";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";

interface IdVars {
  id: string;
}

// One allowlist entry: the CIDR plus the operator-facing label a human gave
// it — both persist end to end (w4/m24).
export interface IpAllowListEntry {
  cidrBlock: string;
  description: string;
}
export interface KeyValueIpAllowListQuery {
  keyValue: {
    id: string | null;
    ipAllowListEntries: Array<{
      cidrBlock: string;
      description: string | null;
    } | null> | null;
  } | null;
}
export const KeyValueIpAllowListDocument = gql`
  query KeyValueIpAllowList($id: String!) {
    keyValue(id: $id) {
      id
      ipAllowListEntries {
        cidrBlock
        description
      }
    }
  }
` as unknown as TypedDocumentNode<KeyValueIpAllowListQuery, IdVars>;

export interface SetKeyValueIpAllowListMutation {
  setKeyValueIpAllowList: {
    id: string | null;
    ipAllowListEntries: Array<{
      cidrBlock: string;
      description: string | null;
    } | null> | null;
  } | null;
}
interface SetKeyValueIpAllowListVars {
  id: string;
  entries: IpAllowListEntry[];
}
export const SetKeyValueIpAllowListDocument = gql`
  mutation SetKeyValueIpAllowList(
    $id: String!
    $entries: [IPAllowListEntryInput!]
  ) {
    setKeyValueIpAllowList(id: $id, entries: $entries) {
      id
      ipAllowListEntries {
        cidrBlock
        description
      }
    }
  }
` as unknown as TypedDocumentNode<
  SetKeyValueIpAllowListMutation,
  SetKeyValueIpAllowListVars
>;

// --- plan update (m16) ---

export interface UpdateKeyValuePlanVars {
  id: string;
  plan: string;
}
export interface UpdateKeyValuePlanMutation {
  updateKeyValuePlan: {
    id: string | null;
    name: string | null;
    plan: string | null;
    status: string | null;
  } | null;
}
export const UpdateKeyValuePlanDocument = gql`
  mutation UpdateKeyValuePlan($id: String!, $plan: String!) {
    updateKeyValuePlan(id: $id, plan: $plan) {
      id
      name
      plan
      status
    }
  }
` as unknown as TypedDocumentNode<
  UpdateKeyValuePlanMutation,
  UpdateKeyValuePlanVars
>;
