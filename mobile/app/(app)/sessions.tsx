import { ShellScreen } from "@/components/shell-screen";
import { TopBar } from "@/components/top-bar";
import { useTranslations } from "@/common/hooks/use-translations";

export default function SessionsScreen() {
  const { t } = useTranslations();
  return (
    <ShellScreen
      titleKey="sessions.title"
      bodyKey="sessions.body"
      badgeKey="sessions.gated"
      icon="sparkles"
      header={<TopBar title={t("navigation.sessions")} />}
    />
  );
}
