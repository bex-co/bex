// Hand-written TypedDocumentNodes for the service Networking surface
// (w7/m32: inbound IP allowlist for web_service + static_site). These mirror
// the operation in server.graphql / the setServiceIpAllowList mutation that
// `yarn codegen` would generate once it can introspect a live bex-api (codegen
// needs a session token; see codegen.ts). Until then the hook imports its
// typed documents from here so the feature type-checks and tests run offline —
// the same bridge keyvalue/api/operations.ts uses.

import gql from "graphql-tag";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";

export interface IpAllowListEntry {
  cidrBlock: string;
  description: string;
}

interface SetServiceIpAllowListVars {
  id: string;
  entries: IpAllowListEntry[];
}

export interface SetServiceIpAllowListMutation {
  setServiceIpAllowList: {
    id: string | null;
    ipAllowListEntries: Array<{
      cidrBlock: string;
      description: string | null;
    } | null> | null;
  } | null;
}

export const SetServiceIpAllowListDocument = gql`
  mutation SetServiceIpAllowList(
    $id: String!
    $entries: [IPAllowListEntryInput!]
  ) {
    setServiceIpAllowList(id: $id, entries: $entries) {
      id
      ipAllowListEntries {
        cidrBlock
        description
      }
    }
  }
` as unknown as TypedDocumentNode<
  SetServiceIpAllowListMutation,
  SetServiceIpAllowListVars
>;
