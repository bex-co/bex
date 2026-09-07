import { InvitationFrame } from "@/features/invites/invitation-frame";
import { useRouterState, useSearch } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  CardSkeleton,
  FieldRowSkeleton,
  FieldRowsSkeleton,
  MetadataListSkeleton,
  PlanPickerGridSkeleton,
} from "@/common/components/detail-skeletons";
import { CREATE_PLAN_CARD_GRID_CLASS } from "@/common/components/plan-card-grid";
import {
  SECTION_NAVIGATION_ITEMS_CLASS,
  SECTION_NAVIGATION_STICKY_CLASS,
} from "@/common/components/section-navigation";
import { cn } from "@/common/lib/utils/utils";
import {
  DEFAULT_SERVICE_TYPE,
  SERVICE_TYPES,
} from "@/features/services/lib/create-context";
import { LogPanelSkeleton } from "@/features/logs/components/log-panel-skeleton";

/**
 * Route-pending frames for m79. Each exported component names one real page
 * geometry; sharing happens only below that level, through small primitives
 * such as cards and table rows. `data-skeleton-region` is deliberately part of
 * the DOM contract: focused tests and the slow-navigation sweep use it to prove
 * that a pending frame still reserves every always-present ready-state region.
 */

function PendingFrame({
  route,
  className,
  children,
}: {
  route: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      aria-hidden="true"
      data-skeleton-frame="true"
      data-route-skeleton={route}
      className={cn(
        "group animate-pulse motion-reduce:animate-none",
        className,
      )}
    >
      {children}
    </div>
  );
}

function Region({
  name,
  className,
  children,
}: {
  name: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div data-skeleton-region={name} className={className}>
      {children}
    </div>
  );
}

function PageHeaderSkeleton({
  description = false,
  back = false,
  actions = 1,
}: {
  description?: boolean;
  back?: boolean;
  actions?: number;
}) {
  return (
    <Region
      name="page-header"
      className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6"
    >
      <div className="flex min-w-0 items-center gap-3">
        {back ? <Skeleton className="size-9 shrink-0" /> : null}
        <div className="space-y-1.5">
          <Skeleton className="h-6 w-48 max-w-[55vw]" />
          {description ? <Skeleton className="h-4 w-72 max-w-[65vw]" /> : null}
        </div>
      </div>
      {actions > 0 ? (
        <div className="flex items-center gap-2">
          {Array.from({ length: actions }, (_, index) => (
            <Skeleton key={index} className="h-9 w-24" />
          ))}
        </div>
      ) : null}
    </Region>
  );
}

function TableRowsSkeleton({
  rows = 3,
  columns = 4,
}: {
  rows?: number;
  columns?: number;
}) {
  return (
    <div className="overflow-hidden rounded-md border">
      <div
        className="grid gap-4 border-b bg-muted/30 px-4 py-3"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
      >
        {Array.from({ length: columns }, (_, index) => (
          <Skeleton key={index} className="h-3 w-16 max-w-full" />
        ))}
      </div>
      {Array.from({ length: rows }, (_, row) => (
        <div
          key={row}
          className="grid min-h-14 items-center gap-4 border-b px-4 py-3 last:border-b-0"
          style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
        >
          {Array.from({ length: columns }, (_, column) => (
            <Skeleton
              key={column}
              className={
                column === 0 ? "h-4 w-28 max-w-full" : "h-4 w-16 max-w-full"
              }
            />
          ))}
        </div>
      ))}
    </div>
  );
}

function TableCardSkeleton({
  rows = 3,
  columns = 4,
  controls = false,
  description = false,
  action = true,
}: {
  rows?: number;
  columns?: number;
  controls?: boolean;
  description?: boolean;
  action?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="space-y-3">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-2">
            <Skeleton className="h-5 w-40" />
            {description ? <Skeleton className="h-4 w-64 max-w-full" /> : null}
          </div>
          {action ? <Skeleton className="h-8 w-24" /> : null}
        </div>
        {controls ? (
          <div className="flex flex-col gap-2 sm:flex-row">
            <Skeleton className="h-9 min-w-0 flex-1" />
            <Skeleton className="h-9 w-full sm:w-44" />
          </div>
        ) : null}
      </CardHeader>
      <CardContent>
        <TableRowsSkeleton rows={rows} columns={columns} />
      </CardContent>
    </Card>
  );
}

function VerticalNavigationSkeleton({ count = 5 }: { count?: number }) {
  return (
    <Region name="section-navigation" className="space-y-2">
      {Array.from({ length: count }, (_, index) => (
        <Skeleton
          key={index}
          className={index === 0 ? "h-8 w-full" : "h-8 w-5/6"}
        />
      ))}
    </Region>
  );
}

function ResponsiveSectionNavigationSkeleton({ count }: { count: number }) {
  return (
    <Region
      name="section-navigation"
      className={cn(SECTION_NAVIGATION_STICKY_CLASS, "min-w-0")}
    >
      <div className={SECTION_NAVIGATION_ITEMS_CLASS}>
        {Array.from({ length: count }, (_, index) => (
          <Skeleton key={index} className="h-8 w-28 shrink-0 lg:w-full" />
        ))}
      </div>
    </Region>
  );
}

function FormCardSkeleton({
  fields,
  tallField = false,
}: {
  fields: number;
  tallField?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="space-y-2">
        <Skeleton className="h-6 w-52 max-w-full" />
        <Skeleton className="h-4 w-80 max-w-full" />
      </CardHeader>
      <CardContent className="space-y-6">
        <FieldRowsSkeleton rows={fields} />
        {tallField ? <Skeleton className="h-32 w-full" /> : null}
        <div className="flex justify-end gap-2 border-t pt-4">
          <Skeleton className="h-9 w-20" />
          <Skeleton className="h-9 w-24" />
        </div>
      </CardContent>
    </Card>
  );
}

function FormActionsSkeleton({ border = false }: { border?: boolean }) {
  return (
    <div className={cn("flex justify-end gap-2", border && "border-t pt-4")}>
      <Skeleton className="h-9 w-20" />
      <Skeleton className="h-9 w-32" />
    </div>
  );
}

