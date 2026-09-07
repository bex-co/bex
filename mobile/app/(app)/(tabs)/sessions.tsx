import { SessionsListScreen } from "@/features/agent-sessions/sessions-list-screen";
import { LazyTabScreen } from "@/components/lazy-tab-screen";

export default function SessionsScreen() {
  return (
    <LazyTabScreen>
      <SessionsListScreen />
    </LazyTabScreen>
  );
}
