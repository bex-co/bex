import { ResourceStatusScreen } from "@/features/resources/resource-status-screen";
import { LazyTabScreen } from "@/components/lazy-tab-screen";

export default function ActivityScreen() {
  return (
    <LazyTabScreen>
      <ResourceStatusScreen activityOnly />
    </LazyTabScreen>
  );
}