function SourcePickerSkeleton({ tabs }: { tabs: 2 | 3 }) {
  return (
    <div className="space-y-3">
      <Skeleton className="h-4 w-32" />
      <div
        className="grid gap-1 rounded-lg bg-muted p-1"
        style={{ gridTemplateColumns: `repeat(${tabs}, minmax(0, 1fr))` }}
      >
        {Array.from({ length: tabs }, (_, index) => (
          <Skeleton key={index} className="h-8 w-full" />
        ))}
      </div>
      <div className="space-y-2">
        <div
          className="flex flex-col gap-2 sm:flex-row"
          data-skeleton-region="source-toolbar"
        >
          <Skeleton className="h-9 min-w-0 flex-1" />
          <Skeleton className="h-9 w-full sm:w-48" />
        </div>
        <div
          className="divide-y rounded-md border"
          data-skeleton-region="source-repositories"
        >
          {Array.from({ length: 3 }, (_, index) => (
            <div key={index} className="flex items-center gap-3 p-3">
              <Skeleton className="size-4 shrink-0 rounded-full" />
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-3 w-12" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function CreatePlanGridSkeleton({ count }: { count: number }) {
  return (
    <div className={CREATE_PLAN_CARD_GRID_CLASS}>
      {Array.from({ length: count }, (_, index) => (
        <Skeleton key={index} className="h-[4.25rem] w-full rounded-lg" />
      ))}
    </div>
  );
}

function ProjectEnvironmentFieldsSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton className="h-4 w-44" />
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
      </div>
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-4/5" />
    </div>
  );
}

function ToggleRowSkeleton() {
  return (
    <div className="flex items-start justify-between gap-4 rounded-md border p-3">
      <div className="flex-1 space-y-2">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-4 w-4/5" />
      </div>
      <Skeleton className="h-6 w-11 shrink-0 rounded-full" />
    </div>
  );
}

function EditorSummarySkeleton({
  description = false,
}: {
  description?: boolean;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-8 w-28" />
      </div>
      {description ? <Skeleton className="h-4 w-4/5" /> : null}
    </div>
  );
}

function WebhookEventPickerSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton className="h-4 w-28" />
      <div className="overflow-hidden rounded-md border">
        <div className="space-y-2 border-b p-2">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-7 w-32" />
        </div>
        <div className="h-72 space-y-2 p-3">
          {Array.from({ length: 8 }, (_, index) => (
            <div key={index} className="flex items-center gap-3">
              <Skeleton className="size-4 shrink-0" />
              <Skeleton className={index % 3 === 0 ? "h-4 w-40" : "h-4 w-56"} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function SettingsFormCardSkeleton({
  fields,
  description = true,
  className,
}: {
  fields: number;
  description?: boolean;
  className?: string;
}) {
  return (
    <Card className={className}>
      <CardHeader className="space-y-2">
        <Skeleton className="h-5 w-40" />
        {description ? <Skeleton className="h-4 w-4/5" /> : null}
      </CardHeader>
      <CardContent className="space-y-6">
        {Array.from({ length: fields }, (_, index) => (
          <FieldRowSkeleton key={index} hint />
        ))}
      </CardContent>
    </Card>
  );
}

function SettingsActionCardSkeleton({
  description = true,
  className,
}: {
  description?: boolean;
  className?: string;
}) {
  return (
    <Card className={className}>
      <CardHeader className="space-y-2">
        <Skeleton className="h-5 w-36" />
        {description ? <Skeleton className="h-4 w-4/5" /> : null}
      </CardHeader>
      <CardContent>
        <Skeleton className="h-9 w-28" />
      </CardContent>
    </Card>
  );
}

function EditableMetadataSkeleton({ rows }: { rows: number }) {
  return (
    <Card>
      <CardHeader>
        <Skeleton className="h-5 w-32" />
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {Array.from({ length: rows }, (_, index) => (
            <div
              key={index}
              className="flex min-h-[2.875rem] items-center justify-between gap-4 border-b pb-2 sm:min-h-[3.75rem]"
            >
              <Skeleton className="h-4 w-24" />
              <Skeleton className={index % 3 === 0 ? "h-9 w-28" : "h-4 w-20"} />
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function ReservedCardSkeleton({
  height,
  className,
}: {
  height?: number;
  className?: string;
}) {
  return (
    <Card className={className} style={height ? { height } : undefined}>
      <CardHeader>
        <Skeleton className="h-5 w-40" />
      </CardHeader>
      <CardContent className="min-h-0 flex-1">
        <Skeleton className="h-full min-h-10 w-full" />
      </CardContent>
    </Card>
  );
}

export function OverviewPageSkeleton() {
  return (
    <PendingFrame route="overview" className="flex-1 overflow-auto p-4 sm:p-6">
      <div className="w-full space-y-6">
        <Region
          name="page-header"
          className="flex items-center justify-between gap-2"
        >
          <Skeleton className="h-6 w-36" />
          <Skeleton className="h-8 w-24" />
        </Region>
        <Region name="projects" className="space-y-3">
          <Skeleton className="h-4 w-24" />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }, (_, index) => (
              <Skeleton key={index} className="h-32 w-full rounded-xl" />
            ))}
          </div>
        </Region>
        <Region name="resources" className="space-y-3">
          <Skeleton className="h-4 w-28" />
          <TableRowsSkeleton rows={2} columns={4} />
        </Region>
      </div>
    </PendingFrame>
  );
}

export function BillingPageSkeleton() {
  return (
    <PendingFrame route="billing" className="flex-1 overflow-auto p-4 sm:p-6">
      <div className="mx-auto grid w-full max-w-5xl items-start gap-8 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
        <div className="min-w-0 space-y-6">
          <Region
            name="page-header"
            className="flex flex-wrap items-start justify-between gap-4"
          >
            <div className="space-y-2">
              <Skeleton className="h-6 w-36" />
              <Skeleton className="h-4 w-72 max-w-full" />
            </div>
            <Skeleton className="h-8 w-44" />
          </Region>
          <Region name="mobile-navigation" className="border-y py-2 lg:hidden">
            <div className="flex gap-2 overflow-hidden">
              {Array.from({ length: 4 }, (_, index) => (
                <Skeleton key={index} className="h-8 w-24 shrink-0" />
              ))}
            </div>
          </Region>
          <Region name="plan">
            <CardSkeleton rows={3} />
          </Region>
          <Region name="payment-method">
            <CardSkeleton rows={3} />
          </Region>
          <Region name="included-usage">
            <CardSkeleton rows={4} />
          </Region>
          <Region name="charges">
            <CardSkeleton rows={7} />
          </Region>
          <Region name="credit-balance">
            <CardSkeleton rows={3} />
          </Region>
          <Region name="invoice-history">
            <TableCardSkeleton
              rows={3}
              columns={3}
              description
              action={false}
            />
          </Region>
        </div>
        <div className="sticky top-6 hidden lg:block">
          <VerticalNavigationSkeleton count={6} />
        </div>
      </div>
    </PendingFrame>
  );
}

export function NotificationsPageSkeleton() {
  return (
    <PendingFrame
      route="notifications"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <div className="mx-auto w-full max-w-2xl space-y-6">
        <Region name="email-notifications">
          <CardSkeleton rows={3} />
        </Region>
        <Region name="push-notifications">
          <CardSkeleton rows={5} />
        </Region>
        <Region name="web-push">
          <CardSkeleton rows={2} />
        </Region>
      </div>
    </PendingFrame>
  );
}

export function BlueprintsListPageSkeleton() {
  return (
    <PendingFrame
      route="blueprints-list"
      className="flex min-h-0 flex-1 flex-col"
    >
      <PageHeaderSkeleton actions={2} />
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <Region name="blueprints-table" className="mx-auto w-full max-w-4xl">
          <BlueprintsTableSkeleton />
        </Region>
      </div>
    </PendingFrame>
  );
}

export function BlueprintsTableSkeleton() {
  return <TableCardSkeleton rows={3} columns={5} action={false} />;
}

export function BlueprintDetailContentSkeleton() {
  return (
    <PendingFrame route="blueprint-detail" className="space-y-6">
      <Region name="metadata">
        <MetadataListSkeleton rows={6} />
      </Region>
      <Region name="resources">
        <TableCardSkeleton rows={2} columns={2} action={false} />
      </Region>
      <Region name="sync-history">
        <TableCardSkeleton rows={3} columns={5} action={false} />
      </Region>
      <Region name="manifest">
        <CardSkeleton rows={8} />
      </Region>
      <Region name="validation">
        <CardSkeleton rows={3} />
      </Region>
    </PendingFrame>
  );
}

export function BlueprintCreatePageSkeleton() {
  return (
    <PendingFrame
      route="blueprint-create"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <Region name="blueprint-form" className="mx-auto w-full max-w-2xl">
        <Card className="min-h-[902px] sm:min-h-0">
          <CardHeader className="space-y-2" data-skeleton-region="form-header">
            <Skeleton className="h-6 w-56 max-w-full" />
            <div className="space-y-1">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-4/5" />
            </div>
          </CardHeader>
          <CardContent className="space-y-6">
            <Region name="source-picker">
              <SourcePickerSkeleton tabs={2} />
            </Region>
            <Region name="settings" className="space-y-4">
              <FieldRowSkeleton />
              <FieldRowSkeleton hint />
              <FieldRowSkeleton hint hintLines={2} />
            </Region>
            <Region name="preview" className="space-y-3">
              <Skeleton className="h-6 w-56" />
              <Skeleton className="h-5 w-full" />
            </Region>
            <Region name="actions">
              <FormActionsSkeleton />
            </Region>
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function EnvGroupsListPageSkeleton() {
  return (
    <PendingFrame
      route="env-groups-list"
      className="flex min-h-0 flex-1 flex-col"
    >
      <PageHeaderSkeleton description />
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <EnvGroupsContentSkeleton />
      </div>
    </PendingFrame>
  );
}

export function EnvGroupsContentSkeleton() {
  return (
    <div className="mx-auto w-full max-w-5xl space-y-4">
      <Region name="search">
        <Skeleton className="h-9 w-full max-w-md" />
      </Region>
      <Region name="env-groups-table">
        <TableRowsSkeleton rows={3} columns={5} />
      </Region>
    </div>
  );
}

export function EnvGroupDetailContentSkeleton() {
  return (
    <PendingFrame route="env-group-detail" className="space-y-6">
      <Region name="metadata">
        <MetadataListSkeleton rows={5} />
      </Region>
      <Region name="environment-editor">
        <TableCardSkeleton rows={3} columns={3} description />
      </Region>
      <Region name="linked-services">
        <TableCardSkeleton rows={3} columns={3} description />
      </Region>
    </PendingFrame>
  );
}

export function WebhooksListPageSkeleton() {
  return (
    <PendingFrame
      route="webhooks-list"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <Region name="webhooks-card" className="mx-auto w-full max-w-2xl">
        <TableCardSkeleton rows={3} columns={5} controls description />
      </Region>
    </PendingFrame>
  );
}

export function WebhookCreatePageSkeleton() {
  return (
    <PendingFrame
      route="webhook-create"
      className="flex min-h-0 flex-1 flex-col"
    >
      <PageHeaderSkeleton back actions={0} />
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <Region name="webhook-form" className="mx-auto w-full max-w-2xl">
          <Card className="min-h-[973px] sm:min-h-0">
            <CardHeader
              className="space-y-2"
              data-skeleton-region="form-header"
            >
              <Skeleton className="h-6 w-40" />
              <Skeleton className="h-4 w-full" />
            </CardHeader>
            <CardContent className="space-y-6">
              <Region name="identity" className="space-y-6">
                <FieldRowSkeleton hint />
                <FieldRowSkeleton hint />
              </Region>
              <Region name="events" className="space-y-2">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-4 w-4/5" />
                <WebhookEventPickerSkeleton />
              </Region>
              <Region name="status">
                <ToggleRowSkeleton />
              </Region>
              <Region name="actions">
                <div className="flex justify-end">
                  <Skeleton className="h-9 w-32" />
                </div>
              </Region>
            </CardContent>
          </Card>
        </Region>
      </div>
    </PendingFrame>
  );
}

export function ProjectOverviewPageSkeleton() {
  return (
    <PendingFrame
      route="project-overview"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <div className="w-full space-y-6">
        <Region name="project-header" className="space-y-2">
          <Skeleton className="h-3 w-20" />
          <div className="flex items-center gap-2">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="size-8" />
          </div>
        </Region>
        <ProjectEnvironmentsSkeleton />
      </div>
    </PendingFrame>
  );
}

export function ProjectEnvironmentsSkeleton() {
  return (
    <Region name="environments" className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="space-y-2">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-9 w-64" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-8 w-28" />
          <Skeleton className="h-8 w-32" />
        </div>
      </div>
      <ProjectEnvironmentCardSkeleton />
    </Region>
  );
}

export function ProjectEnvironmentCardSkeleton() {
  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-3">
        <div className="flex items-center gap-2">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-16" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-8 w-24" />
          <Skeleton className="size-8" />
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-2 sm:flex-row">
          <Skeleton className="h-9 min-w-0 flex-1" />
          <Skeleton className="h-9 w-full sm:w-36" />
        </div>
        <TableRowsSkeleton rows={3} columns={5} />
      </CardContent>
    </Card>
  );
}

export function ProjectSettingsPageSkeleton() {
  return (
    <PendingFrame
      route="project-settings"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <div className="w-full max-w-2xl space-y-6">
        <Region name="page-header">
          <Skeleton className="h-6 w-44" />
        </Region>
        <Region name="project-name">
          <CardSkeleton rows={2} />
        </Region>
        <Region name="danger-zone">
          <CardSkeleton rows={2} />
        </Region>
      </div>
    </PendingFrame>
  );
}

/** Parent project loading chooses the child shape the URL will reveal. */
export function ProjectRouteSkeleton() {
  const routeId = useRouterState({
    select: (state) => state.matches.at(-1)?.routeId,
  });
  return (
    <PendingFrame
      route="project-active-child"
      className="flex min-h-0 flex-1 flex-col"
    >
      <Region name="active-child" className="contents">
        {routeId === "/project/$projectId/settings" ? (
          <ProjectSettingsPageSkeleton />
        ) : (
          <ProjectOverviewPageSkeleton />
        )}
      </Region>
    </PendingFrame>
  );
}

export function KeyValueCreatePageSkeleton() {
  return (
    <PendingFrame
      route="keyvalue-create"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <Region name="keyvalue-form" className="mx-auto w-full max-w-2xl">
        <Card className="min-h-[1190px] sm:min-h-0">
          <CardHeader className="space-y-2" data-skeleton-region="form-header">
            <Skeleton className="h-6 w-44" />
            <Skeleton className="h-4 w-80 max-w-full" />
          </CardHeader>
          <CardContent className="space-y-6">
            <Region name="name">
              <FieldRowSkeleton />
            </Region>
            <Region name="plan-picker" className="space-y-2">
              <Skeleton className="h-4 w-28" />
              <CreatePlanGridSkeleton count={3} />
            </Region>
            <Region name="version">
              <FieldRowSkeleton />
            </Region>
            <Region name="memory-policy">
              <FieldRowSkeleton hint />
            </Region>
            <Region name="persistence">
              <FieldRowSkeleton hint />
            </Region>
            <Region name="project-environment">
              <ProjectEnvironmentFieldsSkeleton />
            </Region>
            <Region name="public-access">
              <ToggleRowSkeleton />
            </Region>
            <Region name="actions">
              <FormActionsSkeleton />
            </Region>
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function WorkspaceCreatePageSkeleton() {
  return (
    <PendingFrame
      route="workspace-create"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <div className="mx-auto w-full max-w-4xl space-y-8">
        <Region name="page-header" className="space-y-2">
          <Skeleton className="h-8 w-56" />
          <Skeleton className="h-5 w-96 max-w-full" />
        </Region>
        <Region name="workspace-details" className="space-y-4">
          <Skeleton className="h-7 w-44" />
          <FieldRowsSkeleton rows={2} />
        </Region>
        <Region name="workspace-plans" className="space-y-2">
          <Skeleton className="h-4 w-24" />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            {Array.from({ length: 4 }, (_, index) => (
              <Skeleton key={index} className="h-56 w-full rounded-xl" />
            ))}
          </div>
        </Region>
        <Region name="payment-method" className="space-y-3">
          <div className="space-y-2">
            <Skeleton className="h-7 w-40" />
            <Skeleton className="h-4 w-[32rem] max-w-full" />
          </div>
          <Skeleton className="h-24 w-full rounded-lg" />
        </Region>
        <Region name="actions" className="flex justify-end gap-2 border-t pt-4">
          <Skeleton className="h-9 w-20" />
          <Skeleton className="h-9 w-24" />
        </Region>
      </div>
    </PendingFrame>
  );
}

export function WorkspaceSettingsPageSkeleton() {
  return (
    <PendingFrame
      route="workspace-settings"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <div className="mx-auto grid w-full max-w-4xl items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
        <ResponsiveSectionNavigationSkeleton count={3} />
        <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
          <Region name="page-header" className="space-y-2">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-80 max-w-full" />
          </Region>
          <Region name="general">
            <CardSkeleton rows={5} />
          </Region>
          <Region name="team">
            <TableCardSkeleton rows={3} columns={4} />
          </Region>
          <Region name="danger-zone">
            <CardSkeleton rows={2} />
          </Region>
        </div>
      </div>
    </PendingFrame>
  );
}

export function AccountSettingsPageSkeleton() {
  return (
    <PendingFrame
      route="account-settings"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <div className="mx-auto grid w-full max-w-6xl items-start gap-8 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
        <div className="min-w-0">
          <Region name="page-header" className="space-y-2">
            <Skeleton className="h-6 w-44" />
            <Skeleton className="h-4 w-80 max-w-full" />
          </Region>
          <Region
            name="mobile-navigation"
            className="mt-6 border-y py-2 lg:hidden"
          >
            <div className="flex gap-2 overflow-hidden">
              {Array.from({ length: 5 }, (_, index) => (
                <Skeleton key={index} className="h-8 w-24 shrink-0" />
              ))}
            </div>
          </Region>
          <div className="mt-8 space-y-10 lg:mt-10">
            {[
              { name: "account", count: 2 },
              { name: "integrations", count: 2 },
              { name: "access", count: 2 },
              { name: "security", count: 3 },
              { name: "danger-zone", count: 1 },
            ].map(({ name, count }) => (
              <Region key={name} name={name} className="space-y-4">
                <div className="space-y-2">
                  <Skeleton className="h-5 w-48" />
                  <Skeleton className="h-4 w-72 max-w-full" />
                </div>
                {Array.from({ length: count }, (_, index) => (
                  <CardSkeleton key={index} rows={3} />
                ))}
              </Region>
            ))}
          </div>
        </div>
        <div className="sticky top-6 hidden lg:block">
          <VerticalNavigationSkeleton count={5} />
        </div>
      </div>
    </PendingFrame>
  );
}

function AuthFeatureRailSkeleton() {
  return (
    <Region
      name="feature-rail"
      className="hidden flex-1 items-center justify-center bg-muted/30 px-4 py-12 sm:px-6 lg:flex lg:px-8"
    >
      <div className="w-full max-w-md space-y-8">
        {Array.from({ length: 3 }, (_, index) => (
          <div key={index} className="space-y-4">
            <Skeleton className="size-16 rounded-full" />
            <div className="space-y-2">
              <Skeleton className="h-6 w-48" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-4/5" />
            </div>
          </div>
        ))}
      </div>
    </Region>
  );
}

export function AuthWidgetSkeleton({ fields = 2 }: { fields?: number }) {
  return (
    <Region
      name="auth-widget"
      className="grid gap-6 rounded-xl border bg-card p-6 shadow-sm sm:p-8"
    >
      <div className="space-y-2">
        <Skeleton className="h-7 w-32" />
        <Skeleton className="h-5 w-4/5" />
      </div>
      <div className="grid gap-6">
        {Array.from({ length: fields }, (_, index) => (
          <div key={index} className="space-y-1.5">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-9 w-full" />
          </div>
        ))}
        <Skeleton className="h-9 w-full" />
      </div>
      <Skeleton className="h-4 w-3/5" />
    </Region>
  );
}

function AuthRouteSkeleton({
  route,
  featureRail = false,
  children,
}: {
  route: string;
  featureRail?: boolean;
  children: React.ReactNode;
}) {
  return (
    <PendingFrame
      route={route}
      className="relative flex min-h-screen bg-background"
    >
      <Region name="language-action" className="absolute right-4 top-4 z-10">
        <Skeleton className="h-9 w-24" />
      </Region>
      <div className="flex flex-1 items-center justify-center px-6 py-12 lg:px-8">
        <div className="w-full max-w-[30rem] space-y-8">
          <Region name="page-header" className="space-y-2">
            <Skeleton className="h-9 w-64 max-w-full" />
            <Skeleton className="h-5 w-96 max-w-full" />
          </Region>
          {children}
        </div>
      </div>
      {featureRail ? <AuthFeatureRailSkeleton /> : null}
    </PendingFrame>
  );
}

export function LoginRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-login" featureRail>
      <AuthWidgetSkeleton fields={2} />
    </AuthRouteSkeleton>
  );
}

export function RegistrationRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-registration" featureRail>
      <AuthWidgetSkeleton fields={3} />
    </AuthRouteSkeleton>
  );
}

export function RecoveryRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-recovery">
      <AuthWidgetSkeleton fields={2} />
    </AuthRouteSkeleton>
  );
}

export function VerificationRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-verification">
      <AuthWidgetSkeleton fields={2} />
    </AuthRouteSkeleton>
  );
}

export function LogoutRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-logout">
      <Region name="logout-card">
        <Card>
          <CardContent className="flex flex-col items-center gap-6 py-8">
            <Skeleton className="size-28 rounded-full" />
            <div className="flex gap-3">
              <Skeleton className="h-9 w-24" />
              <Skeleton className="h-9 w-24" />
            </div>
          </CardContent>
        </Card>
      </Region>
    </AuthRouteSkeleton>
  );
}

