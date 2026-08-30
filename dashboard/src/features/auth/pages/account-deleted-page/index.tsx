import { CheckCircle2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

export default function AccountDeletedPage() {
  const { t } = useTranslations();
  return (
    <AuthPageShell
      title={t("auth.accountDeletedTitle")}
      subtitle={t("auth.accountDeletedSubtitle")}
    >
      <div className="space-y-6 rounded-xl border bg-card p-6 shadow-sm sm:p-8">
        <div className="flex items-start gap-3">
          <CheckCircle2 className="mt-0.5 size-6 shrink-0 text-primary" />
          <p className="text-sm text-muted-foreground">
            {t("auth.accountDeletedStatus")}
          </p>
        </div>
        <Button asChild variant="outline">
          <a href="/">{t("auth.accountDeletedHome")}</a>
        </Button>
      </div>
    </AuthPageShell>
  );
}
