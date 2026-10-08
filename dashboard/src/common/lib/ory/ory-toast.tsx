import { useIntl } from "react-intl";
import { XIcon } from "lucide-react";
import { toast } from "sonner";
import {
  messageTestId,
  uiTextToFormattedMessage,
  type OryToastProps,
} from "@ory/elements-react";
import { cn } from "@/common/lib/utils/utils.ts";

/**
 * Replaces Ory's `DefaultToast` (settings flow messages, e.g. "Settings
 * updated" after a recovery or password change).
 *
 * Ory's toast is styled with `.ory-elements .bg-interface-…` selectors, but
 * sonner portals it into the app-wide `<Toaster/>` — outside any
 * `.ory-elements` ancestor — so none of those rules match and it rendered as
 * an unthemed light box with near-invisible text in dark mode. This version
 * uses the app's own popover tokens, so it follows the light/dark theme like
 * every other toast. Copy and test ids mirror Ory's original.
 */
export function OryToast({ message, id }: OryToastProps) {
  const intl = useIntl();
  const title =
    message.type === "error"
      ? intl.formatMessage({
          id: "settings.messages.toast-title.error",
          defaultMessage: "Could not update settings",
        })
      : intl.formatMessage({
          id: "settings.messages.toast-title.success",
          defaultMessage: "Settings updated",
        });

  return (
    <div
      className="flex w-full flex-col gap-1 rounded-md border bg-popover px-4 py-3 text-sm text-popover-foreground shadow-lg sm:w-[356px]"
      {...messageTestId(message)}
    >
      <div className="flex items-center justify-between gap-2">
        <p
          className={cn("font-medium", {
            "text-emerald-600 dark:text-emerald-400":
              message.type === "success",
            "text-destructive": message.type === "error",
          })}
        >
          {title}
        </p>
        <button
          type="button"
          data-testid={`ory/message/${message.id}.close`}
          aria-label={intl.formatMessage({
            id: "settings.messages.toast-close",
            defaultMessage: "Close",
          })}
          className="cursor-pointer text-muted-foreground hover:text-foreground"
          onClick={() => toast.dismiss(id)}
        >
          <XIcon className="size-4" />
        </button>
      </div>
      <p className="text-muted-foreground">
        {uiTextToFormattedMessage(message, intl)}
      </p>
    </div>
  );
}
