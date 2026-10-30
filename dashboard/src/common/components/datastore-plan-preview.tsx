import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { CREATE_PLAN_CARD_GRID_CLASS } from "@/common/components/plan-card-grid";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Inert tier slots with a ready plan card's anatomy — p-3 border, a 24px name
 * line and a 20px specs line (70px) in the same 12px-gap grid — so a plan
 * region keeps its geometry while the catalog loads (w4/m208). `lastWraps`
 * gives the last slot the extra name line the top compute tier wraps onto in
 * three desktop columns (Pro Ultra's 94px card), so a service create form's
 * rows match the ready grid too (w4/329).
 */
export function PlanTierSlots({
  count = 3,
  lastWraps = false,
}: {
  count?: number;
  lastWraps?: boolean;
}) {
  return (
    <div className={CREATE_PLAN_CARD_GRID_CLASS} aria-hidden="true">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="rounded-lg border border-border p-3">
          <Skeleton
            className={
              lastWraps && i === count - 1 ? "h-6 w-20 sm:h-12" : "h-6 w-20"
            }
          />
          <Skeleton className="h-5 w-32" />
        </div>
      ))}
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
const KEY_VALUE_PLAN_SLOTS = 3;

const PLAN_COPY = {
  postgres: {
    title: "databases.planTitle",
    description: "databases.planDescription",
    cancel: "databases.planPickerCancel",
    save: "databases.planPickerSave",
    slots: POSTGRES_PLAN_SLOTS,
  },
  keyvalue: {
    title: "keyvalue.planTitle",
    description: "keyvalue.planDescription",
    cancel: "keyvalue.planPickerCancel",
    save: "keyvalue.planPickerSave",
    slots: KEY_VALUE_PLAN_SLOTS,
  },
} as const;

/**
 * A datastore plan card before its resource or code is ready: the ready
 * card's real title, description, tier slots and bordered footer, inert, so
 * every loading stage keeps the ready bounds (w4/m208).
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
        <PlanTierSlots count={copy.slots} />
        <div className="flex justify-end gap-2 border-t pt-4">
          <Button variant="outline" disabled>
            {t(copy.cancel)}
          </Button>
          <Button disabled>{t(copy.save)}</Button>
        </div>
      </CardContent>
    </Card>
  );
}
