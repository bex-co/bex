import { CheckCircle2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { AuthStatusCard } from "@/features/auth/components/auth-status-card";

export default function AccountDeletedPage() {
  const { t } = useTranslations();
  return (
    <AuthPageShell
      title={t("auth.accountDeletedTitle")}
      subtitle={t("auth.accountDeletedSubtitle")}
    >
      <AuthStatusCard
        icon={<CheckCircle2 className="mt-0.5 size-6 shrink-0 text-primary" />}
        message={t("auth.accountDeletedStatus")}
        action={
          <Button asChild variant="outline">
            <a href="/">{t("auth.accountDeletedHome")}</a>
          </Button>
        }
      />
    </AuthPageShell>
  );
}
