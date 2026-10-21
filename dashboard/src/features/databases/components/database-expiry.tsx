import { useNow } from "@/common/hooks/use-now";
import { useTranslations } from "@/common/hooks/use-translations";
import { RelativeUntil } from "@/common/components/relative-time";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  expiryTone,
  freeDeletionAt,
  freeExpiryState,
  isExpiryWarning,
  utcDate,
} from "@/features/databases/lib/expiry";

/**
 * The Details card's Expires value (w8/m126): when the free database expires,
 * and, within the warning lead or after expiry, a warning with the way out —
 * the Plan section's upgrade picker.
 */
export function DatabaseExpiryValue({ expiresAt }: { expiresAt: string }) {
  const { t } = useTranslations();
  const state = freeExpiryState(expiresAt, useNow());
  return (
    <div className="space-y-1">
      <RelativeUntil value={expiresAt} title={expiresAt} />
      {isExpiryWarning(state) ? (
        <p role="alert" className={cn("text-sm", expiryTone(state))}>
          {state === "expired"
            ? t("databases.expiryExpired", {
                deleteAt: utcDate(freeDeletionAt(expiresAt)),
              })
            : t("databases.expirySoon", { expiresAt: utcDate(expiresAt) })}{" "}
          <a href="#plan" className="font-medium underline">
            {t("databases.expiryUpgrade")}
          </a>
        </p>
      ) : null}
    </div>
  );
}

/** A list row's compact hint: shown only near or past expiry. */
export function DatabaseExpiryHint({
  expiresAt,
}: {
  expiresAt: string | null | undefined;
}) {
  const { t } = useTranslations();
  const state = freeExpiryState(expiresAt, useNow());
  if (!expiresAt || !isExpiryWarning(state)) return null;
  return (
    <span
      className={cn("text-xs whitespace-nowrap", expiryTone(state))}
      title={expiresAt}
    >
      {state === "expired"
        ? t("databases.expiryHintExpired")
        : t("databases.expiryHintSoon")}
    </span>
  );
}
