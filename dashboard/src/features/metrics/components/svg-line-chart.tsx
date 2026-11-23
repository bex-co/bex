import { useCallback, useMemo, useState } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatAxisTime, formatTimeOfDay } from "@/common/lib/format";
import {
  axisTickValues,
  formatAxisTickLabels,
} from "@/features/metrics/lib/format";
import {
  CHART_FRAME_STYLE,
  axisLeftPad,
  axisTimeTicks,
  chartFrameStyle,
  useChartBox,
  tickLabelX,
  CHART_PAD as PAD,
  groupPointsByTime,
  frameTooltipRows,
  type SeriesInput,
  type ChartFrame,
  useEventLane,
} from "@/features/metrics/components/chart-layout";
import {
  AXIS_LABEL_CLASS,
  ChartTimeAxis,
} from "@/features/metrics/components/chart-time-axis";
import {
  ChartEventLines,
  ChartEventStrip,
  EventLaneSlot,
} from "@/features/metrics/components/chart-event-markers";
import {
  ChartInstanceLegendTrigger,
  ChartInstanceMenu,
  type ChartInstanceActions,
} from "@/features/metrics/components/chart-instance-menu";
import {
  clusterMarkers,
  type ChartEventMarker,
} from "@/features/metrics/lib/chart-events";

/** One line of a (possibly multi-series) chart — e.g. one App instance. */
export type LineSeriesInput = SeriesInput;

interface SvgLineChartProps {
  unit: string;
  /** One line per entry (e.g. one per instance); one entry = a plain chart. */
  series: LineSeriesInput[];
  /** Dashed horizontal reference (e.g. a resource limit), drawn when within ~2x the data max. */
  referenceValue?: number;
  /** Service events to mark on the chart (vertical line + badge strip). */
  markers?: ChartEventMarker[];
  /** The service the markers' badges link into; required to render markers. */
  markersServiceId?: string;
  /** Per-instance chart actions (w4/717: Render's click-a-series action card). */
  instanceActions?: ChartInstanceActions;
}

interface OpenInstanceMenu {
  instanceId: string;
  color: string;
  xPercent: number;
  yPx: number;
  flipLeft: boolean;
}

/**
 * A time-series line chart: 2px lines, hairline gridlines, and a crosshair+
 * tooltip that tracks the pointer (or ArrowLeft/ArrowRight when focused) and
 * snaps to the nearest sample time. Accepts one or many series; x is mapped by
 * timestamp (not index) so series that start late — a pod created mid-range —
 * still align. The area wash only draws for a single series (stacked washes
 * turn to mud).
 */
