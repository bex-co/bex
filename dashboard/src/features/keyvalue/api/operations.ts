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

export interface KeyValueIpAllowListQuery {
  keyValueIpAllowList: Array<string | null> | null;
}
export const KeyValueIpAllowListDocument = gql`
  query KeyValueIpAllowList($id: String!) {
    keyValueIpAllowList(id: $id)
  }
` as unknown as TypedDocumentNode<KeyValueIpAllowListQuery, IdVars>;

export interface SetKeyValueIpAllowListMutation {
  setKeyValueIpAllowList: {
    id: string | null;
    ipAllowList: Array<string | null> | null;
  } | null;
}
interface SetKeyValueIpAllowListVars {
  id: string;
  cidrs: string[];
}
export const SetKeyValueIpAllowListDocument = gql`
  mutation SetKeyValueIpAllowList($id: String!, $cidrs: [String]) {
    setKeyValueIpAllowList(id: $id, cidrs: $cidrs) {
      id
      ipAllowList
    }
  }
` as unknown as TypedDocumentNode<
  SetKeyValueIpAllowListMutation,
  SetKeyValueIpAllowListVars
>;
