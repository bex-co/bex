import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A non-blocking note under the Destination URL when its host is one bex can
 * never deliver to (a loopback, private or cluster-internal address). Render
 * accepts any URL and reports the failed delivery later, so the form still
 * saves; the note just says so now instead of after the first "Transport
 * error" (w21/046).
 */
export function WebhookUrlNotice({ id }: { id: string }) {
  const { t } = useTranslations();
  return (
    <p id={id} role="status" className="text-muted-foreground text-sm">
      {t("webhooks.destinationUnreachableNotice")}
    </p>
  );
}
