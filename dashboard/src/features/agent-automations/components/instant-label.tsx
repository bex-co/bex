import { InstantTooltip } from "@/common/components/instant-tooltip";
import { RelativeAge, RelativeUntil } from "@/common/components/relative-time";

/**
 * A relative time ("in 3h", "2d") whose hover or focus reveals the exact
 * instant (local, UTC, Unix), the dashboard's one timestamp affordance.
 */
export function InstantLabel({
  value,
  direction,
}: {
  value: string | null | undefined;
  direction: "past" | "future";
}) {
  if (!value || Number.isNaN(Date.parse(value))) return <>{"—"}</>;
  const Relative = direction === "future" ? RelativeUntil : RelativeAge;
  return (
    <InstantTooltip value={value}>
      <Relative value={value} as="span" />
    </InstantTooltip>
  );
}
