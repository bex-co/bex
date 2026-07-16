import { CheckCircle2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

export default function DeviceSuccessPage() {
  const { t } = useTranslations();
  return (
    <AuthPageShell
      title={t("auth.deviceSuccessTitle")}
      subtitle={t("auth.deviceSuccessSubtitle")}
    >
      <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
        <CheckCircle2 className="size-5 text-primary" />
        <span>{t("auth.deviceSuccessHint")}</span>
      </div>
    </AuthPageShell>
  );
}