export function AccountDeletedRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="account-deleted">
      <Region name="status-card">
        <Card>
          <CardContent className="space-y-6 p-6 sm:p-8">
            <div className="flex items-start gap-3">
              <Skeleton className="size-6 shrink-0 rounded-full" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-4/5" />
              </div>
            </div>
            <Skeleton className="h-9 w-32" />
          </CardContent>
        </Card>
      </Region>
    </AuthRouteSkeleton>
  );
}

/** `/setup/payment` — the sign-up payment wall: hero + one card (title row,
 *  description, the workspace line, the Checkout button, the hosted note, and
 *  the self-host/sign-out footer), matching PaymentSetupPage's ready state. */
export function PaymentSetupRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="payment-setup">
      <Region name="payment-setup-card">
        <Card>
          <CardHeader className="space-y-2">
            <Skeleton className="h-5 w-48" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-11/12" />
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-3">
              <Skeleton className="h-5 w-2/3" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-4 w-4/5" />
            </div>
            <div className="space-y-2 border-t pt-4">
              <Skeleton className="h-4 w-full" />
              <div className="flex gap-4">
                <Skeleton className="h-4 w-36" />
                <Skeleton className="h-4 w-16" />
              </div>
            </div>
          </CardContent>
        </Card>
      </Region>
    </AuthRouteSkeleton>
  );
}

