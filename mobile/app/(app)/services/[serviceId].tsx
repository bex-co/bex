import { useLocalSearchParams } from "expo-router";
import { ShellScreen } from "@/components/shell-screen";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validServiceDeepLink } from "@/features/navigation/deep-link";

export default function ServiceDeepLinkScreen() {
  const { serviceId } = useLocalSearchParams<{
    serviceId?: string | string[];
  }>();
  if (!validServiceDeepLink(serviceId)) return <InvalidDeepLinkScreen />;
  return (
    <ShellScreen
      titleKey="deepLink.serviceTitle"
      bodyKey="deepLink.serviceBody"
      icon="cube-outline"
    />
  );
}
