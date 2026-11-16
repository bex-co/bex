import { InvitationFrame } from "@/features/invites/invitation-frame";
import { EnvGroupsTableHeader } from "@/features/env-groups/components/env-groups-table-header";
import { WebhooksTableHeader } from "@/features/webhooks/components/webhooks-table-header";
import { WebhooksPageHeader } from "@/features/webhooks/components/webhooks-panel";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import { useRouterState, useSearch } from "@tanstack/react-router";
import { Bell, Pencil, Plus, Webhook } from "lucide-react";
import { PushNotificationEvent } from "@/graphql/definitions";
import {
  pushEventKeys,
  pushEvents,
} from "@/features/notifications/lib/push-policy";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { PanelCenteredState } from "@/common/components/panel-states";
import { useWorkspace } from "@/features/workspaces/context";
import { useGettingStartedDismissal } from "@/features/onboarding/hooks/use-getting-started-dismissal";
import { useOverviewShape } from "@/features/onboarding/hooks/use-overview-shape";
import { proFeaturesPlan } from "@/features/workspaces/types";
import {
  DatastorePlanPreview,
  KEY_VALUE_PLAN_SLOTS,
  PlanTierSlots,
} from "@/common/components/datastore-plan-preview";
import { servicePlanSlotCount } from "@/features/services/lib/instance-type";
import {
  DatabaseHighAvailabilityPreview,
  DatastoreConnectionsPreview,
  DatastoreMetadataPreview,
  KeyValueNetworkingPreview,
  KeyValuePolicyPreview,
} from "@/common/components/datastore-overview-previews";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import {
  CardSkeleton,
  FieldRowSkeleton,
  FieldRowsSkeleton,
  MetadataListSkeleton,
} from "@/common/components/detail-skeletons";
import {
  SECTIONED_BODY_CLASS,
  SECTIONED_GRID_CLASS,
  SECTION_NAVIGATION_ITEMS_CLASS,
  SECTION_NAVIGATION_STICKY_CLASS,
} from "@/common/components/section-navigation";
import { cn } from "@/common/lib/utils/utils";
import {
  PageFrame,
  PageHeader,
  PageScroll,
} from "@/common/components/page-frame";
import { UsageNavigation } from "@/features/usage/components/usage-navigation";
import { WorkspaceSettingsNavigation } from "@/features/workspaces/components/workspace-settings-navigation";
import { SettingsNavigation } from "@/features/auth/pages/settings-page/settings-navigation";
import { NotificationsNavigation } from "@/features/notifications/components/notifications-navigation";
import {
  DEFAULT_SERVICE_TYPE,
  SERVICE_TYPES,
  serviceTypeCreateCopy,
} from "@/features/services/lib/create-context";
import { LogPanelSkeleton } from "@/features/logs/components/log-panel-skeleton";
import { AgentEnvironmentCardSkeleton } from "@/features/agent-environment/components/agent-environment-card-skeleton";
import { AgentSecretsCardSkeleton } from "@/features/agent-environment/components/agent-secrets-card-skeleton";
import { AgentRepoEnvironmentsCardSkeleton } from "@/features/agent-environment/components/agent-repo-environments-card-skeleton";
import { AgentSkillsCardSkeleton } from "@/features/agent-environment/components/agent-skills-card-skeleton";
import {
  LeaveWorkspaceCardSkeleton,
  TeamPanelSkeleton,
} from "@/features/team/components/team-panel-skeleton";
import { BuildPipelineCardSkeleton } from "@/features/usage/components/build-pipeline-card-skeleton";
import { WorkspaceIPRulesCardSkeleton } from "@/features/workspaces/components/workspace-ip-rules-card-skeleton";
import { WorkspaceTwoFactorCardSkeleton } from "@/features/workspaces/components/workspace-two-factor-card-skeleton";
import { OverlappingDeployPolicyCardSkeleton } from "@/features/workspaces/components/overlapping-deploy-policy-card-skeleton";
import { AgentModelCardSkeleton } from "@/features/usage/components/agent-model-card-skeleton";
import { WorkspaceNotificationsCardSkeleton } from "@/features/notifications/components/workspace-notifications-card-skeleton";
import { NotificationOverridesCardSkeleton } from "@/features/notifications/components/notification-overrides-card-skeleton";
import { RegistryCredentialsPanelSkeleton } from "@/features/registry-credentials/components/registry-credentials-panel-skeleton";
import { AuditLogPanelSkeleton } from "@/features/audit/components/audit-log-panel-skeleton";
import { CHART_FRAME_STYLE } from "@/features/metrics/components/chart-layout";
import { InstanceTypePickerCardSkeleton } from "@/features/services/components/instance-type-groups";
import {
  autoSleepEligibleType,
  planSleeps,
} from "@/features/services/lib/idle-timeout";
import {
  isCronType,
  isDockerBuild,
  isStaticSiteType,
  publiclyRoutable,
  supportsMaxShutdownDelay,
} from "@/features/services/lib/service-type";
import {
  navigableSettingsSections,
  serviceSettingsSections,
  type ServiceSettingsSectionId,
  type SettingsSectionsService,
} from "@/features/services/lib/settings-sections";

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