export function ConsentRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="oauth-consent">
      <Region name="consent-card">
        <FormCardSkeleton fields={3} tallField />
      </Region>
    </AuthRouteSkeleton>
  );
}

export function DeviceConfirmRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="oauth-device-confirm">
      <Region name="device-card">
        <FormCardSkeleton fields={1} tallField />
      </Region>
    </AuthRouteSkeleton>
  );
}

export function DeviceSuccessRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="oauth-device-success">
      <Region name="terminal" className="space-y-6">
        <div className="overflow-hidden rounded-xl border bg-zinc-950">
          <div className="flex h-9 items-center gap-2 border-b border-zinc-800 px-4">
            {Array.from({ length: 3 }, (_, index) => (
              <Skeleton
                key={index}
                className="size-2.5 rounded-full bg-zinc-700"
              />
            ))}
          </div>
          <div className="space-y-2 px-4 py-4">
            {Array.from({ length: 4 }, (_, index) => (
              <Skeleton key={index} className="h-4 w-3/4 bg-zinc-800" />
            ))}
          </div>
        </div>
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-8 w-32" />
      </Region>
    </AuthRouteSkeleton>
  );
}

export function InviteRouteSkeleton({
  authenticated = true,
}: { authenticated?: boolean } = {}) {
  return (
    <InvitationFrame>
      <div
        data-route-skeleton="invite"
        aria-busy="true"
        className="flex flex-1 flex-col gap-6"
      >
        <div className="space-y-3">
          {authenticated && <Skeleton className="h-5 w-40" />}
          <Skeleton className="h-8 w-4/5" />
          <Skeleton className="h-5 w-full" />
          <Skeleton className="h-5 w-4/5" />
          {authenticated && <Skeleton className="h-5 w-3/5" />}
        </div>
        <div className="mt-auto space-y-3">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-9 w-full" />
          {!authenticated && <Skeleton className="h-9 w-full" />}
        </div>
      </div>
    </InvitationFrame>
  );
}

