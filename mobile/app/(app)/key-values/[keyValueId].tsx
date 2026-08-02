import { useLocalSearchParams } from "expo-router";
import { KeyValueDetailScreen } from "@/features/keyvalue/keyvalue-detail-screen";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validKeyValueDeepLink } from "@/features/navigation/deep-link";

export default function KeyValueDetailRoute() {
  const { keyValueId } = useLocalSearchParams<{
    keyValueId?: string | string[];
  }>();
  if (!validKeyValueDeepLink(keyValueId)) return <InvalidDeepLinkScreen />;
  return <KeyValueDetailScreen keyValueId={keyValueId} />;
}
