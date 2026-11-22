import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A Settings → Agents switch section's failed save, announced; id lets a
 * control name it in aria-describedby.
 */
export function ToggleSaveAlert({
  errorKey,
  id,
}: {
  errorKey: string | null;
  id?: string;
}) {
  const { t } = useTranslations();
  return errorKey ? (
    <p id={id} className="text-destructive text-sm" role="alert">
      {t(errorKey)}
    </p>
  ) : null;
}