export function ServiceCreatePageSkeleton() {
  const { type } = useSearch({ from: "/services/new" });
  const serviceType = type ?? DEFAULT_SERVICE_TYPE;
  const staticSite = serviceType === "static_site";
  const cron = serviceType === "cron_job";
  const noPublicUrl =
    serviceType === "private_service" || serviceType === "background_worker";
  const fieldCount = staticSite ? 5 : cron ? 7 : 6;

  return (
    <PendingFrame
      route="service-create"
      className="flex-1 overflow-auto p-4 sm:p-6"
    >
      <Region name="service-form" className="mx-auto w-full max-w-2xl">
        <Card className={staticSite ? undefined : "min-h-[2458px] sm:min-h-0"}>
          <CardHeader className="space-y-2" data-skeleton-region="form-header">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-full" />
          </CardHeader>
          <CardContent className="space-y-6">
            <Region name="service-type" className="space-y-3">
              <Skeleton className="h-4 w-28" />
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                {Array.from({ length: SERVICE_TYPES.length }, (_, index) => (
                  <Skeleton
                    key={index}
                    className="h-[4.5rem] w-full rounded-lg"
                  />
                ))}
              </div>
            </Region>
            <Region name="source-picker">
              <SourcePickerSkeleton tabs={3} />
            </Region>
            <Region name="settings" className="space-y-4">
              <Skeleton className="h-5 w-24" />
              {Array.from({ length: fieldCount }, (_, index) => (
                <FieldRowSkeleton
                  key={index}
                  hint={index === 2 || (staticSite && index === 4)}
                />
              ))}
              {staticSite ? (
                <Region
                  name="build-filters"
                  className="space-y-4 rounded-md border p-4"
                >
                  <div className="space-y-2">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-3 w-4/5" />
                  </div>
                  <EditorSummarySkeleton description />
                  <EditorSummarySkeleton description />
                </Region>
              ) : null}
              {noPublicUrl ? <Skeleton className="h-4 w-4/5" /> : null}
            </Region>
            {!staticSite ? (
              <Region name="plan-picker" className="space-y-2">
                <Skeleton className="h-4 w-28" />
                <CreatePlanGridSkeleton count={7} />
              </Region>
            ) : null}
            <Region name="project-environment">
              <ProjectEnvironmentFieldsSkeleton />
            </Region>
            <Region name="auto-deploy">
              <ToggleRowSkeleton />
            </Region>
            <Region name="environment-variables">
              <EditorSummarySkeleton />
            </Region>
            <Region name="secret-files">
              <EditorSummarySkeleton description />
            </Region>
            <Region name="actions">
              <FormActionsSkeleton />
            </Region>
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function DeploysListSkeleton() {
  return (
    <PendingFrame route="deploys-list">
      <Region name="deploys-table">
        <TableCardSkeleton rows={3} columns={4} controls />
      </Region>
    </PendingFrame>
  );
}

export function DeployDetailSkeleton() {
  return (
    <PendingFrame route="deploy-detail" className="space-y-6">
      <Region name="deploy-header">
        <CardSkeleton rows={3} />
      </Region>
      <Region name="deploy-timeline">
        <CardSkeleton rows={3} />
      </Region>
      <Region name="deploy-logs" className="space-y-3">
        <div className="flex gap-2">
          <Skeleton className="h-9 w-28" />
          <Skeleton className="h-9 max-w-sm flex-1" />
          <Skeleton className="size-9" />
        </div>
        <Skeleton className="h-64 w-full rounded-md" />
      </Region>
    </PendingFrame>
  );
}

export function ServiceEventsSkeleton() {
  return (
    <PendingFrame route="service-events" className="space-y-6">
      <Region name="event-feed">
        <Card className="gap-0 overflow-hidden py-0">
          <CardHeader className="border-b py-5">
            <div className="flex flex-col justify-between gap-4 sm:flex-row">
              <div className="flex gap-3">
                <Skeleton className="size-9 shrink-0" />
                <div className="space-y-2">
                  <Skeleton className="h-5 w-36" />
                  <Skeleton className="h-4 w-64 max-w-full" />
                </div>
              </div>
              <Skeleton className="h-9 w-32" />
            </div>
          </CardHeader>
          <CardContent className="space-y-5 p-5 sm:p-6">
            {Array.from({ length: 3 }, (_, index) => (
              <div key={index} className="flex items-start gap-3">
                <Skeleton className="size-9 shrink-0 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-40" />
                  <Skeleton className="h-3 w-24" />
                </div>
                <Skeleton className="h-3 w-12" />
              </div>
            ))}
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function ServiceLogsSkeleton() {
  return (
    <PendingFrame route="service-logs" className="space-y-4">
      <Region name="log-filters" className="flex flex-wrap gap-2">
        <Skeleton className="h-9 w-36" />
        <Skeleton className="h-9 min-w-40 flex-1" />
        <Skeleton className="h-9 w-32" />
        <Skeleton className="h-9 w-24" />
      </Region>
      <Skeleton className="h-3 w-64 max-w-full" />
      <Region name="log-panel">
        <LogPanelSkeleton />
      </Region>
    </PendingFrame>
  );
}

function MetricsCardSkeleton({
  sections,
  compact = false,
}: {
  sections: number;
  compact?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-3">
        <div className="space-y-2">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-64 max-w-full" />
        </div>
        <Skeleton className="h-9 w-40" />
      </CardHeader>
      <CardContent className="space-y-6">
        {Array.from({ length: sections }, (_, index) => (
          <div key={index} className="space-y-2">
            <div className="flex items-center justify-between">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-7 w-32" />
            </div>
            <Skeleton
              className={compact ? "h-[154px] w-full" : "h-[180px] w-full"}
            />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

export function ServiceMetricsSkeleton({
  staticSite = false,
}: {
  staticSite?: boolean;
}) {
  return (
    <PendingFrame
      route={staticSite ? "static-metrics" : "service-metrics"}
      className="space-y-6"
    >
      <Region
        name="metrics-filters"
        className="flex flex-wrap items-center gap-3"
      >
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-8 w-32" />
        <Skeleton className="ml-auto size-8" />
      </Region>
      {!staticSite ? (
        <Region name="application-metrics">
          <MetricsCardSkeleton sections={3} />
        </Region>
      ) : null}
      <Region name="network-metrics">
        <MetricsCardSkeleton sections={3} />
      </Region>
    </PendingFrame>
  );
}

export function ServiceEnvironmentSkeleton() {
  return (
    <PendingFrame route="service-environment" className="space-y-6">
      <Region
        name="page-header"
        className="flex flex-col justify-between gap-3 sm:flex-row"
      >
        <div className="space-y-2">
          <Skeleton className="h-6 w-40" />
          <div className="space-y-1">
            <Skeleton className="h-4 w-72 max-w-full" />
            <Skeleton className="h-5 w-3/4 sm:hidden" />
          </div>
        </div>
        <Skeleton className="h-9 w-36" />
      </Region>
      <Region name="environment-editor" className="space-y-6">
        {Array.from({ length: 2 }, (_, index) => (
          <Card key={index}>
            <CardHeader className="flex-row items-start justify-between gap-3">
              <div className="space-y-2">
                <Skeleton className="h-5 w-40" />
                <div className="space-y-1">
                  <Skeleton className="h-4 w-72 max-w-full" />
                  {index === 0 ? (
                    <Skeleton className="h-4 w-2/3 sm:hidden" />
                  ) : null}
                </div>
              </div>
              <div className="flex gap-2">
                <Skeleton className="h-8 w-20" />
                <Skeleton className="h-8 w-16" />
              </div>
            </CardHeader>
            <CardContent>
              <Skeleton className="h-[6.375rem] w-full sm:h-24" />
            </CardContent>
          </Card>
        ))}
      </Region>
      <Region name="environment-groups">
        <Card>
          <CardHeader className="space-y-2">
            <Skeleton className="h-5 w-48" />
            <div className="space-y-1">
              {Array.from({ length: 5 }, (_, index) => (
                <Skeleton
                  key={index}
                  className={`${index === 0 ? "w-4/5" : "w-full"} h-4 ${index === 0 ? "" : "sm:hidden"}`}
                />
              ))}
            </div>
          </CardHeader>
          <CardContent>
            <Skeleton className="h-[15.5rem] w-full sm:h-[11.75rem]" />
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function ServiceSettingsSkeleton({
  staticSite = false,
  sourceKind,
}: {
  staticSite?: boolean;
  sourceKind?: "repo" | "image";
}) {
  const { t } = useTranslations();
  const sections = staticSite
    ? [
        "general",
        "build",
        "static-site",
        "domains",
        "networking",
        "notifications",
        "suspend",
        "danger-zone",
      ]
    : [
        "general",
        "source",
        "build",
        "domains",
        "networking",
        "registry-credential",
        "notifications",
        "health-checks",
        "maintenance",
        "suspend",
        "danger-zone",
      ];

  return (
    <PendingFrame
      route={staticSite ? "static-settings" : "service-settings"}
      className="service-settings-layout grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10"
    >
      <ResponsiveSectionNavigationSkeleton count={sections.length} />
      <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
        <Region name="general">
          <SettingsFormCardSkeleton
            fields={staticSite ? 1 : 5}
            className={staticSite ? undefined : "min-h-[698px] sm:min-h-0"}
          />
        </Region>
        {!staticSite ? (
          <Region name="source">
            <Card>
              <CardHeader className="flex-row items-start justify-between gap-4">
                <div className="space-y-1.5">
                  <Skeleton className="h-4 w-20" />
                  <CardDescription className="relative">
                    <span className="invisible">
                      {t(
                        sourceKind === "image"
                          ? "services.sourceImageDescription"
                          : "services.sourceRepoDescription",
                      )}
                    </span>
                    <Skeleton className="absolute inset-0" />
                  </CardDescription>
                </div>
                <Skeleton className="h-8 w-full" />
              </CardHeader>
              <CardContent>
                <div
                  className="grid gap-4 text-sm sm:grid-cols-2"
                  data-skeleton-region="source-fields"
                >
                  <div className="space-y-1">
                    <Skeleton className="h-5 w-16" />
                    <Skeleton className="h-5 w-4/5" />
                  </div>
                  {sourceKind !== "image" ? (
                    <div className="space-y-1">
                      <Skeleton className="h-5 w-14" />
                      <Skeleton className="h-5 w-2/5" />
                    </div>
                  ) : null}
                </div>
              </CardContent>
            </Card>
          </Region>
        ) : null}
        <Region
          name="build"
          className={`space-y-6 ${staticSite ? "" : "min-h-[2018px] sm:min-h-0"}`}
        >
          <SettingsFormCardSkeleton fields={staticSite ? 7 : 8} />
          <SettingsFormCardSkeleton fields={staticSite ? 3 : 6} />
        </Region>
        {staticSite ? (
          <Region name="static-site">
            <SettingsFormCardSkeleton fields={2} />
          </Region>
        ) : null}
        <Region name="domains">
          <SettingsFormCardSkeleton
            fields={2}
            className={staticSite ? undefined : "min-h-[387px] sm:min-h-0"}
          />
        </Region>
        <Region name="networking">
          <SettingsFormCardSkeleton
            fields={1}
            className={staticSite ? undefined : "min-h-[302px] sm:min-h-0"}
          />
        </Region>
        {!staticSite ? (
          <Region name="registry-credential">
            <SettingsFormCardSkeleton
              fields={1}
              className="min-h-[284px] sm:min-h-0"
            />
          </Region>
        ) : null}
        <Region name="notifications">
          <SettingsFormCardSkeleton
            fields={1}
            className={staticSite ? undefined : "min-h-[246px] sm:min-h-0"}
          />
        </Region>
        {!staticSite ? (
          <>
            <Region name="health-checks">
              <SettingsFormCardSkeleton
                fields={1}
                className="min-h-[346px] sm:min-h-0"
              />
            </Region>
            <Region name="maintenance">
              <SettingsFormCardSkeleton
                fields={2}
                className="min-h-[346px] sm:min-h-0"
              />
            </Region>
          </>
        ) : null}
        <Region name="suspend">
          <SettingsActionCardSkeleton
            className={staticSite ? undefined : "min-h-[194px] sm:min-h-0"}
          />
        </Region>
        <Region name="danger-zone">
          <SettingsActionCardSkeleton
            className={staticSite ? undefined : "min-h-[154px] sm:min-h-0"}
          />
        </Region>
      </div>
    </PendingFrame>
  );
}

export function ServiceScalingSkeleton() {
  return (
    <PendingFrame route="service-scaling" className="space-y-6">
      <Region name="autoscaling">
        <CardSkeleton rows={5} />
      </Region>
      <Region name="manual-scaling">
        <CardSkeleton rows={2} />
      </Region>
      <Region name="recent-metrics">
        <MetricsCardSkeleton sections={3} />
      </Region>
    </PendingFrame>
  );
}

export function ServicePlanSkeleton() {
  return (
    <PendingFrame route="service-plan">
      <Region name="plan-picker">
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-48" />
          </CardHeader>
          <CardContent className="space-y-6">
            <PlanPickerGridSkeleton />
            <div className="flex justify-end gap-2 border-t pt-4">
              <Skeleton className="h-9 w-20" />
              <Skeleton className="h-9 w-20" />
            </div>
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function ServiceDiskSkeleton() {
  return (
    <PendingFrame route="service-disk" className="p-4 sm:p-6">
      <Region name="disk-card">
        <Card>
          <CardHeader className="space-y-1">
            <Skeleton className="h-5 w-24" />
            <Skeleton className="h-4 w-full" />
          </CardHeader>
          <CardContent className="flex flex-col items-center gap-4 py-8">
            <Skeleton className="size-10 rounded-full" />
            <div className="w-full max-w-md space-y-2">
              <Skeleton className="mx-auto h-5 w-40" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="mx-auto h-4 w-4/5" />
            </div>
            <Skeleton className="h-9 w-24" />
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function ServiceShellSkeleton() {
  return (
    <PendingFrame route="service-shell" className="space-y-6">
      <Region name="page-header" className="space-y-2">
        <Skeleton className="h-6 w-32" />
        <div className="space-y-1">
          <Skeleton className="h-4 w-72 max-w-full" />
          <Skeleton className="h-5 w-3/4 sm:hidden" />
        </div>
      </Region>
      <ServiceShellCardsSkeleton />
    </PendingFrame>
  );
}

export function ServiceShellCardsSkeleton() {
  return (
    <>
      <Region name="web-terminal">
        <Card>
          <CardHeader className="space-y-1">
            <Skeleton className="h-5 w-32" />
            <div className="space-y-1">
              <Skeleton className="h-4 w-4/5" />
              <Skeleton className="h-3 w-2/3 sm:hidden" />
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-3">
              <div className="flex h-8 items-center gap-2">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-8 w-[220px]" />
              </div>
              <div className="space-y-2">
                <Skeleton className="h-5 w-24" />
                <div className="rounded-md border bg-zinc-950 p-2">
                  <Skeleton className="h-96 w-full bg-zinc-800" />
                </div>
              </div>
            </div>
            <div className="space-y-1">
              <Skeleton className="h-4 w-4/5" />
              <Skeleton className="h-5 w-2/3 sm:hidden" />
            </div>
          </CardContent>
        </Card>
      </Region>
      <Region name="ssh-connection">
        <Card>
          <CardHeader className="space-y-1">
            <Skeleton className="h-5 w-36" />
            <div className="space-y-1">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-3 w-3/4 sm:hidden" />
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Skeleton className="h-5 w-28" />
              <Skeleton className="h-9 w-full" />
            </div>
            <Skeleton className="h-[4.375rem] w-full sm:h-4" />
            <Skeleton className="h-9 w-44" />
          </CardContent>
        </Card>
      </Region>
    </>
  );
}

export function StaticEdgeRulesSkeleton() {
  return (
    <PendingFrame route="static-edge-rules">
      <Region name="rules-editor">
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-32" />
          </CardHeader>
          <CardContent>
            <StaticEdgeRulesEditorSkeleton />
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function StaticEdgeRulesEditorSkeleton() {
  return (
    <div aria-hidden="true" className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div className="space-y-1.5">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-3 w-64 max-w-full" />
        </div>
        <Skeleton className="h-8 w-24" />
      </div>
      <TableRowsSkeleton rows={2} columns={4} />
      <div className="flex justify-end">
        <Skeleton className="h-9 w-20" />
      </div>
    </div>
  );
}

export function WebhookActivitySkeleton() {
  return (
    <PendingFrame route="webhook-activity">
      <Region name="deliveries">
        <TableCardSkeleton rows={3} columns={7} controls description />
      </Region>
    </PendingFrame>
  );
}

export function WebhookSettingsSkeleton() {
  return (
    <PendingFrame route="webhook-settings" className="space-y-6">
      <Region name="settings-general">
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-28" />
          </CardHeader>
          <CardContent className="space-y-6">
            <ToggleRowSkeleton />
            <FieldRowSkeleton hint />
            <FieldRowSkeleton hint />
            <div className="space-y-2">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-4/5" />
            </div>
          </CardContent>
        </Card>
      </Region>
      <Region name="settings-events">
        <Card>
          <CardHeader className="space-y-2">
            <Skeleton className="h-5 w-28" />
            <Skeleton className="h-4 w-4/5" />
          </CardHeader>
          <CardContent className="space-y-4">
            <WebhookEventPickerSkeleton />
            <div className="flex justify-end">
              <Skeleton className="h-9 w-28" />
            </div>
          </CardContent>
        </Card>
      </Region>
      <Region name="danger-zone">
        <Card>
          <CardHeader className="space-y-2">
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-4 w-4/5" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-9 w-32" />
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function WebhookDetailShellSkeleton() {
  const routeId = useRouterState({
    select: (state) => state.matches.at(-1)?.routeId,
  });
  const settings = routeId === "/webhook/$webhookId/settings";
  return (
    <PendingFrame
      route="webhook-active-tab"
      className="flex min-h-0 flex-1 flex-col"
    >
      <Region
        name="webhook-header"
        className="space-y-3 border-b px-4 py-4 sm:px-6"
      >
        <div className="flex items-center gap-3">
          <Skeleton className="size-9 shrink-0" />
          <div className="space-y-1.5">
            <Skeleton className="h-3 w-24" />
            <div className="flex items-center gap-2">
              <Skeleton className="h-6 w-48" />
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          </div>
        </div>
        <div className="flex flex-wrap gap-x-6 gap-y-2 pl-12">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-4 w-64 max-w-full" />
          <Skeleton className="h-4 w-40" />
        </div>
        <div className="flex flex-wrap gap-2 pl-12">
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-5 w-20 rounded-full" />
          ))}
        </div>
      </Region>
      <Region name="tabs" className="flex gap-4 border-b px-4 py-2 sm:px-6">
        <Skeleton className="h-5 w-16" />
        <Skeleton className="h-5 w-16" />
      </Region>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <Region name="active-tab" className="mx-auto w-full max-w-4xl">
          {settings ? <WebhookSettingsSkeleton /> : <WebhookActivitySkeleton />}
        </Region>
      </div>
    </PendingFrame>
  );
}

export function DatabaseOverviewSkeleton() {
  return (
    <PendingFrame
      route="database-overview"
      className="mx-auto grid w-full max-w-6xl items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10"
    >
      <ResponsiveSectionNavigationSkeleton count={10} />
      <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
        <Region name="metadata">
          <EditableMetadataSkeleton rows={10} />
        </Region>
        <Region name="connection">
          <CardSkeleton rows={2} />
        </Region>
        <Region name="sql-console">
          <ReservedCardSkeleton height={375} />
        </Region>
        <Region name="high-availability">
          <ReservedCardSkeleton height={160} />
        </Region>
        <Region name="metrics">
          <ReservedCardSkeleton height={320} />
        </Region>
        <Region name="plan">
          <ReservedCardSkeleton height={200} />
        </Region>
        <Region name="insights">
          <ReservedCardSkeleton height={360} />
        </Region>
        <Region name="recovery">
          <ReservedCardSkeleton height={240} />
        </Region>
        <Region name="access-control">
          <ReservedCardSkeleton height={240} />
        </Region>
        <Region name="danger-zone" className="flex flex-wrap gap-2">
          <Skeleton className="h-9 w-32" />
          <Skeleton className="h-9 w-36" />
        </Region>
      </div>
    </PendingFrame>
  );
}

export function DatastoreLogsSkeleton() {
  return (
    <PendingFrame
      route="datastore-logs"
      className="mx-auto w-full max-w-4xl space-y-4"
    >
      <Region name="log-filters" className="flex flex-wrap items-center gap-3">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-9 min-w-56 flex-1" />
      </Region>
      <Region name="log-lines" className="min-h-64">
        <LogPanelSkeleton rows={8} />
      </Region>
    </PendingFrame>
  );
}

export function DatastoreMetricsSkeleton({
  kind,
}: {
  kind: "database" | "keyvalue";
}) {
  return (
    <PendingFrame
      route={`${kind}-metrics`}
      className="mx-auto w-full max-w-4xl"
    >
      <Region name="datastore-metrics">
        <MetricsCardSkeleton sections={3} compact />
      </Region>
    </PendingFrame>
  );
}

export function KeyValueOverviewSkeleton() {
  return (
    <PendingFrame
      route="keyvalue-overview"
      className="mx-auto grid w-full max-w-6xl items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10"
    >
      <ResponsiveSectionNavigationSkeleton count={6} />
      <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
        <Region name="metadata">
          <EditableMetadataSkeleton rows={8} />
        </Region>
        <Region name="connection">
          <ReservedCardSkeleton className="h-[174px] sm:h-[134px]" />
        </Region>
        <Region name="networking">
          <ReservedCardSkeleton className="h-[358px] sm:h-[246px]" />
        </Region>
        <Region name="plan">
          <ReservedCardSkeleton className="h-[207px] sm:h-[167px]" />
        </Region>
        <Region name="maxmemory-policy">
          <ReservedCardSkeleton className="h-[174px] sm:h-[134px]" />
        </Region>
        <Region
          name="danger-zone"
          className="flex min-h-[5.25rem] flex-wrap gap-2 sm:min-h-9"
        >
          <Skeleton className="h-9 w-32" />
          <Skeleton className="h-9 w-36" />
        </Region>
      </div>
    </PendingFrame>
  );
}

type ServiceRouteSkeletonKind =
  | "deploy-detail"
  | "deploys"
  | "disk"
  | "environment"
  | "events"
  | "logs"
  | "metrics"
  | "plan"
  | "rules"
  | "scaling"
  | "settings"
  | "shell";

const SERVICE_ROUTE_SKELETON_BY_ID = {
  "/services/$serviceId": "deploys",
  "/services/$serviceId/": "deploys",
  "/services/$serviceId/deploys/": "deploys",
  "/services/$serviceId/deploys/$deployId": "deploy-detail",
  "/services/$serviceId/disk": "disk",
  "/services/$serviceId/env": "environment",
  "/services/$serviceId/events": "events",
  "/services/$serviceId/headers": "rules",
  "/services/$serviceId/logs": "logs",
  "/services/$serviceId/metrics": "metrics",
  "/services/$serviceId/plan": "plan",
  "/services/$serviceId/redirects": "rules",
  "/services/$serviceId/scaling": "scaling",
  "/services/$serviceId/settings": "settings",
  "/services/$serviceId/shell": "shell",
  "/static/$serviceId": "events",
  "/static/$serviceId/": "events",
  "/static/$serviceId/deploys/": "deploys",
  "/static/$serviceId/deploys/$deployId": "deploy-detail",
  "/static/$serviceId/env": "environment",
  "/static/$serviceId/events": "events",
  "/static/$serviceId/headers": "rules",
  "/static/$serviceId/metrics": "metrics",
  "/static/$serviceId/redirects": "rules",
  "/static/$serviceId/settings": "settings",
} as const satisfies Record<string, ServiceRouteSkeletonKind>;

/** Select the tab frame that a service/static parent loader is blocking. */
export function ServiceRouteContentSkeleton({
  base,
}: {
  base: "/services" | "/static";
}) {
  const routeId = useRouterState({
    select: (state) => state.matches.at(-1)?.routeId,
  });
  const kind = routeId
    ? SERVICE_ROUTE_SKELETON_BY_ID[
        routeId as keyof typeof SERVICE_ROUTE_SKELETON_BY_ID
      ]
    : undefined;

  switch (kind) {
    case "deploy-detail":
      return <DeployDetailSkeleton />;
    case "deploys":
      return <DeploysListSkeleton />;
    case "disk":
      return <ServiceDiskSkeleton />;
    case "environment":
      return <ServiceEnvironmentSkeleton />;
    case "events":
      return <ServiceEventsSkeleton />;
    case "logs":
      return <ServiceLogsSkeleton />;
    case "metrics":
      return <ServiceMetricsSkeleton staticSite={base === "/static"} />;
    case "plan":
      return <ServicePlanSkeleton />;
    case "rules":
      return <StaticEdgeRulesSkeleton />;
    case "scaling":
      return <ServiceScalingSkeleton />;
    case "settings":
      return <ServiceSettingsSkeleton staticSite={base === "/static"} />;
    case "shell":
      return <ServiceShellSkeleton />;
    default:
      return base === "/static" ? (
        <ServiceEventsSkeleton />
      ) : (
        <DeploysListSkeleton />
      );
  }
}

export function WorkspaceAliasSkeleton({
  destination,
}: {
  destination: "overview" | "billing" | "settings";
}) {
  return (
    <PendingFrame
      route="workspace-alias-destination"
      className="flex min-h-0 flex-1 flex-col"
    >
      <Region name="destination-page" className="contents">
        {destination === "billing" ? (
          <BillingPageSkeleton />
        ) : destination === "settings" ? (
          <WorkspaceSettingsPageSkeleton />
        ) : (
          <OverviewPageSkeleton />
        )}
      </Region>
    </PendingFrame>
  );
}
