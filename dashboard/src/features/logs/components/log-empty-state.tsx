import { EmptyState } from "@/common/components/empty-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { LogTruncationNotice } from "./log-truncation-notice";
import type { UseLogHistoryResult } from "../hooks/use-log-history";

/**
 * A log view with no lines to show: its search timed out, failed, matched
 * nothing, or found no logs at all. The service, datastore and deploy log
 * views all render it (w5/m125), each naming its own failed and empty copy.
 */
export function LogEmptyState(
  props:
    | { reason: "timeout" }
    | { reason: "failed"; title: string; message: string }
    | {
        reason: "empty";
        filtered: boolean;
        emptyTitle: string;
        emptyBody: string;
        /** The older pages not searched yet; omitted where the view shows its own notice. */
        pages?: Pick<
          UseLogHistoryResult,
          "hasMore" | "loadingOlder" | "loadOlder"
        >;
      },
) {
  const { t } = useTranslations();
  switch (props.reason) {
    case "timeout":
      // The search could not cover any of the range in the server's budget;
      // the same search would time out again (w4/m140).
      return (
        <EmptyState
          iconName="AlertCircle"
          title={t("logs.timeoutTitle")}
          description={t("logs.timeoutBody")}
        />
      );
    case "failed":
      return (
        <EmptyState
          iconName="AlertCircle"
          title={props.title}
          description={props.message}
        />
      );
    case "empty": {
      // A search that matched nothing is not an empty store (w4/195).
      const empty = (
        <EmptyState
          iconName="ScrollText"
          title={
            props.filtered ? t("logs.emptyFilteredTitle") : props.emptyTitle
          }
          description={
            props.filtered ? t("logs.emptyFilteredBody") : props.emptyBody
          }
        />
      );
      // Nothing in the part searched so far, but the rest of the range is
      // still unsearched: say so, and offer to keep going (w4/m140).
      return props.pages?.hasMore ? (
        <div className="space-y-2">
          <LogTruncationNotice
            partial
            loadingOlder={props.pages.loadingOlder}
            onLoadOlder={props.pages.loadOlder}
          />
          {empty}
        </div>
      ) : (
        empty
      );
    }
  }
}
