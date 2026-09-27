import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The capped-view banner every paged log surface shows while the server
 * reports older history (`hasMore`, w4/m107). Scrolling the pane to its top
 * pages back; the button is the same action for a keyboard user or a pane too
 * short to scroll (a client-side filter can leave the deploy panel underfilled,
 * w4/m136, and a time-boxed search can return an empty or short page, w4/m140).
 */
export function LogTruncationNotice({
  loadingOlder,
  onLoadOlder,
  partial = false,
}: {
  loadingOlder: boolean;
  onLoadOlder: () => void;
  /**
   * Fewer than a page of lines came back while more history remains: the
   * server stopped a long search at its time budget, so the rest of the range
   * is unsearched rather than capped (w4/m140).
   */
  partial?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="status"
      className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground"
    >
      <span>
        {partial ? t("logs.partialSearchNotice") : t("logs.truncatedNotice")}
      </span>
      <Button
        size="sm"
        variant="outline"
        disabled={loadingOlder}
        onClick={onLoadOlder}
      >
        {loadingOlder ? t("logs.loadingOlder") : t("logs.loadOlder")}
      </Button>
    </div>
  );
}
