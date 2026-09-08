import { AccessBoundary } from "@/features/capabilities/access-required-screen";
import { ResourceStatusScreen } from "@/features/resources/resource-status-screen";
import { LazyTabScreen } from "@/components/lazy-tab-screen";

export default function StatusScreen() {
  return (
    <LazyTabScreen>
      <AccessBoundary>
        <ResourceStatusScreen />
      </AccessBoundary>
    </LazyTabScreen>
  );
}
