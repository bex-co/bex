import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { SLOT } from "@/common/lib/translation-slot";

/** Render's page on pipeline minutes, which its notice's Learn more opens.
 *  bex documents no page of its own on them; its behavior is that page's
 *  (docs/ADR030 §13). */
export const PIPELINE_MINUTES_DOCS_URL =
  "https://render.com/docs/build-pipeline#pipeline-minutes";

const LINK_CLASS = "underline underline-offset-2";

/**
 * Why Manual Build and Manual Deploy are disabled while the workspace's
 * pipeline minutes are spent (w3/108), in Render's words: "You’ve used up the
 * maximum number of pipeline minutes specified by your spend limit." with
 * "pipeline minutes" linking Workspace Settings → Build Pipeline, then "Learn
 * more." on its own line (Render's `Fk`).
 */
export function PipelineMinutesReason() {
  const { t } = useTranslations();
  // Every language names {minutes} before {learnMore}.
  const [before, between = "", after = ""] = t(
    "usage.pipelineMinutesExceeded",
    { minutes: SLOT, learnMore: SLOT },
  ).split(SLOT);
  return (
    <span className="block max-w-64 space-y-1 text-left">
      <span className="block">
        {before}
        <Link
          to="/workspace/settings"
          hash="build-pipeline"
          className={LINK_CLASS}
        >
          {t("usage.pipelineMinutesLink")}
        </Link>
        {between.trimEnd()}
      </span>
      <span className="block">
        <a
          href={PIPELINE_MINUTES_DOCS_URL}
          target="_blank"
          rel="noreferrer"
          className={LINK_CLASS}
        >
          {t("usage.pipelineMinutesLearnMore")}
        </a>
        {after}
      </span>
    </span>
  );
}