export function SvgLineChart({
  unit,
  series,
  referenceValue,
  markers,
  markersServiceId,
  instanceActions,
}: SvgLineChartProps) {
  const { t, i18n } = useTranslations();
  const [activeFrame, setActiveFrame] = useState<number | null>(null);
  const [activeSeries, setActiveSeries] = useState<string | null>(null);
  const [openMenu, setOpenMenu] = useState<OpenInstanceMenu | null>(null);
  // The SVG draws in its own measured pixels (see useChartBox).
  const { ref: svgRef, box } = useChartBox<SVGSVGElement>();
  const { width: WIDTH, height: HEIGHT } = box;

  const geo = useMemo(
    () => buildGeometry(series, referenceValue, unit, WIDTH, HEIGHT),
    [series, referenceValue, unit, WIDTH, HEIGHT],
  );

  const eventClusters = useMemo(
    () =>
      geo && markers?.length && markersServiceId
        ? clusterMarkers(markers, geo.xForT, geo.left, WIDTH - PAD.right)
        : [],
    [geo, markers, markersServiceId, WIDTH],
  );

  // The time axis only once measured: its labels read the browser's
  // timezone, so the server render must not carry them. Memoized, so the
  // crosshair's re-renders do not re-lay the axis.
  const xTicks = useMemo(
    () =>
      geo && box.measured
        ? axisTimeTicks(geo.minT, geo.maxT, geo.xForT, WIDTH, (t, step) =>
            formatAxisTime(t, geo.maxT - geo.minT, i18n.language, step),
          )
        : [],
    [geo, box.measured, WIDTH, i18n.language],
  );

  const closeMenu = useCallback(() => setOpenMenu(null), []);

  if (!geo) {
    return <EmptyChart />;
  }

  const { frames, xForT, yFor, yTicks, paths, referenceY, left } = geo;

  function nearestPointForPointer(
    clientX: number,
    clientY: number,
    svg: SVGSVGElement,
  ): { frameIndex: number; rowIndex: number } {
    const rect = svg.getBoundingClientRect();
    const hasRect = rect.width > 0 && rect.height > 0;
    const relX = hasRect ? ((clientX - rect.left) / rect.width) * WIDTH : 0;
    const relY = hasRect ? ((clientY - rect.top) / rect.height) * HEIGHT : 0;
    let bestFrame = 0;
    if (hasRect) {
      let bestDist = Infinity;
      frames.forEach((f, i) => {
        const d = Math.abs(xForT(f.time) - relX);
        if (d < bestDist) {
          bestDist = d;
          bestFrame = i;
        }
      });
    }
    const frame = frames[bestFrame];
    let bestRow = 0;
    if (hasRect && frame.rows.length > 1) {
      let bestYDist = Infinity;
      frame.rows.forEach((row, i) => {
        const d = Math.abs(yFor(row.value) - relY);
        if (d < bestYDist) {
          bestYDist = d;
          bestRow = i;
        }
      });
    }
    return { frameIndex: bestFrame, rowIndex: bestRow };
  }

  function openMenuForRow(
    frame: ChartFrame,
    row: ChartFrame["rows"][number] | undefined,
  ) {
    if (!instanceActions || !row?.label) return;
    const x = xForT(frame.time);
    const y = yFor(row.value);
    const xPercent = WIDTH > 0 ? (x / WIDTH) * 100 : 0;
    setOpenMenu({
      instanceId: row.label,
      color: row.color,
      xPercent,
      yPx: Math.min(HEIGHT - 16, Math.max(4, y + 8)),
      flipLeft: x > WIDTH / 2,
    });
  }

  const active = activeFrame != null ? frames[activeFrame] : null;
  const visibleMenu =
    openMenu && series.some((s) => s.label === openMenu.instanceId)
      ? openMenu
      : null;
  const activeLabeledRow =
    active && instanceActions
      ? (active.rows.find((r) => r.label === activeSeries) ??
        active.rows.find((r) => Boolean(r.label)))
      : undefined;
  const hasInstanceSeries = Boolean(
    instanceActions && series.some((s) => Boolean(s.label) && s.points.length > 0),
  );
  const tooltipHint =
    activeLabeledRow?.label && instanceActions
      ? t("metrics.viewActionsFor", {
          instance:
            instanceActions.formatLabel?.(activeLabeledRow.label) ??
            activeLabeledRow.label,
        })
      : undefined;

  return (
    <div className="relative">
      {markersServiceId && (
        <ChartEventStrip
          clusters={eventClusters}
          serviceId={markersServiceId}
          width={WIDTH}
        />
      )}
      <svg
        ref={svgRef}
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className={hasInstanceSeries ? "block w-full cursor-pointer" : "block w-full"}
        style={CHART_FRAME_STYLE}
        role="img"
        aria-label={t("metrics.lineChartLabel", { count: frames.length })}
        tabIndex={0}
        onPointerMove={(e) => {
          if (visibleMenu) return;
          const { frameIndex, rowIndex } = nearestPointForPointer(
            e.clientX,
            e.clientY,
            e.currentTarget,
          );
          setActiveFrame(frameIndex);
          setActiveSeries(frames[frameIndex]?.rows[rowIndex]?.label ?? null);
        }}
        onPointerLeave={() => {
          if (visibleMenu) return;
          setActiveFrame(null);
          setActiveSeries(null);
        }}
        onClick={
          hasInstanceSeries
            ? (e) => {
                const useExisting =
                  e.clientX === 0 && e.clientY === 0 && active != null;
                if (useExisting) {
                  openMenuForRow(active, activeLabeledRow);
                  return;
                }
                const { frameIndex, rowIndex } = nearestPointForPointer(
                  e.clientX,
                  e.clientY,
                  e.currentTarget,
                );
                const frame = frames[frameIndex];
                if (!frame) return;
                const row =
                  frame.rows[rowIndex] ??
                  frame.rows.find((r) => Boolean(r.label));
                setActiveFrame(frameIndex);
                setActiveSeries(row?.label ?? null);
                openMenuForRow(frame, row);
              }
            : undefined
        }
        onKeyDown={(e) => {
          if (e.key === "ArrowRight") {
            const next = Math.min(frames.length - 1, (activeFrame ?? -1) + 1);
            setActiveFrame(next);
            const rows = frames[next]?.rows ?? [];
            if (!rows.some((r) => r.label === activeSeries)) {
              setActiveSeries(rows.find((r) => Boolean(r.label))?.label ?? null);
            }
          } else if (e.key === "ArrowLeft") {
            const next = Math.max(0, (activeFrame ?? frames.length) - 1);
            setActiveFrame(next);
            const rows = frames[next]?.rows ?? [];
            if (!rows.some((r) => r.label === activeSeries)) {
              setActiveSeries(rows.find((r) => Boolean(r.label))?.label ?? null);
            }
          } else if (
            hasInstanceSeries &&
            active &&
            (e.key === "ArrowDown" || e.key === "ArrowUp")
          ) {
            const labeled = active.rows
              .map((r) => r.label)
              .filter((l): l is string => Boolean(l));
            if (labeled.length > 1) {
              e.preventDefault();
              const cur = labeled.indexOf(activeLabeledRow?.label ?? "");
              const delta = e.key === "ArrowDown" ? 1 : -1;
              const next = (cur + delta + labeled.length) % labeled.length;
              setActiveSeries(labeled[next]);
            }
          } else if (
            hasInstanceSeries &&
            (e.key === "Enter" || e.key === " ")
          ) {
            const frame = active ?? frames[frames.length - 1];
            if (!frame) return;
            const row =
              (active ? activeLabeledRow : undefined) ??
              frame.rows.find((r) => Boolean(r.label));
            if (row?.label) {
              e.preventDefault();
              setActiveFrame(activeFrame ?? frames.length - 1);
              setActiveSeries(row.label);
              openMenuForRow(frame, row);
            }
          } else if (e.key === "Escape") {
            if (visibleMenu) {
              setOpenMenu(null);
            } else {
              setActiveFrame(null);
              setActiveSeries(null);
            }
          }
        }}
      >
        {/* Gridlines: hairline, recessive, one step off the surface. Keyed by
            position, not value — a flat series (all zero) repeats the same
            value across every tick. */}
        {yTicks.map((t, i) => (
          <g key={i}>
            <line
              x1={left}
              x2={WIDTH - PAD.right}
              y1={t.y}
              y2={t.y}
              stroke="var(--border)"
              strokeWidth={1}
            />
            <text
              x={tickLabelX(left)}
              y={t.y}
              textAnchor="end"
              dominantBaseline="middle"
              className={AXIS_LABEL_CLASS}
            >
              {t.label}
            </text>
          </g>
        ))}

        <ChartTimeAxis ticks={xTicks} height={HEIGHT} />

        {/* Reference (e.g. limit): dashed, recessive, above gridlines. */}
        {referenceY != null && (
          <line
            x1={left}
            x2={WIDTH - PAD.right}
            y1={referenceY}
            y2={referenceY}
            stroke="var(--muted-foreground)"
            strokeWidth={1}
            strokeDasharray="4 3"
          />
        )}

        {/* Event markers' vertical lines, under the data like the reference. */}
        {eventClusters.length > 0 && (
          <ChartEventLines clusters={eventClusters} height={HEIGHT} />
        )}

        {/* Area wash (~10% opacity, single series only) then the 2px lines. */}
        {paths.map((p, i) => (
          <g key={i}>
            {p.areaPath && (
              <path d={p.areaPath} fill={p.color} opacity={0.1} stroke="none" />
            )}
            <path
              d={p.linePath}
              fill="none"
              stroke={p.color}
              strokeWidth={2}
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          </g>
        ))}

        {/* Crosshair + active markers, one per series with a sample here. */}
        {active && (
          <g>
            <line
              x1={xForT(active.time)}
              x2={xForT(active.time)}
              y1={PAD.top}
              y2={HEIGHT - PAD.bottom}
              stroke="var(--border)"
              strokeWidth={1}
            />
            {active.rows.map((row, i) => (
              <circle
                key={i}
                cx={xForT(active.time)}
                cy={yFor(row.value)}
                r={
                  activeLabeledRow?.label && row.label === activeLabeledRow.label
                    ? 5
                    : 4
                }
                fill={row.color}
                stroke="var(--background)"
                strokeWidth={2}
              />
            ))}
          </g>
        )}
      </svg>

      {visibleMenu && instanceActions ? (
        <ChartInstanceMenu
          instanceId={visibleMenu.instanceId}
          color={visibleMenu.color}
          xPercent={visibleMenu.xPercent}
          yPx={visibleMenu.yPx}
          flipLeft={visibleMenu.flipLeft}
          actions={instanceActions}
          onClose={closeMenu}
        />
      ) : active ? (
        <ChartTooltip
          label={formatTimeOfDay(active.time, i18n.language)}
          rows={frameTooltipRows(active, unit)}
          activeName={activeLabeledRow?.label}
          hint={tooltipHint}
        />
      ) : null}
    </div>
  );
}

