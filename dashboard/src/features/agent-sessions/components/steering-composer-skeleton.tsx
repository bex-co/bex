import { SendHorizonal } from "lucide-react";
import { buttonVariants } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The padding of `SteeringComposer`'s text box row and the layout of its
 * toolbar row. The composer and this skeleton both draw their rows with them,
 * so the pending box keeps the ready box's height.
 */
export const STEERING_COMPOSER_INPUT_CLASS = "px-3 pt-2.5 pb-1";
export const STEERING_COMPOSER_TOOLBAR_CLASS =
  "flex items-center gap-1 px-1.5 pb-1.5";

/**
 * SteeringComposer's dock while the session page loads: the bordered box with
 * the text box's row above the toolbar's (w10/056), and the hint line under
 * it. The text row and the hint carry an idle session's real copy invisibly,
 * in the ready elements' type and padding, so they wrap to the same number of
 * lines at every width and in every language. The toolbar holds one
 * placeholder per control at that control's size: Attach, Skills, the model
 * and Send.
 */
export function SteeringComposerSkeleton() {
  const { t } = useTranslations();
  // Ghost sizing carries no background, so the Skeleton's own pulse shows.
  const textControl = buttonVariants({ variant: "ghost", size: "sm" });
  return (
    <div data-skeleton-region="composer" className="bg-background shrink-0">
      <div className="mx-auto w-full max-w-3xl px-4 py-2">
        <div
          data-skeleton-region="composer-box"
          className="border-input bg-background rounded-xl border shadow-xs"
        >
          <div
            data-skeleton-region="composer-input"
            className={`${STEERING_COMPOSER_INPUT_CLASS} text-base text-transparent md:text-sm`}
          >
            {t("agentSessions.steerPlaceholderIdle")}
          </div>
          <div
            data-skeleton-region="composer-toolbar"
            className={STEERING_COMPOSER_TOOLBAR_CLASS}
          >
            <Skeleton className="size-8 shrink-0" />
            <Skeleton className="size-8 shrink-0" />
            <Skeleton className="h-8 w-[10.5rem] min-w-0" />
            <Skeleton
              className={`${textControl} ml-auto shrink-0 rounded-xl text-transparent`}
            >
              <SendHorizonal />
              {t("agentSessions.steerSubmit")}
            </Skeleton>
          </div>
        </div>
        <Skeleton
          data-skeleton-region="composer-hint"
          className="mx-1 mt-1 w-fit text-xs text-transparent"
        >
          {t("agentSessions.steerHintIdle")}
        </Skeleton>
      </div>
    </div>
  );
}
