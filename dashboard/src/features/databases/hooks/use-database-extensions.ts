import { useQuery } from "@apollo/client/react";
import { DatabaseExtensionsDocument } from "@/graphql/definitions";
import { nonNull } from "@/common/lib/non-null";

export interface DatabaseExtensionView {
  name: string;
  blocksUpgradeDatabase: boolean;
}

/**
 * The extensions installed in a database, and the ones an upgrade to the
 * latest major cannot carry (Render's databaseExtensions, w3/m676). A
 * database whose extensions cannot be read (suspended, or not serving yet)
 * reads as none: the server refuses the upgrade itself when it can read them.
 */
export function useDatabaseExtensions(id: string) {
  const { data, loading } = useQuery(DatabaseExtensionsDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const extensions: DatabaseExtensionView[] = (data?.databaseExtensions ?? [])
    .filter(nonNull)
    .map((e) => ({
      name: e.name,
      blocksUpgradeDatabase: e.blocksUpgradeDatabase,
    }));
  return { extensions, loading: loading && !data };
}
