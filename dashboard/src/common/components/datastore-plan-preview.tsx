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
 * Three inert tier slots with a ready plan card's anatomy — p-3 border, a
 * 24px name line and a 20px specs line (70px) in the same 12px-gap grid — so
 * a datastore plan region keeps its geometry while the catalog loads (w4/m208).
 */
export function PlanTierSlots() {
  return (
    <div className={CREATE_PLAN_CARD_GRID_CLASS} aria-hidden="true">
      {Array.from({ length: 3 }, (_, i) => (
        <div key={i} className="rounded-lg border border-border p-3">
          <Skeleton className="h-6 w-20" />
          <Skeleton className="h-5 w-32" />
        </div>
      ))}
    </div>
  );
}

const PLAN_COPY = {
  postgres: {
    title: "databases.planTitle",
    description: "databases.planDescription",
    cancel: "databases.planPickerCancel",
    save: "databases.planPickerSave",
  },
  keyvalue: {
    title: "keyvalue.planTitle",
    description: "keyvalue.planDescription",
    cancel: "keyvalue.planPickerCancel",
    save: "keyvalue.planPickerSave",
  },
} as const;

/**
 * A datastore plan card before its resource or code is ready: the ready
 * card's real title, description, tier slots and bordered footer, inert, so
 * every loading stage keeps the ready bounds (w4/m208).
 */
export function DatastorePlanPreview({ kind }: { kind: keyof typeof PLAN_COPY }) {
  const { t } = useTranslations();
  const copy = PLAN_COPY[kind];
  return (
    <Card inert aria-busy="true">
      <CardHeader>
        <CardTitle>{t(copy.title)}</CardTitle>
        <CardDescription>{t(copy.description)}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <PlanTierSlots />
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
