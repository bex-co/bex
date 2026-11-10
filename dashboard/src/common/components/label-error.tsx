import { useTranslations } from "@/common/hooks/use-translations";
import { isInvalidLabel } from "@/common/lib/utils/resource-label";

/**
 * The inline refusal under a free-form name field, shown before submit so the
 * server's English `DISPLAY_NAME_*` toast is unreachable from the form. Wire
 * the field with `aria-invalid={isInvalidLabel(v)}` and `aria-describedby={id}`.
 */
export function LabelError({ id, value }: { id: string; value: string }) {
  const { t } = useTranslations();
  if (!isInvalidLabel(value)) return null;
  return (
    <p id={id} role="alert" className="text-xs text-destructive">
      {t("common.labelInvalid")}
    </p>
  );
}
