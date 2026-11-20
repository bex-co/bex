import { useTranslations } from "@/common/hooks/use-translations";

/** A Settings → Agents switch section's failed save, announced. */
export function ToggleSaveAlert({ errorKey }: { errorKey: string | null }) {
  const { t } = useTranslations();
  return errorKey ? (
    <p className="text-destructive text-sm" role="alert">
      {t(errorKey)}
    </p>
  ) : null;
}