export interface TooltipRow {
  color: string;
  value: string;
  /** Series name; omitted on single-series charts. */
  name?: string;
}

export function ChartTooltip({
  label,
  rows,
  activeName,
  hint,
}: {
  label: string;
  rows: TooltipRow[];
  /** Highlighted series when hovering a multi-series interactive chart. */
  activeName?: string;
  /** Render's footer hint ("View actions for <instance>"). */
  hint?: string;
}) {
  return (
    <div className="pointer-events-none absolute top-1 right-1 rounded-md border bg-popover px-2 py-1 text-xs shadow-sm">
      {rows.map((row, i) => {
        const dimmed =
          rows.length > 1 &&
          activeName != null &&
          row.name != null &&
          row.name !== activeName;
        return (
          <div
            key={i}
            className={
              dimmed
                ? "flex items-center gap-1.5 opacity-60"
                : "flex items-center gap-1.5"
            }
          >
            <span
              className="inline-block h-0.5 w-3 rounded-full"
              style={{ backgroundColor: row.color }}
            />
            {row.name && (
              <span className="max-w-40 truncate text-muted-foreground">
                {row.name}
              </span>
            )}
            <span className="font-semibold text-popover-foreground">
              {row.value}
            </span>
          </div>
        );
      })}
      <div className="text-muted-foreground">{label}</div>
      {hint ? (
        <div className="mt-0.5 border-t pt-0.5 text-muted-foreground">
          {hint}
        </div>
      ) : null}
    </div>
  );
}

