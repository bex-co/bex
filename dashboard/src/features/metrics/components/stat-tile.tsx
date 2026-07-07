import { formatMetricValue } from "@/features/metrics/lib/format";

interface StatTileProps {
  label: string;
  unit: string;
  value: number | null;
  color: string;
  loading: boolean;
}

/**
 * A stat-tile for bex's resource metrics (cpu/memory/instance_count): these
 * are single-point metrics-server snapshots, not time-series (docs/
 * observability.md), so this renders the current value plainly rather than
 * faking a trend line from one point.
 */
export function StatTile({
  label,
  unit,
  value,
  color,
  loading,
}: StatTileProps) {
  return (
    <div className="rounded-md border p-4">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-1 flex items-baseline gap-2">
        <span
          className="inline-block h-2.5 w-2.5 rounded-full"
          style={{ backgroundColor: color }}
        />
        <span className="text-2xl font-semibold text-foreground">
          {value == null
            ? loading
              ? "…"
              : "—"
            : formatMetricValue(unit, value)}
        </span>
      </div>
    </div>
  );
}
