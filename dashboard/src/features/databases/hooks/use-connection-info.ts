import type { ApolloClient } from "@apollo/client";
import { DatabaseConnectionInfoDocument } from "@/graphql/definitions";
import type { ConnectionInfoView } from "@/features/databases/types";
import {
  useConnectionReveal,
  type ConnectionReveal,
} from "@/features/capabilities/hooks/use-connection-reveal";

export type UseConnectionInfoResult = ConnectionReveal<ConnectionInfoView>;

// `network-only` so a reveal is always fresh; `errorPolicy: none` so an
// authz/not-provisioned error surfaces to the panel (docs/ADR006-bex-api.md:
// connection-info requires the sensitive-read scope and 404s until CNPG has
// generated the Secret).
async function fetchConnectionInfo(
  client: ApolloClient,
  id: string,
): Promise<ConnectionInfoView> {
  const res = await client.query({
    query: DatabaseConnectionInfoDocument,
    variables: { id },
    fetchPolicy: "network-only",
    errorPolicy: "none",
  });
  const ci = res.data?.databaseConnectionInfo;
  return {
    password: ci?.password ?? "",
    internalConnectionString: ci?.internalConnectionString ?? "",
    externalConnectionString: ci?.externalConnectionString ?? "",
    psqlCommand: ci?.psqlCommand ?? "",
    serverCaCertificate: ci?.serverCaCertificate ?? "",
    readReplicaConnectionStrings: [],
  };
}

/** A database's connection info and password, fetched only on Reveal. */
export function useConnectionInfo(id: string): UseConnectionInfoResult {
  return useConnectionReveal(id, fetchConnectionInfo);
}
