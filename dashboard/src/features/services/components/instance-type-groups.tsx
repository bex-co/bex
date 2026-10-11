import type { ReactNode } from "react";
import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { offersFreeTier } from "@/features/services/lib/instance-type";
import { cn } from "@/common/lib/utils/utils.ts";

// The picker's group/card layout, shared by its ready state and its loading
// skeletons so the two cannot drift apart.

// Placeholder cards per group: the catalog's one Free tier and six paid tiers.
const FREE_TIER_PLACEHOLDERS = 1;
const PAID_TIER_PLACEHOLDERS = 6;

export function InstanceTypeGroup({
  label,
  children,
}: {
  label: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2" data-instance-type-group="">
      <div className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
        {label}
      </div>
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

function GroupSkeleton({ cards }: { cards: number }) {
  return (
    <InstanceTypeGroup label={<Bar className="h-3 w-12" />}>
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
  return (
    <div
      aria-hidden="true"
      className="space-y-4"
      data-instance-type-groups-skeleton=""
    >
      {serviceType !== null && offersFreeTier(serviceType) && (
        <GroupSkeleton cards={FREE_TIER_PLACEHOLDERS} />
      )}
      <GroupSkeleton cards={PAID_TIER_PLACEHOLDERS} />
    </div>
  );
}

/** The whole picker card — title, groups and Cancel/Save slot. */
export function InstanceTypePickerCardSkeleton({
  serviceType,
}: {
  serviceType: string | null;
}) {
  return (
    <Card>
      <CardHeader>
        <Skeleton className="h-4 w-48" />
      </CardHeader>
      <CardContent className="space-y-6">
        <InstanceTypeGroupsSkeleton serviceType={serviceType} />
        <div className="flex justify-end gap-2 border-t pt-4">
          <Skeleton className="h-9 w-20" />
          <Skeleton className="h-9 w-32" />
        </div>
      </CardContent>
    </Card>
  );
}
