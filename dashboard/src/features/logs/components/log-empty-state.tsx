import type { ReactNode } from "react";
import { AlertCircle, Hourglass, RefreshCcw, ScrollText } from "lucide-react";
import { EmptyState } from "@/common/components/empty-state";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  mutationErrorMessage,
  relayableReason,
} from "@/common/lib/graphql-error";
import type { LogPanelFill } from "@/features/logs/hooks/use-log-panel-height";
import { LogPanelState } from "./log-panel-state";
import { LogTruncationNotice } from "./log-truncation-notice";
import type { UseLogHistoryResult } from "../hooks/use-log-history";

/**
 * A log view with no lines to show: its search timed out, failed, matched
 * nothing, or found no logs at all. The service, datastore and deploy log
 * views all render it (w5/m125), each naming its own failed and empty copy.
 * A view whose panel fills the screen (the service Logs tab) passes its
 * `fillViewport`, and the state takes the panel's own box instead of a card.
 */
export function LogEmptyState(
  props: { fillViewport?: LogPanelFill } & (
    | { reason: "timeout" }
    | {
        reason: "failed";
        title: string;
        /** The failed read: its server refusal is shown, else generic copy. */
        error: unknown;
        /** Reads the history again; shown as Try again in the panel. */
        onRetry?: () => void;
        /** The live tail is still streaming: new lines will land here. */
        liveTailing?: boolean;
      }
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
        /** The way forward in the panel: Turn on Live, Clear filters. */
        action?: ReactNode;
      }
  ),
) {
  const { t } = useTranslations();
  const fill = props.fillViewport;
  switch (props.reason) {
    case "timeout":
      // The search could not cover any of the range in the server's budget;
      // the same search would time out again (w4/m140).
      return fill !== undefined ? (
        <LogPanelState
          fillViewport={fill}
          role="alert"
          icon={Hourglass}
          title={t("logs.timeoutTitle")}
          body={t("logs.timeoutBody")}
        />
      ) : (
        <EmptyState
          role="alert"
          iconName="AlertCircle"
          title={t("logs.timeoutTitle")}
          description={t("logs.timeoutBody")}
        />
      );
    case "failed":
      return fill !== undefined ? (
        // What failed and what to do, then the server's own words, for
        // support; a read that never got an answer has none (w5/167).
        <LogPanelState
          fillViewport={fill}
          role="alert"
          icon={AlertCircle}
          tone="destructive"
          title={props.title}
          body={t(props.liveTailing ? "logs.errorBodyLive" : "logs.errorBody")}
          detail={relayableReason(props.error) || undefined}
          action={
            props.onRetry ? (
              <Button variant="outline" size="sm" onClick={props.onRetry}>
                <RefreshCcw className="size-3.5" aria-hidden="true" />
                {t("common.tryAgain")}
              </Button>
            ) : undefined
          }
        />
      ) : (
        <EmptyState
          role="alert"
          iconName="AlertCircle"
          title={props.title}
          description={mutationErrorMessage(
            props.error,
            t("common.loadErrorRetry"),
          )}
        />
      );
    case "empty": {
      // A search that matched nothing is not an empty store (w4/195).
      const title = props.filtered
        ? t("logs.emptyFilteredTitle")
        : props.emptyTitle;
      const body = props.filtered
        ? t("logs.emptyFilteredBody")
        : props.emptyBody;
      const empty =
        fill !== undefined ? (
          <LogPanelState
            fillViewport={fill}
            icon={ScrollText}
            title={title}
            body={body}
            action={props.action}
          />
        ) : (
          <EmptyState iconName="ScrollText" title={title} description={body} />
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
