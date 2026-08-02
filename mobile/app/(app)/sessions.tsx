import { ShellScreen } from "@/components/shell-screen";

export default function SessionsScreen() {
  return (
    <ShellScreen
      titleKey="sessions.title"
      bodyKey="sessions.body"
      badgeKey="sessions.gated"
      icon="sparkles"
    />
  );
}
