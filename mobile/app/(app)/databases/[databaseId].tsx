import { AccessBoundary } from "@/features/capabilities/access-required-screen";
import { useLocalSearchParams } from "expo-router";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validDatabaseDeepLink } from "@/features/navigation/deep-link";
import { PostgresDetailScreen } from "@/features/postgres/postgres-detail-screen";

export default function DatabaseDetailRoute() {
  const { databaseId } = useLocalSearchParams<{
    databaseId?: string | string[];
  }>();
  if (!validDatabaseDeepLink(databaseId)) return <InvalidDeepLinkScreen />;
  return (
    <AccessBoundary>
      <PostgresDetailScreen databaseId={databaseId} />
    </AccessBoundary>
  );
}
