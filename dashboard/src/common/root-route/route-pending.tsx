import { Loader2 } from "lucide-react";
import PendingRouteTitle from "@/common/lib/document-head/pending-route-title";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The router's default pending fallback. Its predecessor (`PendingRouteTitle`
 * alone) rendered no DOM at all, so any navigation slow enough to show the
 * pending state unmounted the whole page into a blank document — the
 * "white flash" on sidebar navigation. The pending fallback fills whatever
 * slot suspends — the entire viewport on a top-level navigation, a detail
 * shell's content outlet on a tab switch — so it must paint something in
 * both: a centered spinner that stretches with `flex-1` inside a flex shell
 * and still reserves height (`min-h-[50vh]`) when it is the only content.
 */
export default function RoutePending() {
  const { t } = useTranslations();
  return (
    <>
      <PendingRouteTitle />
      <div
        role="status"
        aria-label={t("common.loading")}
        className="flex min-h-[50vh] w-full flex-1 items-center justify-center"
      >
        <Loader2 className="text-muted-foreground size-6 animate-spin" />
      </div>
    </>
  );
}
