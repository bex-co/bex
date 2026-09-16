import { Fragment, type ReactNode } from "react";

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatInstantDetails } from "@/common/lib/format";
import { cn } from "@/common/lib/utils/utils";

interface InstantTooltipProps {
  /** RFC3339/ISO instant behind the visible text. */
  value: string;
  /** The visible rendering — usually elapsed time, "Deployed 2 hours ago". */
  children: ReactNode;
  className?: string;
}

/**
 * Render's timestamp affordance: hovering (or focusing) a relative time
 * reveals the exact instant three ways — the viewer's local clock, UTC, and
 * the Unix timestamp — each selectable so it can be copied into a log query or
 * a support thread. Hoverable content (Radix's default) is what lets the
 * pointer travel into the tooltip to select text without closing it.
 *
 * The trigger is a `<time>` carrying the machine-readable instant. The visible
 * text is the caller's and may be elapsed-time text, which drifts between the
 * SSR pass and hydration — the same divergence `RelativeAge` suppresses
 * (w6/m102). The detail rows only mount when the tooltip opens, so the local
 * reading is never computed into the hydrated markup (the w6/m107 rule).
 */
export function InstantTooltip({
  value,
  children,
  className,
}: InstantTooltipProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <time
          dateTime={value}
          tabIndex={0}
          className={cn(
            "cursor-default rounded-sm underline decoration-dotted underline-offset-2 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
            className,
          )}
          suppressHydrationWarning
        >
          {children}
        </time>
      </TooltipTrigger>
      <TooltipContent className="select-text px-3 py-2 text-left">
        <InstantDetails value={value} />
      </TooltipContent>
    </Tooltip>
  );
}

function InstantDetails({ value }: { value: string }) {
  const { t } = useTranslations();
  const details = formatInstantDetails(value);
  if (!details) return null;
  const rows: [string, string][] = [
    [t("common.timeLocal"), details.local],
    [t("common.timeUtc"), details.utc],
    [t("common.timeUnix"), details.unix],
  ];
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 whitespace-nowrap">
      {rows.map(([label, text]) => (
        <Fragment key={label}>
          <dt className="text-background/70">{label}</dt>
          {/* select-all: one click selects the whole value for copying. */}
          <dd className="tabular-nums select-all">{text}</dd>
        </Fragment>
      ))}
    </dl>
  );
}
