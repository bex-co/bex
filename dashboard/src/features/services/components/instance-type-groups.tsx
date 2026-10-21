import type { ReactNode } from "react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  legacyFreeNotice,
  offersFreeTier,
} from "@/features/services/lib/instance-type";
import { cn } from "@/common/lib/utils/utils.ts";

// The picker's group/card layout, shared by its ready state and its loading
// skeletons so the two cannot drift apart.

// Placeholder cards per group: the catalog's one Free tier and six paid tiers.
const FREE_TIER_PLACEHOLDERS = 1;
const PAID_TIER_PLACEHOLDERS = 6;

export function InstanceTypeGroup({
  label,
  note,
  children,
}: {
  label: ReactNode;
  /** A line under the label: the group's outbound-SMTP rule (w8/m127). */
  note?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2" data-instance-type-group="">
      <div className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
        {label}
      </div>
      {note ? <p className="text-xs text-muted-foreground">{note}</p> : null}
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {children}
      </div>
    </div>
  );
}

/** A tier card's frame and text rows; ready cards pass text, skeletons pass bars. */
export function InstanceTypeCardBody({
  name,
  memory,
  cpu,
}: {
  name: ReactNode;
  memory: ReactNode;
  cpu: ReactNode;
}) {
  return (
    <>
      <div className="mb-2 font-semibold">{name}</div>
      <div className="space-y-0.5 text-sm text-muted-foreground">
        <div>{memory}</div>
        <div>{cpu}</div>
      </div>
    </>
  );
}

export const INSTANCE_TYPE_CARD_FRAME = "rounded-lg border p-4 text-left";

// Inline bars sit inside the real text rows, so each row keeps its line height.
function Bar({ className }: { className: string }) {
  return <Skeleton className={cn("inline-block align-middle", className)} />;
}

function InstanceTypeCardSkeleton() {
  return (
    <div className={cn(INSTANCE_TYPE_CARD_FRAME, "border-border")}>
      <InstanceTypeCardBody
        name={<Bar className="h-4 w-20" />}
        memory={<Bar className="h-3.5 w-28" />}
        cpu={<Bar className="h-3.5 w-14" />}
      />
    </div>
  );
}

function GroupSkeleton({ cards, note }: { cards: number; note: ReactNode }) {
  return (
    <InstanceTypeGroup label={<Bar className="h-3 w-12" />} note={note}>
      {Array.from({ length: cards }, (_, i) => (
        <InstanceTypeCardSkeleton key={i} />
      ))}
    </InstanceTypeGroup>
  );
}

/**
 * The picker's Free/Paid groups as an inert placeholder. The Free group is
 * reserved only for a known type `offersFreeTier` accepts; an undecided type
 * (null) never guesses Free eligibility, so it reserves only the paid ladder
 * every compute type has.
 */
export function InstanceTypeGroupsSkeleton({
  serviceType,
}: {
  serviceType: string | null;
}) {
  const { t } = useTranslations();
  // The SMTP notes are static copy, drawn here so the cards sit where the
  // ready groups put them.
  return (
    <div
      aria-hidden="true"
      className="space-y-4"
      data-instance-type-groups-skeleton=""
    >
      {serviceType !== null && offersFreeTier(serviceType) && (
        <GroupSkeleton
          cards={FREE_TIER_PLACEHOLDERS}
          note={t("services.planPickerFreeSmtp")}
        />
      )}
      <GroupSkeleton
        cards={PAID_TIER_PLACEHOLDERS}
        note={t("services.planPickerPaidSmtp")}
      />
    </div>
  );
}

/**
 * The notice above the cards for a paid-only service still on Free, or
 * untiered (legacyFreeNotice). Its copy needs no data, so the loading skeleton
 * renders it too and keeps the cards where the ready page puts them.
 */
export function LegacyFreeNotice({
  serviceType,
  currentPlan,
  id,
}: {
  serviceType: string | null;
  currentPlan: string | null | undefined;
  id?: string;
}) {
  const { t } = useTranslations();
  const notice = legacyFreeNotice(serviceType, currentPlan);
  if (!notice) return null;
  return (
    <Alert id={id} role="note">
      <AlertDescription>{t(notice)}</AlertDescription>
    </Alert>
  );
}

/** The whole picker card — title, notice, groups and Cancel/Save slot. */
export function InstanceTypePickerCardSkeleton({
  serviceType,
  currentPlan,
}: {
  serviceType: string | null;
  /** Undefined while unknown: no notice is drawn. */
  currentPlan?: string | null;
}) {
  return (
    <Card>
      <CardHeader>
        <Skeleton className="h-4 w-48" />
      </CardHeader>
      <CardContent className="space-y-6">
        <LegacyFreeNotice serviceType={serviceType} currentPlan={currentPlan} />
        <InstanceTypeGroupsSkeleton serviceType={serviceType} />
        <div className="flex justify-end gap-2 border-t pt-4">
          <Skeleton className="h-9 w-20" />
          <Skeleton className="h-9 w-32" />
        </div>
      </CardContent>
    </Card>
  );
}