export function EmptyChart({
  height,
  message,
}: {
  /** Overrides the chart's responsive ratio with a fixed height. */
  height?: number;
  /** Defaults to the localized "no data in range" copy when omitted. */
  message?: string;
}) {
  const { t } = useTranslations();
  const lane = useEventLane();
  return (
    <>
      {lane && <EventLaneSlot />}
      <div
        className="flex w-full items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground"
        style={chartFrameStyle(height)}
      >
        {message ?? t("metrics.chartEmpty")}
      </div>
    </>
  );
}

/**
 * A legend row for a multi-series chart: one colored dot + label per named
 * series. Hides itself for single-series (or unlabeled) charts. When
 * `instanceActions` is provided, each entry opens the instance action menu.
 */
export function ChartLegend({
  entries,
  instanceActions,
}: {
  entries: { color: string; label?: string }[];
  instanceActions?: ChartInstanceActions;
}) {
  const named = entries.filter(
    (e): e is { color: string; label: string } => !!e.label,
  );
  if (named.length < 2) return null;
  return (
    <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1">
      {named.map((e, i) =>
        instanceActions ? (
          <ChartInstanceLegendTrigger
            key={`${e.label}-${i}`}
            instanceId={e.label}
            color={e.color}
            actions={instanceActions}
          />
        ) : (
          <span
            key={i}
            className="flex items-center gap-1.5 text-xs text-muted-foreground"
          >
            <span
              className="inline-block h-2 w-2 rounded-full"
              style={{ backgroundColor: e.color }}
            />
            {e.label}
          </span>
        ),
      )}
    </div>
  );
}