export function TableRowsSkeleton({
  rows = 3,
  columns = 4,
  rowClassName,
}: {
  rows?: number;
  columns?: number;
  rowClassName?: string;
}) {
  return (
    <div className="overflow-hidden rounded-md border">
      <div
        className={cn(
          "grid gap-4 border-b bg-muted/30 px-4 py-3",
          rowClassName,
        )}
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
      >
        {Array.from({ length: columns }, (_, index) => (
          <Skeleton key={index} className="h-3 w-16 max-w-full" />
        ))}
      </div>
      {Array.from({ length: rows }, (_, row) => (
        <div
          key={row}
          className={cn(
            "grid min-h-14 items-center gap-4 border-b px-4 py-3 last:border-b-0",
            rowClassName,
          )}
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

function ResponsiveSectionNavigationSkeleton({ count }: { count: number }) {
  return (
    <Region
      name="section-navigation"
      className={SECTION_NAVIGATION_STICKY_CLASS}
    >
      <div className={SECTION_NAVIGATION_ITEMS_CLASS}>
        {Array.from({ length: count }, (_, index) => (
          <Skeleton
            key={index}
            className="h-8 w-24 shrink-0 rounded-full @4xl/page:w-full @4xl/page:rounded-md"
          />
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

/** The ready EventPicker, inert. Its count line is a text-sm paragraph, so the
 * bar reserves that 20px line box, not a 16px glyph height (w4/445). */
function WebhookEventPickerSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton data-event-count className="h-5 w-28" />
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
    <Card className={cn("overflow-hidden", className)}>
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
    <Card className={cn("overflow-hidden", className)}>
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

/**
 * The Overview body of a workspace with no resources: the getting-started card
 * (unless dismissed) and the empty state with its Create action. Shared by the
 * route's pending frame and the page's own first-load state.
 */
export function OverviewEmptySkeleton() {
  const { currentWorkspaceId } = useWorkspace();
  const { dismissed } = useGettingStartedDismissal(currentWorkspaceId);
  return (
    <>
      {dismissed ? null : (
        <Region
          name="getting-started"
          className="flex flex-col gap-4 rounded-xl border bg-card p-5"
        >
          <div className="flex flex-col gap-1">
            <Skeleton className="h-4 w-36" />
            <div className="flex h-5 items-center">
              <Skeleton className="h-3.5 w-48" />
            </div>
          </div>
          <div className="flex flex-col gap-2">
            {/* The GitHub item wraps to two lines below `sm` (en). */}
            <Skeleton className="h-5 w-72 max-w-full" />
            <Skeleton className="h-10 w-72 max-w-full sm:h-5" />
            <Skeleton className="h-5 w-72 max-w-full" />
          </div>
        </Region>
      )}
      <Region name="empty-state" className="flex flex-col items-center py-8">
        <div className="flex h-6 items-center">
          <Skeleton className="h-4 w-32" />
        </div>
        <div className="flex h-10 w-full items-center justify-center sm:h-5">
          <Skeleton className="h-9 w-96 max-w-full sm:h-3.5" />
        </div>
        <Skeleton className="mt-4 h-8 w-36" />
      </Region>
    </>
  );
}

/**
 * The Overview resource list at the size this workspace last had: one card per
 * project plus the "New project" card (98px; alone, the new-project card is
 * 86px), then, when there were ungrouped resources, the borderless resource
 * table (36px header, 49px rows — 63px below `sm`, where cells wrap).
 */
export function OverviewPopulatedSkeleton({
  projects,
  rows,
}: {
  projects: number;
  rows: number;
}) {
  return (
    <>
      <Region name="projects" className="space-y-3">
        <div className="flex h-5 items-center">
          <Skeleton className="h-3.5 w-20" />
        </div>
        <div className="grid grid-cols-1 gap-4 @xl:grid-cols-2 @4xl:grid-cols-3">
          {Array.from({ length: projects }, (_, index) => (
            <Skeleton
              key={index}
              data-skeleton-item="project-card"
              className="min-h-[98px] w-full rounded-xl"
            />
          ))}
          <Skeleton
            data-skeleton-item="new-project-card"
            className="min-h-[86px] w-full rounded-xl"
          />
        </div>
      </Region>
      {rows > 0 ? (
        <Region name="resources" className="space-y-3">
          <div className="flex h-5 items-center">
            <Skeleton className="h-3.5 w-36" />
          </div>
          <div>
            <div className="flex h-9 items-center border-b">
              <Skeleton className="h-3 w-24" />
            </div>
            {Array.from({ length: rows }, (_, index) => (
              <div
                key={index}
                data-skeleton-item="resource-row"
                className="flex h-[63px] items-center border-b sm:h-[49px]"
              >
                <Skeleton className="h-4 w-40" />
              </div>
            ))}
          </div>
        </Region>
      ) : null}
    </>
  );
}

export function OverviewPageSkeleton() {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const shape = useOverviewShape(currentWorkspaceId);
  return (
    <PageFrame
      pending="overview"
      width="wide"
      header={
        <PageHeader
          title={t("projects.overviewTitle")}
          actions={
            <ReservedButton variant="default" size="sm">
              <Plus />
              {t("projects.newButton")}
            </ReservedButton>
          }
        />
      }
    >
      {shape.kind === "empty" ? (
        <OverviewEmptySkeleton />
      ) : (
        <OverviewPopulatedSkeleton
          projects={shape.projects}
          rows={shape.rows}
        />
      )}
    </PageFrame>
  );
}

export function BillingPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="billing"
      header={
        <PageHeader
          title={t("usage.pageTitle")}
          description={t("usage.pageSubtitle")}
          actions={<Skeleton className="h-8 w-44" />}
        />
      }
      navigation={<UsageNavigation />}
    >
      <Region name="plan">
        <CardSkeleton rows={3} />
      </Region>
      <Region name="payment-method">
        <CardSkeleton rows={3} />
      </Region>
      <Region name="included-usage" className="space-y-6">
        <CardSkeleton rows={4} />
        {/* Pipeline minutes (w6/m308), below the Free instance hours. */}
        <CardSkeleton rows={3} />
        {/* Included bandwidth (w6/m319), below the pipeline minutes. */}
        <CardSkeleton rows={3} />
      </Region>
      <Region name="charges">
        <CardSkeleton rows={7} />
      </Region>
      <Region name="agent-model">
        <AgentModelCardSkeleton />
      </Region>
      <Region name="credit-balance">
        <CardSkeleton rows={3} />
      </Region>
      <Region name="invoice-history">
        <TableCardSkeleton rows={3} columns={3} description action={false} />
      </Region>
    </PageFrame>
  );
}

export function NotificationsPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="notifications"
      header={
        <PageHeader
          title={t("notifications.title")}
          description={t("notifications.pageDescription")}
        />
      }
      navigation={<NotificationsNavigation />}
    >
      <Region name="email-notifications">
        <EmailNotificationsCardSkeleton />
      </Region>
      <Region name="push-notifications">
        <PushNotificationsCardSkeleton />
      </Region>
      <Region name="web-push">
        <WebPushCardSkeleton />
      </Region>
    </PageFrame>
  );
}

/** A ready switch row: label and hint beside the switch, in the row's frame. */
function SwitchRowSkeleton({ label, hint }: { label: string; hint: string }) {
  return (
    <div className="flex items-center justify-between rounded-md border p-3">
      <div className="space-y-0.5 pr-4">
        <ReservedLabel>{label}</ReservedLabel>
        <ReservedHelp>{hint}</ReservedHelp>
      </div>
      <Skeleton className="h-5 w-9 shrink-0 rounded-full" />
    </div>
  );
}

const EMAIL_NOTIFICATION_ROWS = [
  ["notifications.deployStarted", "notifications.deployStartedHint"],
  ["notifications.deploySucceeded", "notifications.deploySucceededHint"],
  ["notifications.deployFailed", "notifications.deployFailedHint"],
] as const;

/** Mirrors `NotificationSettingsPanel`: the three deploy-email switch rows. */
function EmailNotificationsCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle className="relative w-fit">
          <ReservedText>{t("notifications.emailTitle")}</ReservedText>
        </CardTitle>
        <CardDescription className="relative">
          <ReservedText>{t("notifications.description")}</ReservedText>
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {EMAIL_NOTIFICATION_ROWS.map(([label, hint]) => (
            <SwitchRowSkeleton key={label} label={t(label)} hint={t(hint)} />
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

/** A ready push fieldset's legend (a 20px `text-sm font-medium` line). */
function ReservedLegend({ children }: { children: React.ReactNode }) {
  return (
    <legend className="relative text-sm font-medium">
      <ReservedText>{children}</ReservedText>
    </legend>
  );
}

/** A ready schedule or overrides fieldset's legend and hint, one block. */
function PushFieldsetHeading({ title, hint }: { title: string; hint: string }) {
  return (
    <div>
      <ReservedLegend>{title}</ReservedLegend>
      <ReservedHelp>{hint}</ReservedHelp>
    </div>
  );
}

const PUSH_FIELD_LABELS = [
  "notifications.pushMinimumUrgency",
  "notifications.pushTimezone",
  "notifications.pushMaxDeferral",
] as const;

const PUSH_SCHEDULE_EDITORS = [
  ["notifications.pushWorkingHours", "notifications.pushWorkingHoursHint"],
  ["notifications.pushQuietHours", "notifications.pushQuietHoursHint"],
] as const;

/**
 * Events bex-api publishes as `sent: false` (its `unsentPushEvents`, w4/349):
 * the ready option carries a "not sent yet" line under the name. A copy of
 * server data the pending frame cannot fetch: when bex-api starts sending one,
 * drop it here or that row stays 16px taller than the ready option.
 */
const UNSENT_PUSH_EVENTS = new Set<PushNotificationEvent>([
  PushNotificationEvent.AgentNeedsDecision,
]);

/**
 * Mirrors `PushNotificationSettingsPanel`'s form, which is the same height
 * whether push is on or off (off disables the controls, it does not hide
 * them): the enable row, one option per selectable event in two columns, the
 * urgency/time-zone/deferral fields, the working- and quiet-hours editors
 * with no ranges, the overrides editor's add select and Save. The optional
 * banners (unavailable, dropped selections, validation) stay out.
 */
function PushNotificationsCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle className="relative w-fit">
            <ReservedText>{t("notifications.pushTitle")}</ReservedText>
          </CardTitle>
          <span className="relative rounded-full px-2 py-0.5 text-xs font-medium">
            <ReservedText>{t("notifications.bexExtension")}</ReservedText>
          </span>
        </div>
        <CardDescription className="relative">
          <ReservedText>{t("notifications.pushDescription")}</ReservedText>
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <SwitchRowSkeleton
          label={t("notifications.pushEnabled")}
          hint={t("notifications.pushEnabledHint")}
        />
        <fieldset className="space-y-2">
          <ReservedLegend>{t("notifications.pushEvents")}</ReservedLegend>
          <ReservedHelp>{t("notifications.pushEventsHint")}</ReservedHelp>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {pushEvents.map((event) => (
              <div
                key={event}
                data-push-event-skeleton
                className="flex items-start gap-2 rounded-md border p-2 text-sm"
              >
                <Skeleton className="mt-0.5 size-4 shrink-0 rounded-[4px]" />
                <span className="min-w-0 flex-1">
                  <span className="relative flex flex-wrap items-baseline justify-between gap-x-2">
                    <span className="invisible">{t(pushEventKeys[event])}</span>
                    <span className="invisible text-xs">
                      {t("notifications.pushUrgencyImportant")}
                    </span>
                    <Skeleton className="absolute inset-0" />
                  </span>
                  {UNSENT_PUSH_EVENTS.has(event) ? (
                    <span className="relative block text-xs">
                      <ReservedText>
                        {t("notifications.pushEventNotSentYet")}
                      </ReservedText>
                    </span>
                  ) : null}
                </span>
              </div>
            ))}
          </div>
        </fieldset>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {PUSH_FIELD_LABELS.map((label) => (
            <div key={label} className="space-y-2">
              <ReservedLabel>{t(label)}</ReservedLabel>
              <Skeleton className="h-9 w-full" />
            </div>
          ))}
        </div>
        {PUSH_SCHEDULE_EDITORS.map(([title, hint]) => (
          <fieldset key={title} className="space-y-3">
            <PushFieldsetHeading title={t(title)} hint={t(hint)} />
            <div>
              <ReservedButton variant="outline" size="sm">
                <Plus /> {t("notifications.pushAddRange")}
              </ReservedButton>
            </div>
          </fieldset>
        ))}
        <fieldset className="space-y-3">
          <PushFieldsetHeading
            title={t("notifications.pushOverrides")}
            hint={t("notifications.pushOverridesHint")}
          />
          <Skeleton className="h-9 w-full" />
        </fieldset>
        <div className="flex items-center justify-end gap-3">
          <ReservedButton variant="default">
            {t("notifications.pushSave")}
          </ReservedButton>
        </div>
      </CardContent>
    </Card>
  );
}

/** Mirrors `WebPushSettingsPanel` in its usual state: the status line beside
 *  the Enable button. */
function WebPushCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle className="relative w-fit">
          <ReservedText>{t("notifications.webPushTitle")}</ReservedText>
        </CardTitle>
        <CardDescription className="relative">
          <ReservedText>{t("notifications.webPushDescription")}</ReservedText>
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div className="relative text-sm">
            <ReservedText>{t("notifications.webPushOff")}</ReservedText>
          </div>
          <ReservedButton variant="default">
            <Bell className="size-4" />
            {t("notifications.webPushEnable")}
          </ReservedButton>
        </div>
      </CardContent>
    </Card>
  );
}

export function BlueprintsListPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="blueprints-list"
      header={
        <PageHeader
          title={t("blueprints.pageTitle")}
          actions={
            <>
              <ReservedButton variant="outline" size="sm">
                {t("blueprints.generateButton")}
              </ReservedButton>
              <ReservedButton variant="default" size="sm">
                <Plus />
                {t("blueprints.createButton")}
              </ReservedButton>
            </>
          }
        />
      }
    >
      <Region name="blueprints-table">
        <BlueprintsTableSkeleton />
      </Region>
    </PageFrame>
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
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="blueprint-create"
      width="form"
      header={
        <PageHeader
          title={t("blueprints.createTitle")}
          description={t("blueprints.createDescription")}
        />
      }
    >
      <Region name="blueprint-form">
        <Card>
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
    </PageFrame>
  );
}

export function EnvGroupsListPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="env-groups-list"
      header={
        <PageHeader
          title={t("envGroups.pageTitle")}
          description={t("envGroups.pageDescription")}
          actions={
            <ReservedButton variant="default" size="sm">
              <Plus />
              {t("envGroups.newButton")}
            </ReservedButton>
          }
        />
      }
    >
      <EnvGroupsContentSkeleton />
    </PageFrame>
  );
}

export function EnvGroupsContentSkeleton() {
  return (
    <div className="space-y-4">
      <Region name="search">
        <Skeleton className="h-9 w-full max-w-md" />
      </Region>
      <Region name="env-groups-table">
        {/* The ready table's own columns and scroll container (w4/m220). */}
        <div className="rounded-md border">
          <Table>
            <EnvGroupsTableHeader />
            <TableBody>
              {Array.from({ length: 3 }, (_, row) => (
                <TableRow key={row}>
                  <TableCell className="min-w-48">
                    <Skeleton className="h-5 w-32" />
                    <Skeleton className="mt-0.5 h-4 w-48" />
                  </TableCell>
                  <TableCell className="min-w-40">
                    <Skeleton className="h-5 w-24" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="ml-auto h-5 w-4" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="ml-auto h-5 w-4" />
                  </TableCell>
                  <TableCell className="whitespace-nowrap">
                    <Skeleton className="ml-auto h-5 w-28" />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
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
    <PageFrame pending="webhooks-list" header={<WebhooksPageHeader pending />}>
      <Region name="webhooks-card">
        <WebhooksCardPreview />
      </Region>
    </PageFrame>
  );
}

/**
 * The ready WebhooksPanel's own card body, inert: the search field and its
 * help, then the five-column table — so the swap moves no search or table
 * (w4/m220 a832). A workspace known to be below Pro always settles to the
 * panel's plan gate, so it previews that instead (w4/432).
 */
function WebhooksCardPreview() {
  const { t } = useTranslations();
  const { currentWorkspace } = useWorkspace();
  if (currentWorkspace && !proFeaturesPlan(currentWorkspace.plan)) {
    return (
      <Card inert aria-busy="true">
        <CardContent>
          <PanelCenteredState
            icon={<Webhook />}
            title={t("webhooks.emptyTitle")}
            body={t("webhooks.planGate")}
          />
        </CardContent>
      </Card>
    );
  }
  return (
    <Card inert aria-busy="true">
      <CardContent>
        <div className="space-y-4">
          <div className="max-w-md space-y-1.5">
            <Skeleton className="h-9 w-full" />
            <ReservedHelp className="text-xs">
              {t("webhooks.searchEndpointsHelp")}
            </ReservedHelp>
          </div>
          <Table>
            <WebhooksTableHeader />
            <TableBody>
              {Array.from({ length: 3 }, (_, row) => (
                <TableRow key={row}>
                  <TableCell>
                    <Skeleton className="h-5 w-40" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-16" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-20" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-9 rounded-full" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="ml-auto size-8" />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}

export function WebhookCreatePageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="webhook-create"
      width="form"
      header={
        <PageHeader
          leading={<Skeleton className="size-9" />}
          title={t("webhooks.newTitle")}
          description={t("webhooks.createDescription")}
        />
      }
    >
      <Region name="webhook-form">
        <Card>
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
    </PageFrame>
  );
}

export function ProjectOverviewPageSkeleton() {
  return (
    <PageScroll>
      <PendingFrame route="project-overview" className="w-full space-y-6">
        <Region name="project-header" className="space-y-2">
          <Skeleton className="h-3 w-20" />
          <div className="flex items-center gap-2">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="size-8" />
          </div>
        </Region>
        <ProjectEnvironmentsSkeleton />
      </PendingFrame>
    </PageScroll>
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

/**
 * Mirrors `ProjectSettingsPage` in read mode (w4/294): the `text-xl` heading,
 * the name card's title, wrapping description and input + Edit row, and the
 * delete card's title, description and button. Static copy is reserved with
 * its own translated text; the unresolved project name never appears.
 */
export function ProjectSettingsPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="project-settings"
      width="form"
      header={<PageHeader title={t("projects.settingsTitle")} />}
    >
      <Region name="project-name">
        <Card>
          <CardHeader>
            <CardTitle className="relative w-fit">
              <ReservedText>{t("projects.nameCardTitle")}</ReservedText>
            </CardTitle>
            <CardDescription className="relative">
              <ReservedText>{t("projects.nameCardDescription")}</ReservedText>
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              data-skeleton-region="project-name-row"
              className="flex items-center gap-2"
            >
              <Skeleton className="h-9 min-w-0 flex-1" />
              <ReservedButton variant="outline" size="sm">
                <Pencil />
                {t("projects.editNameButton")}
              </ReservedButton>
            </div>
          </CardContent>
        </Card>
      </Region>
      <Region name="danger-zone">
        <Card className="border-destructive/50">
          <CardHeader>
            <CardTitle className="relative w-fit text-destructive">
              <ReservedText>{t("projects.deleteCardTitle")}</ReservedText>
            </CardTitle>
            <CardDescription className="relative">
              <ReservedText>{t("projects.deleteCardDescription")}</ReservedText>
            </CardDescription>
          </CardHeader>
          <CardContent data-skeleton-region="project-delete-action">
            <ReservedButton variant="destructive">
              {t("projects.deleteCardButton")}
            </ReservedButton>
          </CardContent>
        </Card>
      </Region>
    </PageFrame>
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
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="keyvalue-create"
      width="form"
      header={
        <PageHeader
          title={t("keyvalue.createTitle")}
          description={t("keyvalue.createDescription")}
        />
      }
    >
      <Region name="keyvalue-form">
        <Card>
          <CardContent className="space-y-6">
            <Region name="name">
              <FieldRowSkeleton />
            </Region>
            <Region name="plan-picker" className="space-y-2">
              <Skeleton className="h-4 w-28" />
              <PlanTierSlots count={KEY_VALUE_PLAN_SLOTS} />
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
    </PageFrame>
  );
}

export function WorkspaceCreatePageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="workspace-create"
      header={
        <PageHeader
          title={t("workspaces.createTitle")}
          description={t("workspaces.createDescription")}
        />
      }
    >
      <div className="space-y-8">
        <Region name="workspace-details" className="space-y-4">
          <Skeleton className="h-7 w-44" />
          <FieldRowsSkeleton rows={3} />
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
    </PageFrame>
  );
}

export function WorkspaceSettingsPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="workspace-settings"
      header={
        <PageHeader
          title={t("workspaces.settingsTitle")}
          description={t("workspaces.settingsDescription")}
        />
      }
      navigation={<WorkspaceSettingsNavigation showDangerZone />}
    >
      <Region name="general">
        <CardSkeleton rows={5} />
      </Region>
      {/* Build Pipeline (w6/m309): billing managers only, like Audit Logs below. */}
      <Region name="build-pipeline">
        <BuildPipelineCardSkeleton />
      </Region>
      {/* Overlapping Deploy Policy (w4/653): every member reads it. */}
      <Region name="overlapping-deploy-policy">
        <OverlappingDeployPolicyCardSkeleton />
      </Region>
      {/* Inbound IP Rules (w9/m388): every member reads them. */}
      <Region name="inbound-ip-rules">
        <WorkspaceIPRulesCardSkeleton />
      </Region>
      {/* Two-factor authentication (w22/m402): every member sees the card. */}
      <Region name="two-factor">
        <WorkspaceTwoFactorCardSkeleton />
      </Region>
      <Region name="team">
        <TeamPanelSkeleton />
      </Region>
      <Region name="notifications" className="space-y-6">
        <WorkspaceNotificationsCardSkeleton />
        <NotificationOverridesCardSkeleton />
      </Region>
      <Region name="agents" className="space-y-6">
        <AgentEnvironmentCardSkeleton />
        <AgentSecretsCardSkeleton />
        <AgentRepoEnvironmentsCardSkeleton />
        <AgentSkillsCardSkeleton />
      </Region>
      <Region name="registry-credentials">
        <RegistryCredentialsPanelSkeleton />
      </Region>
      <Region name="audit-logs">
        <AuditLogPanelSkeleton />
      </Region>
      <Region name="danger-zone">
        <LeaveWorkspaceCardSkeleton />
      </Region>
    </PageFrame>
  );
}

export function AccountSettingsPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="account-settings"
      header={
        <PageHeader
          title={t("auth.settingsTitle")}
          description={t("auth.settingsSubtitle")}
        />
      }
      navigation={<SettingsNavigation />}
    >
      <div className="space-y-10">
        {[
          { name: "account", count: 2 },
          { name: "integrations", count: 1 },
          { name: "access", count: 2 },
          { name: "security", count: 2 },
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
    </PageFrame>
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
  return <AuthStatusRouteSkeleton route="account-deleted" />;
}

/** `/auth/error` — Kratos's error page (w5/161): the same single status card
 *  as `/auth/account-deleted`. */
export function AuthErrorRouteSkeleton() {
  return <AuthStatusRouteSkeleton route="auth-error" />;
}

/** One status card: an icon beside a two-line message, then one button. */
function AuthStatusRouteSkeleton({ route }: { route: string }) {
  return (
    <AuthRouteSkeleton route={route}>
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
 *  the self-host/sign-out footer), matching PaymentSetupPage's ready state. The
 *  footer's first item previews "Delete this account and self-host" — a link-
 *  styled button at text height, so the row's geometry is unchanged from when it
 *  was an anchor (w3/m163). */
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
                <Skeleton className="h-4 w-52" />
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
  const { t } = useTranslations();
  const { type } = useSearch({ from: "/services/new" });
  const serviceType = type ?? DEFAULT_SERVICE_TYPE;
  const createCopy = serviceTypeCreateCopy(serviceType);
  const staticSite = serviceType === "static_site";
  const cron = serviceType === "cron_job";
  const noPublicUrl =
    serviceType === "private_service" || serviceType === "background_worker";
  const fieldCount = staticSite ? 5 : cron ? 7 : 6;

  return (
    <PageFrame
      pending="service-create"
      width="form"
      header={
        <PageHeader
          title={t(createCopy.titleKey)}
          description={t(createCopy.descriptionKey)}
        />
      }
    >
      <Region name="service-form">
        <Card>
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
                {cron ? (
                  <p className="text-muted-foreground text-xs">
                    {t("services.createFieldPlanCronHint")}
                  </p>
                ) : null}
                <PlanTierSlots
                  count={servicePlanSlotCount(serviceType)}
                  lastWraps
                />
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
    </PageFrame>
  );
}

export function DeploysListSkeleton({
  banner,
}: {
  /** The auto-deploy-off banner the ready page shows above the list
   *  (w3/064): its own content is already known, so the pending state shows
   *  it as is and matches at every width. */
  banner?: React.ReactNode;
} = {}) {
  return (
    <PendingFrame route="deploys-list" className="space-y-4">
      {banner ? <Region name="auto-deploy-off-banner">{banner}</Region> : null}
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

/** A cron job's Runs page (w2/m360): the runs card, its title with Trigger Run
 *  beside it, over the four-column run table. */
export function ServiceCronRunsSkeleton() {
  return (
    <PendingFrame route="cron-runs">
      <Region name="cron-runs-table">
        <TableCardSkeleton rows={3} columns={4} />
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
  eventLane = false,
  controls = true,
}: {
  sections: number;
  compact?: boolean;
  /** Service charts: the empty 24px event lane above a ratio-sized plot,
   *  as the ready MetricSection draws them (w4/446). */
  eventLane?: boolean;
  /** False for a card whose ready header has no filter controls. */
  controls?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-3">
        <div className="space-y-2">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-64 max-w-full" />
        </div>
        {controls ? (
          <div className="flex flex-wrap items-center gap-2">
            <Skeleton className="h-9 w-40" />
            <Skeleton className="h-9 w-36" />
            <Skeleton className="h-9 w-40" />
          </div>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-6">
        {Array.from({ length: sections }, (_, index) => (
          <div key={index} className="space-y-2">
            <div className="flex items-center justify-between">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-7 w-32" />
            </div>
            {eventLane ? (
              <>
                <div className="h-6" />
                <Skeleton className="w-full" style={CHART_FRAME_STYLE} />
              </>
            ) : (
              <Skeleton
                className={compact ? "h-[154px] w-full" : "h-[180px] w-full"}
              />
            )}
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

export function ServiceMetricsSkeleton({
  staticSite = false,
  servesRequests = true,
}: {
  staticSite?: boolean;
  /** False for a worker, cron job or private service: its Network card is
   *  Outbound Bandwidth alone, with no request controls (w4/613). */
  servesRequests?: boolean;
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
          <MetricsCardSkeleton sections={3} eventLane />
        </Region>
      ) : null}
      <Region name="network-metrics">
        <MetricsCardSkeleton
          sections={servesRequests ? 3 : 1}
          controls={servesRequests}
          eventLane
        />
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

// A service whose Settings page is not known yet (its parent is still loading):
// the common shapes, a repo-backed web service and a static site.
const REPRESENTATIVE_SERVICE: SettingsSkeletonService = {
  type: "web_service",
  repo: "https://github.com/example/app",
  runtime: "node",
};
const REPRESENTATIVE_STATIC_SITE: SettingsSkeletonService = {
  type: "static_site",
  repo: "https://github.com/example/site",
};

/** What the Settings skeleton reads from a service. */
type SettingsSkeletonService = SettingsSectionsService & {
  region?: string | null;
  plan?: string | null;
  suspended?: boolean;
};

// Each card reserves its ready card's exact height, measured on dev-5 at
// narrow mobile (390px) and desktop (1440px) with an empty service (w5/m124),
// so the swap does not move the page. The classes are literal so Tailwind
// sees them; the skeleton rows inside are clipped to the card.
type PlainSettingsSection =
  | "static-site"
  | "domains"
  | "networking"
  | "outbound-ips"
  | "registry-credential"
  | "notifications"
  | "port"
  | "health-checks"
  | "maintenance"
  | "deploy-hook";

const SETTINGS_CARD: Record<
  PlainSettingsSection,
  { fields: number; height: string }
> = {
  "static-site": { fields: 2, height: "h-[342px] sm:h-[238px]" },
  domains: { fields: 2, height: "h-[611px] sm:h-[447px]" },
  networking: { fields: 1, height: "h-[302px] sm:h-[182px]" },
  "outbound-ips": { fields: 1, height: "h-[198px] sm:h-[138px]" },
  "registry-credential": { fields: 1, height: "h-[268px] sm:h-[232px]" },
  notifications: { fields: 1, height: "h-[246px] sm:h-[186px]" },
  port: { fields: 1, height: "h-[286px] sm:h-[206px]" },
  "health-checks": { fields: 1, height: "h-[346px] sm:h-[226px]" },
  maintenance: { fields: 2, height: "h-[346px] sm:h-[266px]" },
  "deploy-hook": { fields: 1, height: "h-[262px] sm:h-[222px]" },
};

function isPlainSettingsSection(
  section: ServiceSettingsSectionId,
): section is PlainSettingsSection {
  return section in SETTINGS_CARD;
}

/**
 * General's height: its rows are the name, the instance type, the idle timeout
 * (a free web service's select, a paid one's shorter always-on notice), the
 * max shutdown delay (web, private, worker) and Region when the install sets
 * one. Region is 128px at narrow mobile and 88px at desktop plus a 24px gap; a
 * static site never shows it.
 */
function generalHeight(service: SettingsSkeletonService): string {
  const region = Boolean(service.region) && !isStaticSiteType(service.type);
  switch (service.type) {
    case "web_service":
      if (!planSleeps(service.plan ?? null)) {
        return region ? "h-[698px] sm:h-[546px]" : "h-[546px] sm:h-[434px]";
      }
      return region ? "h-[782px] sm:h-[566px]" : "h-[630px] sm:h-[454px]";
    case "cron_job":
      return region ? "h-[498px] sm:h-[366px]" : "h-[346px] sm:h-[254px]";
    case "static_site":
      return "h-[246px] sm:h-[186px]";
    default:
      return region ? "h-[630px] sm:h-[478px]" : "h-[478px] sm:h-[366px]";
  }
}

function generalRows(service: SettingsSkeletonService): number {
  const { type } = service;
  if (isStaticSiteType(type)) return 1;
  return (
    2 +
    (autoSleepEligibleType(type) ? 1 : 0) +
    (supportsMaxShutdownDelay(service) ? 1 : 0) +
    (service.region ? 1 : 0)
  );
}

/**
 * Suspend's height: a running publicly routed service's card says what its
 * visitors will see; a suspended service's card offers Resume, which a cron job
 * words more briefly.
 */
function suspendHeight(service: SettingsSkeletonService): string {
  if (service.suspended) {
    return isCronType(service.type)
      ? "h-[134px] sm:h-[134px]"
      : "h-[154px] sm:h-[134px]";
  }
  return publiclyRoutable(service.type)
    ? "h-[194px] sm:h-[154px]"
    : "h-[174px] sm:h-[134px]";
}

function SettingsSourceCardSkeleton({ repo }: { repo: boolean }) {
  const { t } = useTranslations();
  return (
    <Card
      className={cn(
        repo ? "h-[272px] sm:h-[192px]" : "h-[232px] sm:h-[192px]",
        "overflow-hidden",
      )}
    >
      <CardHeader className="flex-row items-start justify-between gap-4">
        <div className="space-y-1.5">
          <Skeleton className="h-4 w-20" />
          <CardDescription className="relative">
            <span className="invisible">
              {t(
                repo
                  ? "services.sourceRepoDescription"
                  : "services.sourceImageDescription",
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
          {repo ? (
            <div className="space-y-1">
              <Skeleton className="h-5 w-14" />
              <Skeleton className="h-5 w-2/5" />
            </div>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}

/** The skeleton of one Settings section's card or cards. */
export function SettingsSectionSkeleton({
  section,
  service,
}: {
  section: ServiceSettingsSectionId;
  service: SettingsSkeletonService;
}) {
  const { type } = service;
  if (isPlainSettingsSection(section)) {
    const card = SETTINGS_CARD[section];
    return (
      <SettingsFormCardSkeleton fields={card.fields} className={card.height} />
    );
  }
  switch (section) {
    case "general":
      return (
        <SettingsFormCardSkeleton
          fields={generalRows(service)}
          className={generalHeight(service)}
        />
      );
    case "source":
      return <SettingsSourceCardSkeleton repo={Boolean(service.repo)} />;
    case "deploy":
      // A cron job's schedule and command, or an image's Deploy card:
      // Pre-Deploy, Docker Command and the Deploy Hook.
      return isCronType(type) ? (
        <SettingsFormCardSkeleton
          fields={2}
          className="h-[368px] sm:h-[328px]"
        />
      ) : (
        <SettingsFormCardSkeleton
          fields={3}
          className="h-[650px] sm:h-[530px]"
        />
      );
    case "runtime":
      // The Runtime select beside its rebuild note, then Save. Measured in
      // Chromium (en): the select drops below the note under sm (294px at
      // 390), sits beside it from sm (210px), and the md sidebar narrows the
      // card until xl so the note wraps once more (230px at 768 and 1024).
      return (
        <SettingsFormCardSkeleton
          fields={1}
          className="h-[294px] sm:h-[210px] md:h-[230px] xl:h-[210px]"
        />
      );
    case "build": {
      // A cron job's Build card alone, with Auto-Deploy folded in. Every other
      // build shows Build and Deploy cards; a Dockerfile build shows Dockerfile
      // Path, Docker Build Context Directory and a Docker Command where a
      // native one shows Build and Start Commands. The context row matches
      // Dockerfile Path's height at every width (w8/m128, measured in
      // Chromium at 390/640/1024/1440): +134px below sm, +114px from sm.
      if (isCronType(type)) {
        return (
          <SettingsFormCardSkeleton
            fields={7}
            className="h-[1078px] sm:h-[886px]"
          />
        );
      }
      const docker = isDockerBuild(service);
      return (
        <>
          <SettingsFormCardSkeleton
            fields={docker ? 8 : 7}
            className={
              docker ? "h-[930px] sm:h-[778px]" : "h-[816px] sm:h-[664px]"
            }
          />
          {isStaticSiteType(type) ? (
            <SettingsFormCardSkeleton
              fields={2}
              className="h-[494px] sm:h-[394px]"
            />
          ) : (
            <SettingsFormCardSkeleton
              fields={4}
              className={
                docker ? "h-[802px] sm:h-[642px]" : "h-[800px] sm:h-[640px]"
              }
            />
          )}
        </>
      );
    }
    case "edge-caching":
      // Three option cards, a note and Save/Purge. Measured in Chromium (en):
      // 520px at 390 (540px when a free plan's unavailable note wraps), 456px
      // at sm, 488px from md through lg while the sidebar narrows the card,
      // 436px at xl and 420px from 1440 where the option copy stops wrapping.
      // That breakpoint is 90rem, not 1440px: Tailwind orders min-[...] among
      // the named breakpoints only in their rem unit, so a px one lost to xl.
      return (
        <SettingsFormCardSkeleton
          fields={3}
          className={cn(
            service.plan === "free" ? "h-[540px]" : "h-[520px]",
            "sm:h-[456px] md:h-[488px] xl:h-[436px] min-[90rem]:h-[420px]",
          )}
        />
      );
    case "suspend":
      return <SettingsActionCardSkeleton className={suspendHeight(service)} />;
    case "danger-zone":
      return <SettingsActionCardSkeleton className="h-[154px] sm:h-[134px]" />;
    default: {
      const unrendered: never = section;
      return unrendered;
    }
  }
}

/**
 * A service's Settings page while it loads: one region per section of
 * serviceSettingsSections, the list the page renders and its navigation links,
 * so the three cannot drift apart (w5/m124). service is the parent route's
 * service when it has loaded; otherwise the common shape of the route's base
 * stands in (staticSite).
 */
export function ServiceSettingsSkeleton({
  service,
  staticSite = false,
}: {
  service?: SettingsSkeletonService;
  staticSite?: boolean;
}) {
  const shape =
    service ??
    (staticSite ? REPRESENTATIVE_STATIC_SITE : REPRESENTATIVE_SERVICE);
  const sections = serviceSettingsSections(shape);

  return (
    <PendingFrame
      route={
        isStaticSiteType(shape.type) ? "static-settings" : "service-settings"
      }
      className={cn("service-settings-layout", SECTIONED_GRID_CLASS)}
    >
      <ResponsiveSectionNavigationSkeleton
        count={navigableSettingsSections(sections).length}
      />
      <div className={SECTIONED_BODY_CLASS}>
        {sections.map((section) => (
          <Region
            key={section}
            name={section}
            className={section === "build" ? "space-y-6" : undefined}
          >
            <SettingsSectionSkeleton section={section} service={shape} />
          </Region>
        ))}
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

/** `serviceType` shapes the Free/Paid groups; null/omitted = still undecided. */
export function ServicePlanSkeleton({
  serviceType = null,
  currentPlan,
}: {
  serviceType?: string | null;
  /** Undefined while unknown. */
  currentPlan?: string | null;
}) {
  return (
    <PendingFrame route="service-plan">
      <Region name="plan-picker">
        <InstanceTypePickerCardSkeleton
          serviceType={serviceType}
          currentPlan={currentPlan}
        />
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
      {/* Same 40rem minimum as the ready rule tables, clipped to the card. */}
      <TableRowsSkeleton rows={2} columns={4} rowClassName="min-w-[40rem]" />
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

/** A ready `Label`'s line box (`text-sm leading-none font-medium`). */
function ReservedLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative flex w-fit text-sm leading-none font-medium">
      <ReservedText>{children}</ReservedText>
    </div>
  );
}

/** A ready muted help paragraph, wrapping as its translated copy does. A div,
 *  not a <p>: ReservedText's bar is a div, which the HTML parser moves out of
 *  a paragraph in server-rendered HTML, stretching it across the page. */
function ReservedHelp({
  children,
  className = "text-sm",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("relative text-muted-foreground", className)}>
      <ReservedText>{children}</ReservedText>
    </div>
  );
}

/**
 * Mirrors `WebhookSettingsCard` (w4/295): the plain status row (label + help
 * beside the switch), Name, Destination URL and the masked signing secret as
 * label → help → input (w3/m282) — each static string reserved with its own
 * translated copy so it wraps as the ready page does. No endpoint value shows.
 */
export function WebhookSettingsSkeleton() {
  const { t } = useTranslations();
  return (
    <PendingFrame route="webhook-settings" className="space-y-6">
      <Region name="settings-general">
        <Card>
          <CardHeader>
            <CardTitle className="relative w-fit">
              <ReservedText>{t("webhooks.settingsGeneral")}</ReservedText>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            <div
              data-skeleton-region="settings-status"
              className="flex items-start justify-between gap-4"
            >
              <div>
                <ReservedLabel>{t("webhooks.settingsStatus")}</ReservedLabel>
                <ReservedHelp>{t("webhooks.settingsStatusHelp")}</ReservedHelp>
              </div>
              <Skeleton className="h-5 w-9 shrink-0 rounded-full" />
            </div>
            {(
              [
                ["webhooks.fieldName", "webhooks.fieldNameHelp"],
                ["webhooks.fieldUrl", "webhooks.fieldUrlHelp"],
              ] as const
            ).map(([label, help]) => (
              <div
                key={label}
                data-skeleton-region="settings-field"
                className="space-y-2"
              >
                <ReservedLabel>{t(label)}</ReservedLabel>
                <ReservedHelp>{t(help)}</ReservedHelp>
                <Skeleton className="h-9 w-full" />
              </div>
            ))}
            <div data-skeleton-region="settings-secret" className="space-y-2">
              <ReservedLabel>{t("webhooks.secretLabel")}</ReservedLabel>
              <ReservedHelp>{t("webhooks.secretHelp")}</ReservedHelp>
              <Skeleton className="h-9 w-full" />
            </div>
          </CardContent>
        </Card>
      </Region>
      <Region name="settings-events">
        <Card>
          <CardHeader>
            <CardTitle className="relative w-fit">
              <ReservedText>{t("webhooks.settingsEvents")}</ReservedText>
            </CardTitle>
            <CardDescription className="relative">
              <ReservedText>{t("webhooks.fieldEventsHelp")}</ReservedText>
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <WebhookEventPickerSkeleton />
            <div className="flex justify-end">
              <ReservedButton variant="default">
                {t("webhooks.saveChanges")}
              </ReservedButton>
            </div>
          </CardContent>
        </Card>
      </Region>
      <Region name="danger-zone">
        <Card>
          <CardHeader>
            <CardTitle className="relative w-fit text-destructive">
              <ReservedText>{t("webhooks.deleteSection")}</ReservedText>
            </CardTitle>
            <CardDescription className="relative">
              <ReservedText>{t("webhooks.deleteConfirmBody")}</ReservedText>
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ReservedButton variant="destructive">
              {t("webhooks.delete")}
            </ReservedButton>
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
      <PageScroll>
        <Region name="active-tab" className="mx-auto w-full max-w-5xl">
          {settings ? <WebhookSettingsSkeleton /> : <WebhookActivitySkeleton />}
        </Region>
      </PageScroll>
    </PendingFrame>
  );
}

export function DatabaseOverviewSkeleton() {
  return (
    <PendingFrame
      route="database-overview"
      className={cn("mx-auto max-w-5xl", SECTIONED_GRID_CLASS)}
    >
      <ResponsiveSectionNavigationSkeleton count={10} />
      <div className={SECTIONED_BODY_CLASS}>
        <Region name="metadata">
          <DatastoreMetadataPreview kind="postgres" />
        </Region>
        <Region name="connection">
          <DatastoreConnectionsPreview kind="postgres" />
        </Region>
        <Region name="sql-console">
          <ReservedCardSkeleton height={375} />
        </Region>
        <Region name="high-availability">
          <DatabaseHighAvailabilityPreview />
        </Region>
        <Region name="metrics">
          <ReservedCardSkeleton height={320} />
        </Region>
        <Region name="plan">
          <DatastorePlanPreview kind="postgres" />
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
      className="mx-auto w-full max-w-5xl space-y-4"
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

const DATASTORE_METRIC_SECTIONS = {
  keyvalue: [
    "metrics.diskTitle",
    "metrics.datastoreCpuTitle",
    "metrics.memoryTitle",
    "metrics.kvConnectionsTitle",
  ],
  database: [
    "metrics.diskTitle",
    "metrics.datastoreCpuTitle",
    "metrics.memoryTitle",
    "metrics.connectionsTitle",
    "metrics.replicationLagTitle",
  ],
} as const;

/**
 * DatastoreMetricsPanel before its resource resolves (w4/m209): the panel's
 * real title, description and section headings, no filter row (the panel
 * has none), and each plot at the populated chart's responsive 600:180 ratio,
 * so nothing moves when the panel and then its series arrive.
 */
export function DatastoreMetricsSkeleton({
  kind,
}: {
  kind: "database" | "keyvalue";
}) {
  const { t } = useTranslations();
  return (
    <PendingFrame
      route={`${kind}-metrics`}
      className="mx-auto w-full max-w-5xl"
    >
      <Region name="datastore-metrics">
        <Card>
          <CardHeader>
            <CardTitle>{t("metrics.datastoreMetricsTitle")}</CardTitle>
            <CardDescription>
              {t("metrics.datastoreMetricsDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {DATASTORE_METRIC_SECTIONS[kind].map((titleKey) => (
              <div key={titleKey}>
                <div className="mb-2 flex items-center justify-between gap-2">
                  <div className="text-sm font-medium text-foreground">
                    {t(titleKey)}
                  </div>
                </div>
                <Skeleton
                  className="w-full rounded-md"
                  style={CHART_FRAME_STYLE}
                />
              </div>
            ))}
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function KeyValueOverviewSkeleton() {
  return (
    <PendingFrame
      route="keyvalue-overview"
      className={cn("mx-auto max-w-5xl", SECTIONED_GRID_CLASS)}
    >
      <ResponsiveSectionNavigationSkeleton count={7} />
      <div className={SECTIONED_BODY_CLASS}>
        <Region name="metadata">
          <DatastoreMetadataPreview kind="keyvalue" />
        </Region>
        <Region name="connection">
          <DatastoreConnectionsPreview kind="keyvalue" />
        </Region>
        <Region name="networking">
          <KeyValueNetworkingPreview />
        </Region>
        <Region name="plan">
          <DatastorePlanPreview kind="keyvalue" />
        </Region>
        <Region name="maxmemory-policy">
          <KeyValuePolicyPreview policy="maxmemory" />
        </Region>
        <Region name="persistence-mode">
          <KeyValuePolicyPreview policy="persistence" />
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
  | "cron-runs"
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
  "/services/$serviceId/builds": "deploys",
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
  "/services/$serviceId/runs": "cron-runs",
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
  serviceType = null,
}: {
  base: "/services" | "/static";
  /** The service's type when already known (cache), for type-shaped tabs. */
  serviceType?: string | null;
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
    case "cron-runs":
      return <ServiceCronRunsSkeleton />;
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
      return (
        <ServiceMetricsSkeleton
          staticSite={base === "/static"}
          servesRequests={serviceType == null || publiclyRoutable(serviceType)}
        />
      );
    case "plan":
      return <ServicePlanSkeleton serviceType={serviceType} />;
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
