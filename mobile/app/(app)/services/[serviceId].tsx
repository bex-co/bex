import { AccessBoundary } from "@/features/capabilities/access-required-screen";
import { useLocalSearchParams } from "expo-router";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validServiceDeepLink } from "@/features/navigation/deep-link";
import { ServiceDetailScreen } from "@/features/services/service-detail-screen";

export default function ServiceDeepLinkScreen() {
  const { serviceId } = useLocalSearchParams<{
    serviceId?: string | string[];
  }>();
  if (!validServiceDeepLink(serviceId)) return <InvalidDeepLinkScreen />;
  return (
    <AccessBoundary>
      <ServiceDetailScreen serviceId={serviceId} />
    </AccessBoundary>
  );
}
