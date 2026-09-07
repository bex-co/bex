import { ResourceStatusScreen } from "@/features/resources/resource-status-screen";
import { LazyTabScreen } from "@/components/lazy-tab-screen";

export default function StatusScreen() {
  return (
    <LazyTabScreen>
      <ResourceStatusScreen />
    </LazyTabScreen>
  );
}
