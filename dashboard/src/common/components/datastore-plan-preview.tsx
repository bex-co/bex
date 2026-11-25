import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { RadioDot } from "@/common/components/radio-dot";
import {
  COMPUTE_PLAN_LIST_CLASS,
  COMPUTE_PLAN_ROW_CLASS,
} from "@/common/components/compute-plan-list";
import {
  computePlanRowClasses,
  planListCollapses,
} from "@/common/lib/plan-list";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Inert tier rows with a ready plan row's anatomy (ComputePlanList): the list
 * frame, the row box, the radio, a name line with the specs under it on a
 * narrow list or beside it on a wide one, and the price — so a plan region
 * keeps its geometry while the catalog loads (w4/m208). `connections` adds
 * the connection-limit column every Key Value row carries (w6/m459);
 * `collapsedCount` stands for the rows a collapsed create list shows and its
 * "Show all plans" button.
 */
export function PlanTierSlots({
  count = 3,
  connections = false,
  collapsedCount,
}: {
  count?: number;
  connections?: boolean;
  collapsedCount?: number;
}) {
  const collapses = planListCollapses(count, collapsedCount);
  const rows = collapses ? (collapsedCount ?? count) : count;
  // The ready row's own layout classes, so each slot takes the row's shape.
  const row = computePlanRowClasses(connections);
  return (
    <div className={COMPUTE_PLAN_LIST_CLASS} aria-hidden="true">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className={`${COMPUTE_PLAN_ROW_CLASS} border-transparent`}>
          <RadioDot checked={false} />
          <span className={row.specs}>
            <span className={`flex h-6 items-center ${row.name}`}>
              <Skeleton className="h-4 w-24" />
            </span>
            <span className={`h-5 items-center ${row.specLine}`}>
              <span className={`flex items-center ${row.cpu}`}>
                <Skeleton className="h-3.5 w-12" />
              </span>
              <span className={`flex items-center ${row.ram}`}>
                <Skeleton className="h-3.5 w-16" />
              </span>
              {connections ? (
                <span className={`flex items-center ${row.conn}`}>
                  <Skeleton className="h-3.5 w-24" />
                </span>
              ) : null}
            </span>
          </span>
          <span className={`flex justify-end ${row.price}`}>
            <Skeleton className="h-4 w-16" />
          </span>
        </div>
      ))}
      {collapses ? (
        <div className="px-1.5 pt-1 pb-0.5">
          <Skeleton className="h-11 w-36 rounded-md sm:h-9" />
        </div>
      ) : null}
    </div>
  );
}

/**
 * How many tiers each datastore catalog serves (lego/types/tiers/tiers.yaml),
 * so the loading grid has the ready grid's rows. Postgres went from three to
 * six with the ≥1-CPU plans (w2/m171): two rows at three columns, six stacked
 * on a phone.
 */
export const POSTGRES_PLAN_SLOTS = 6;
/** One slot per Key Value catalog tier, paused ones included. */
export const KEY_VALUE_PLAN_SLOTS = 9;

const PLAN_COPY = {
  postgres: {
    title: "databases.planTitle",
    description: "databases.planDescription",
    cancel: "databases.planPickerCancel",
    save: "databases.planPickerSave",
    slots: POSTGRES_PLAN_SLOTS,
    connections: false,
    // The Storage row below the footer (Render's Plan page Storage, w2/m307),
    // then Storage Autoscaling's switch row under it (w3/m641).
    storage: {
      title: "databases.storageTitle",
      edit: "databases.storageEdit",
      autoscaling: "databases.diskAutoscalingLabel",
    },
  },
  keyvalue: {
    title: "keyvalue.planTitle",
    description: "keyvalue.planDescription",
    cancel: "keyvalue.planPickerCancel",
    save: "keyvalue.planPickerSave",
    slots: KEY_VALUE_PLAN_SLOTS,
    connections: true,
    storage: null,
  },
} as const;

/**
 * A datastore plan card before its resource or code is ready: the ready
 * card's real title, description, tier slots and bordered footer — plus
 * Postgres's Storage and Storage Autoscaling rows, their values slots — inert,
 * so every loading stage keeps the ready bounds (w4/m208).
 */
export function DatastorePlanPreview({
  kind,
}: {
  kind: keyof typeof PLAN_COPY;
}) {
  const { t } = useTranslations();
  const copy = PLAN_COPY[kind];
  return (
    <Card inert aria-busy="true">
      <CardHeader>
        <CardTitle>{t(copy.title)}</CardTitle>
        <CardDescription>{t(copy.description)}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <PlanTierSlots count={copy.slots} connections={copy.connections} />
        <div className="flex justify-end gap-2 border-t pt-4">
          <Button variant="outline" disabled>
            {t(copy.cancel)}
          </Button>
          <Button disabled>{t(copy.save)}</Button>
        </div>
        {copy.storage ? (
          <>
            <div className="flex items-center justify-between gap-4 border-t pt-4">
              <div className="min-w-0 space-y-1">
                <p className="text-sm font-medium">{t(copy.storage.title)}</p>
                {/* The size line: text-sm's 20px line box. */}
                <Skeleton className="h-5 w-12" />
              </div>
              <Button variant="outline" size="sm" disabled>
                {t(copy.storage.edit)}
              </Button>
            </div>
            <div className="flex items-center justify-between gap-4 border-t pt-4">
              <div className="min-w-0 space-y-1">
                <p className="text-sm font-medium">
                  {t(copy.storage.autoscaling)}
                </p>
                {/* The size and cap line, a slot like the Storage size. */}
                <Skeleton className="h-5 w-40" />
              </div>
              {/* The switch; its position is data. */}
              <Skeleton className="h-5 w-9 shrink-0 rounded-full" />
            </div>
          </>
        ) : null}
      </CardContent>
    </Card>
  );
}
