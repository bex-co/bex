import { ShellScreen } from "@/components/shell-screen";
import { Button } from "@/components/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAuth } from "@/features/auth/auth-provider";
import { WorkspaceSwitcher } from "@/features/workspaces/workspace-switcher";

export default function StatusScreen() {
  const { t } = useTranslations();
  const { signOut } = useAuth();
  return (
    <ShellScreen titleKey="status.title" bodyKey="status.body" icon="pulse">
      <WorkspaceSwitcher />
      <Button
        type="outline"
        onPress={() => void signOut().catch(() => undefined)}
        accessibilityLabel={t("auth.signOut")}
      >
        {t("auth.signOut")}
      </Button>
    </ShellScreen>
  );
}