interface LineGeometry {
  frames: ChartFrame[];
  xForT: (t: number) => number;
  yFor: (v: number) => number;
  yTicks: { value: number; label: string; y: number }[];
  paths: { linePath: string; areaPath: string; color: string }[];
  /** y of the reference line, when one is drawn. */
  referenceY: number | null;
  /** The plot's left gutter, sized for its tick labels. */
  left: number;
  /** The time domain the x axis spans. */
  minT: number;
  maxT: number;
}

/**
 * Time-mapped geometry shared by every series: x from the union time domain,
 * y from the union value domain (plus the reference value when it's close
 * enough to the data to draw without flattening it). Null when there is
 * nothing to draw.
 */
function buildGeometry(
  seriesList: LineSeriesInput[],
  referenceValue: number | undefined,
  unit: string,
  WIDTH: number,
  HEIGHT: number,
): LineGeometry | null {
  const drawn = seriesList.filter((s) => s.points.length > 0);
  const allPoints = drawn.flatMap((s) => s.points);
  if (allPoints.length === 0) {
    return null;
  }

  const times = allPoints.map((p) => Date.parse(p.timestamp));
  const minT = Math.min(...times);
  const maxT = Math.max(...times);
  const values = allPoints.map((p) => p.value);
  const dataMax = Math.max(...values, 0);
  const minVal = Math.min(...values, 0);
  // Include a nearby reference (limit) in the domain; a far-away one (usage
  // at 1% of limit) would flatten the data to a hairline, so it's dropped.
  const showReference =
    referenceValue != null && referenceValue <= dataMax * 2.5;
  const maxVal =
    showReference && referenceValue != null
      ? Math.max(dataMax, referenceValue)
      : dataMax;
  const span = maxVal - minVal || 1;

  // Round ticks: min, midpoint, max — enough for a compact PoC axis. Their
  // formatted widths size the left gutter every x below is measured from.
  const tickValues = axisTickValues(unit, minVal, maxVal);
  // One shared precision per axis so differing ticks never share a label
  // (w4/381); the final strings size the gutter.
  const tickLabels = formatAxisTickLabels(unit, tickValues);
  const left = axisLeftPad(tickLabels);
  const innerWidth = WIDTH - left - PAD.right;
  const innerHeight = HEIGHT - PAD.top - PAD.bottom;

  const xForT = (t: number) =>
    maxT === minT
      ? left + innerWidth / 2
      : left + ((t - minT) / (maxT - minT)) * innerWidth;
  const yFor = (v: number) =>
    PAD.top + innerHeight - ((v - minVal) / span) * innerHeight;

  const paths = drawn.map((s) => {
    const line = s.points
      .map(
        (p, i) =>
          `${i === 0 ? "M" : "L"}${xForT(Date.parse(p.timestamp))},${yFor(p.value)}`,
      )
      .join(" ");
    const first = s.points[0];
    const last = s.points[s.points.length - 1];
    const area =
      drawn.length === 1
        ? `${line} L${xForT(Date.parse(last.timestamp))},${PAD.top + innerHeight} L${xForT(Date.parse(first.timestamp))},${PAD.top + innerHeight} Z`
        : "";
    return { linePath: line, areaPath: area, color: s.color };
  });

  const yTicks = tickValues.map((v, i) => ({
    value: v,
    label: tickLabels[i],
    y: yFor(v),
  }));

  return {
    frames: groupPointsByTime(drawn),
    xForT,
    yFor,
    yTicks,
    paths,
    referenceY:
      showReference && referenceValue != null ? yFor(referenceValue) : null,
    left,
    minT,
    maxT,
  };
}
