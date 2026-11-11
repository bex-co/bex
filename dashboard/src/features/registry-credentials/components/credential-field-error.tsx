import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";

/** The inline reason under a credential field, announced and linked to it. */
export function CredentialFieldError({
  id,
  reason,
}: {
  id: string;
  reason: keyof typeof en | null;
}) {
  const { t } = useTranslations();
  if (!reason) return null;
  return (
    <p id={id} role="alert" className="text-destructive text-xs">
      {t(reason)}
    </p>
  );
}
