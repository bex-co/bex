import type { ApolloClient } from "@apollo/client";
import { KeyValueConnectionInfoDocument } from "@/graphql/definitions";
import type { KeyValueConnectionInfoView } from "@/features/keyvalue/types";
import {
  useConnectionReveal,
  type ConnectionReveal,
} from "@/features/capabilities/hooks/use-connection-reveal";

export type UseConnectionInfoResult =
  ConnectionReveal<KeyValueConnectionInfoView>;

async function fetchConnectionInfo(
  client: ApolloClient,
  id: string,
): Promise<KeyValueConnectionInfoView> {
  const res = await client.query({
    query: KeyValueConnectionInfoDocument,
    variables: { id },
    fetchPolicy: "network-only",
    errorPolicy: "none",
  });
  const ci = res.data?.keyValueConnectionInfo;
  return {
    internalConnectionString: ci?.internalConnectionString ?? "",
    externalConnectionString: ci?.externalConnectionString ?? "",
    cliCommand: ci?.cliCommand ?? "",
  };
}

/** A Key Value store's connection URIs, fetched only on Reveal. */
export function useConnectionInfo(id: string): UseConnectionInfoResult {
  return useConnectionReveal(id, fetchConnectionInfo);
}
