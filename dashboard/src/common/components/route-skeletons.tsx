import { useRef } from "react";
import { InvitationFrame } from "@/features/invites/invitation-frame";
import { CRON_MINIMUM_USD } from "@/common/lib/cron-minimum";
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
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  PanelCenteredState,
  PanelCenteredStateSkeleton,
} from "@/common/components/panel-states";
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
  ReservedOryButton,
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
  arrangeSsoProviders,
  rememberedSsoProviders,
  ssoRowColumns,
} from "@/common/lib/ory/sso-providers";
import {
  PageFrame,
  PageHeader,
  PageScroll,
} from "@/common/components/page-frame";
import { UsageNavigation } from "@/features/usage/components/usage-navigation";
import { WorkspaceSettingsNavigation } from "@/features/workspaces/components/workspace-settings-navigation";
import { SettingsNavigation } from "@/features/auth/pages/settings-page/settings-navigation";
import { SettingsSectionHeading } from "@/features/auth/pages/settings-page/settings-section";
import { AccountSettingsFlowSkeleton } from "@/features/auth/pages/settings-page/account-settings-flow-skeleton";
import { AccountDeletionCardSkeleton } from "@/features/auth/pages/settings-page/account-deletion-card-skeleton";
import { ConnectGithubCardSkeleton } from "@/features/git/components/connect-github-card-skeleton";
import { ApiKeysPanelSkeleton } from "@/features/api-keys/components/api-keys-panel-skeleton";
import { SSHKeysPanelSkeleton } from "@/features/ssh-keys/components/ssh-keys-panel-skeleton";
import { ConnectedAgentsPanelSkeleton } from "@/features/connected-agents/components/connected-agents-panel-skeleton";
import { ActiveSessionsPanelSkeleton } from "@/features/sessions/components/active-sessions-panel-skeleton";
import { NotificationsNavigation } from "@/features/notifications/components/notifications-navigation";
import {
  DEFAULT_SERVICE_TYPE,
  SERVICE_TYPES,
  serviceTypeCreateCopy,
} from "@/features/services/lib/create-context";
import { LogPanelSkeleton } from "@/features/logs/components/log-panel-skeleton";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import {
  ObservabilityViewHeader,
  ViewActionButtons,
} from "@/features/logs/components/observability-view-header";
import {
  DATASTORE_LOG_ACTIONS_CLASS,
  LIVE_TOGGLE_CLASS,
  LOG_ACTIONS_CLASS,
  LOG_FILTER_ACTIONS_CLASS,
  LOG_FILTER_BAR_CLASS,
  LOG_FILTER_TAIL_CLASS,
  LOG_ICON_BUTTON_CLASS,
  LOG_SELECT_WIDTH_CLASS,
} from "@/features/logs/components/log-toolbar-layout";
import { useOpenDocked } from "@/features/logs/hooks/use-open-docked";
import { EnvRowsSkeleton } from "@/features/services/components/env-rows-skeleton";
import { SERVICE_TAB_DESCRIPTION_OFFSET } from "@/features/services/components/service-tab-description";
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
import { WorkspaceLoginRequirementCardSkeleton } from "@/features/workspaces/components/workspace-login-requirement-card-skeleton";
import { WorkspacePrivacyCardSkeleton } from "@/features/workspaces/components/workspace-privacy-card-skeleton";
import { OverlappingDeployPolicyCardSkeleton } from "@/features/workspaces/components/overlapping-deploy-policy-card-skeleton";
import { WorkspaceDetailsCardSkeleton } from "@/features/workspaces/components/workspace-details-card-skeleton";
import { DeleteWorkspaceCardSkeleton } from "@/features/workspaces/components/delete-workspace-card-skeleton";
import {
  useWorkspaceSettingsSkeletonSections,
  type WorkspaceSettingsPendingSections,
} from "@/features/workspaces/hooks/use-workspace-settings-skeleton-sections";
import { AgentModelCardSkeleton } from "@/features/usage/components/agent-model-card-skeleton";
import {
  CreditBalanceCardSkeleton,
  IncludedUsageSkeleton,
  PaymentMethodCardSkeleton,
  PlanCardSkeleton,
} from "@/features/usage/components/billing-cards-skeleton";
import { WorkspaceNotificationsCardSkeleton } from "@/features/notifications/components/workspace-notifications-card-skeleton";
import { NotificationOverridesCardSkeleton } from "@/features/notifications/components/notification-overrides-card-skeleton";
import { RegistryCredentialsPanelSkeleton } from "@/features/registry-credentials/components/registry-credentials-panel-skeleton";
import { AuditLogPanelSkeleton } from "@/features/audit/components/audit-log-panel-skeleton";
import { DeploysListFrameSkeleton } from "@/features/deploys/components/deploys-list-frame";
import {
  EVENT_BODY_CELL,
  EVENT_FOOTER_CLASS,
  EVENT_ICON_CELL,
  EVENT_ROW_CLASS,
  EVENT_TIME_CELL,
  EVENT_TITLE_CELL,
  EVENT_TOOLBAR_CLASS,
} from "@/features/events/lib/event-row-layout";
import {
  DeployBackLinkSkeleton,
  DeployHeaderSkeleton,
  DeployLogToolbarSkeleton,
  DeployTimelineCardSkeleton,
} from "@/features/deploys/components/deploy-page-skeletons";
import {
  DEPLOY_BODY_CLASS,
  DEPLOY_PAGE_TOP_CLASS,
} from "@/features/deploys/components/deploy-page-layout";
import { CHART_FRAME_STYLE } from "@/features/metrics/components/chart-layout";
import { ComputePlanSectionSkeleton } from "@/features/services/components/compute-plan-frames";
import {
  AutoscalingLoadingCard,
  ManualScalingSkeleton,
  ScalingRecentMetricsSkeleton,
} from "@/features/services/components/scaling-frames";
import {
  autoSleepEligibleType,
  planSleeps,
} from "@/features/services/lib/idle-timeout";
import {
  deriveServiceType,
  isCronType,
  isDockerBuild,
  isStaticSiteType,
  publiclyRoutable,
  supportsMaxShutdownDelay,
  supportsScalingType,
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

/** A pending frame's named region, which the skeleton manifest lists. */
export function Region({
  name,
  className,
  children,
  ...rest
}: {
  name: string;
  className?: string;
  children: React.ReactNode;
} & Omit<React.ComponentProps<"div">, "className" | "children">) {
  return (
    <div data-skeleton-region={name} className={className} {...rest}>
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
            className="h-8 w-24 shrink-0 rounded-full max-lg:h-11 pointer-coarse:h-11 @4xl/page:w-full @4xl/page:rounded-md"
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

/**
 * The Billing page's pending frame (w4/707): each region down to Charges keeps
 * its ready card's structure and height for the current month at every width,
 * read from the workspace the shell already holds.
 */
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
        <PlanCardSkeleton />
      </Region>
      <Region name="payment-method">
        <PaymentMethodCardSkeleton />
      </Region>
      <Region name="included-usage" className="space-y-6">
        <IncludedUsageSkeleton />
      </Region>
      {/* The charge tree's categories and lines are the month's data: a
          fixed frame, whose height the ready card does not keep. */}
      <Region name="charges">
        <CardSkeleton rows={7} />
      </Region>
      <Region name="agent-model">
        <AgentModelCardSkeleton />
      </Region>
      <Region name="credit-balance">
        <CreditBalanceCardSkeleton />
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
  let child: React.ReactNode;
  if (routeId === "/project/$projectId/settings") {
    child = <ProjectSettingsPageSkeleton />;
  } else if (
    routeId === "/project/$projectId/observability/logs/application" ||
    routeId === "/project/$projectId/observability/logs/request"
  ) {
    child = <ProjectLogViewPageSkeleton />;
  } else if (routeId === "/project/$projectId/observability/metrics") {
    child = <ProjectMetricsViewPageSkeleton />;
  } else {
    child = <ProjectOverviewPageSkeleton />;
  }
  return (
    <PendingFrame
      route="project-active-child"
      className="flex min-h-0 flex-1 flex-col"
    >
      <Region name="active-child" className="contents">
        {child}
      </Region>
    </PendingFrame>
  );
}

/**
 * A project observability view (w3/m505) while its route or its saved search
 * loads: the ready page's frame and header with the view's name reserved and
 * its Edit view, Duplicate and Delete buttons (w3/m507) disabled, the
 * log explorer's toolbar rows (one on a desktop, [search] then [range | Live
 * | Maximize | Options] on a phone, w3/m509), its range note, and the log
 * panel at its own height.
 */
export function ProjectLogViewPageSkeleton() {
  const { t } = useTranslations();
  return (
    <PageFrame
      pending="project-log-view"
      width="wide"
      header={
        <ObservabilityViewHeader
          name={<Skeleton className="h-6 w-44" />}
          actions={<ViewActionButtons disabled />}
        />
      }
    >
      <div className="space-y-4">
        <Region
          name="log-toolbar"
          className="flex flex-wrap items-center gap-3 max-sm:gap-2"
        >
          <Skeleton className="h-9 min-w-48 flex-1 max-sm:min-w-0 max-sm:basis-full" />
          <Skeleton className="h-8 w-40 max-sm:w-auto max-sm:min-w-0 max-sm:flex-1" />
          {/* LiveToggle's own markup and row height (a 44px touch target
              below lg, which sets the phone's second row), without pulling
              the filter bar into the skeleton chunk. */}
          <Label className={LIVE_TOGGLE_CLASS}>
            <Switch checked disabled />
            {t("logs.live")}
          </Label>
          <div className={LOG_ACTIONS_CLASS}>
            <Skeleton className={LOG_ICON_BUTTON_CLASS} />
            <Skeleton className={LOG_ICON_BUTTON_CLASS} />
          </div>
        </Region>
        <p className="text-xs text-muted-foreground">
          {t("logs.rangeHistoryNote")}
        </p>
        <Region name="log-panel">
          <LogPanelSkeleton fillViewport />
        </Region>
      </div>
    </PageFrame>
  );
}

/**
 * A project's Service Metrics view (w3/m508) while its route or its saved
 * search loads: the view header with its buttons disabled, the range
 * picker, the legend row, and the Application and Network cards with their
 * real titles, each chart's title row (with the Group by and Percentile
 * selects where the ready charts have them) over a frame at the chart's own
 * ratio, as the ready sections draw while their reads are in flight.
 */
export function ProjectMetricsViewPageSkeleton() {
  const { t } = useTranslations();
  const cards = [
    {
      region: "application-metrics",
      title: t("metrics.applicationTitle"),
      description: t("metrics.projectApplicationDescription"),
      charts: [
        { title: t("metrics.cpu") },
        { title: t("metrics.memory") },
        { title: t("metrics.totalInstances") },
      ],
    },
    {
      region: "network-metrics",
      title: t("metrics.networkTitle"),
      description: t("metrics.projectNetworkDescription"),
      charts: [
        { title: t("metrics.totalRequests"), control: "w-44" },
        { title: t("metrics.responseTimes"), control: "w-36" },
        { title: t("metrics.outboundBandwidth") },
      ],
    },
  ];
  return (
    <PageFrame
      pending="project-metrics-view"
      width="wide"
      header={
        <ObservabilityViewHeader
          name={<Skeleton className="h-6 w-44" />}
          actions={<ViewActionButtons disabled />}
        />
      }
    >
      <div className="space-y-6">
        <Region
          name="metrics-toolbar"
          className="flex flex-wrap items-center gap-3"
        >
          <Skeleton className="h-8 w-40" />
        </Region>
        <Region
          name="metrics-legend"
          className="flex min-h-7 flex-wrap items-center gap-2"
        >
          {["w-24", "w-20", "w-28"].map((width) => (
            <Skeleton key={width} className={`h-7 ${width}`} />
          ))}
        </Region>
        {cards.map((card) => (
          <Region key={card.region} name={card.region}>
            <Card>
              <CardHeader>
                <CardTitle>{card.title}</CardTitle>
                <CardDescription>{card.description}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {card.charts.map((chart) => (
                  <div key={chart.title}>
                    <div className="mb-2 flex flex-wrap items-center justify-between gap-x-2 gap-y-1">
                      <div className="text-sm font-medium text-foreground">
                        {chart.title}
                      </div>
                      {chart.control ? (
                        <Skeleton className={`h-8 ${chart.control}`} />
                      ) : null}
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
        ))}
      </div>
    </PageFrame>
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
              <PlanTierSlots count={KEY_VALUE_PLAN_SLOTS} connections />
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

/** Workspace Settings' sections while the page or its workspace loads: the
 *  route's pending frame and the page's own inline skeleton draw this one
 *  body, so the two cannot drift apart. */
export function WorkspaceSettingsSkeletonSections({
  sections,
}: {
  sections: WorkspaceSettingsPendingSections;
}) {
  // A known non-admin's cards are drawn as the ready page draws them for
  // that caller (w7/094); an admin and an unknown role keep the Admin's.
  const canManage = sections.canManage ?? true;
  const readOnly = sections.canManage === false;
  return (
    <>
      <Region name="general">
        <WorkspaceDetailsCardSkeleton planOwner={sections.planOwner} />
      </Region>
      {sections.privacy ? (
        <Region name="privacy">
          <WorkspacePrivacyCardSkeleton />
        </Region>
      ) : null}
      {sections.buildPipeline ? (
        <Region name="build-pipeline">
          <BuildPipelineCardSkeleton />
        </Region>
      ) : null}
      {/* Overlapping Deploy Policy (w4/653): every member reads it. */}
      <Region name="overlapping-deploy-policy">
        <OverlappingDeployPolicyCardSkeleton canManage={canManage} />
      </Region>
      {/* Inbound IP Rules (w9/m388): every member reads them. */}
      <Region name="inbound-ip-rules">
        <WorkspaceIPRulesCardSkeleton
          belowScale={sections.belowScale}
          canManage={canManage}
        />
      </Region>
      {/* Two-factor authentication (w22/m402) and Google login (w7/m465):
          every member sees both cards. */}
      <Region name="two-factor">
        <WorkspaceLoginRequirementCardSkeleton
          requirement="twoFactor"
          status={sections.loginRequirementStatus?.twoFactor}
          belowPro={sections.belowPro}
          organization={sections.organization}
        />
      </Region>
      <Region name="google-login">
        <WorkspaceLoginRequirementCardSkeleton
          requirement="googleLogin"
          status={sections.loginRequirementStatus?.googleLogin}
          belowPro={sections.belowPro}
          organization={sections.organization}
        />
      </Region>
      <Region name="team">
        <TeamPanelSkeleton canManage={canManage} />
      </Region>
      <Region name="notifications" className="space-y-6">
        <WorkspaceNotificationsCardSkeleton readOnly={readOnly} />
        <NotificationOverridesCardSkeleton />
      </Region>
      <Region name="agents" className="space-y-6">
        <AgentEnvironmentCardSkeleton readOnly={readOnly} />
        <AgentSecretsCardSkeleton canManage={sections.canManage} />
        <AgentRepoEnvironmentsCardSkeleton canManage={sections.canManage} />
        <AgentSkillsCardSkeleton canManage={sections.canManage} />
      </Region>
      <Region name="registry-credentials">
        <RegistryCredentialsPanelSkeleton />
      </Region>
      {sections.auditLogs ? (
        <Region name="audit-logs">
          <AuditLogPanelSkeleton />
        </Region>
      ) : null}
      <Region name="danger-zone" className="space-y-6">
        <LeaveWorkspaceCardSkeleton workspaceName={sections.workspaceName} />
        {sections.deleteWorkspace ? (
          <DeleteWorkspaceCardSkeleton workspaceName={sections.workspaceName} />
        ) : null}
      </Region>
    </>
  );
}

export function WorkspaceSettingsPageSkeleton() {
  const { t } = useTranslations();
  const sections = useWorkspaceSettingsSkeletonSections();
  return (
    <PageFrame
      pending="workspace-settings"
      header={
        <PageHeader
          title={t("workspaces.settingsTitle")}
          description={t("workspaces.settingsDescription")}
        />
      }
      navigation={
        <WorkspaceSettingsNavigation
          showDangerZone
          showAuditLogs={sections.auditLogs}
          showBuildPipeline={sections.buildPipeline}
          showPrivacy={sections.privacy}
        />
      }
    >
      <WorkspaceSettingsSkeletonSections sections={sections} />
    </PageFrame>
  );
}

/** An Account Settings section: its real heading over its cards' frames. */
function SettingsSectionRegion({
  name,
  title,
  description,
  children,
}: {
  name: string;
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <Region name={name} className="space-y-4">
      <SettingsSectionHeading title={title} description={description} />
      {children}
    </Region>
  );
}

/**
 * Account Settings while the page's chunk loads (w3/067): each section's real
 * heading over the frames its cards draw while they load, so a section stands
 * as tall as it will once ready. The Kratos flow's six cards are the page's own
 * flow frame. Residuals are the account's data: rows past each table's first,
 * an empty agents list, a connected GitHub account's list, and the account's
 * second factors.
 */
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
        <SettingsSectionRegion
          name="account"
          title={t("auth.accountSection")}
          description={t("auth.accountSectionSubtitle")}
        >
          <AccountSettingsFlowSkeleton />
        </SettingsSectionRegion>
        <SettingsSectionRegion
          name="integrations"
          title={t("auth.integrationsSection")}
          description={t("auth.integrationsSectionSubtitle")}
        >
          <ConnectGithubCardSkeleton />
        </SettingsSectionRegion>
        <SettingsSectionRegion
          name="access"
          title={t("auth.accessSection")}
          description={t("auth.accessSectionSubtitle")}
        >
          <ApiKeysPanelSkeleton />
          <SSHKeysPanelSkeleton />
        </SettingsSectionRegion>
        <SettingsSectionRegion
          name="security"
          title={t("auth.securityComplianceSection")}
          description={t("auth.securityComplianceSectionSubtitle")}
        >
          <ConnectedAgentsPanelSkeleton />
          <ActiveSessionsPanelSkeleton />
        </SettingsSectionRegion>
        <SettingsSectionRegion
          name="danger-zone"
          title={t("auth.dangerZoneSection")}
          description={t("auth.dangerZoneSectionSubtitle")}
        >
          <AccountDeletionCardSkeleton />
        </SettingsSectionRegion>
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

/** The flows whose Ory card has a loading frame. */
type OryCardFlow = "login" | "registration" | "recovery" | "verification";

/** Ory's "a, b or c" (`joinWithCommaOr` in its theme). */
function joinWithOr(parts: readonly string[], or: string): string {
  if (parts.length < 2) return parts[0] ?? "";
  return `${parts.slice(0, -1).join(", ")} ${or} ${parts.at(-1)}`;
}

/**
 * The words Ory's card draws for a flow's first step, in the page's language
 * from the mirrored catalog (`ory-copy/keys.ts`, w7/092): the header
 * (`useCardHeaderText` in `@ory/elements-react`'s theme) and, on login and
 * sign-up, the footer's link to the other one. Login's subtitle names the
 * e-mail and password (Ory passes the literal "email" in every language),
 * sign-up's the e-mail field by the label the forms give it (`useOryConfig`),
 * and both add "a social provider" when the card offers one, in Ory's order.
 */
function useOryCardCopy(
  flow: OryCardFlow,
  hasSso: boolean,
): { title: string; subtitle: string; footer?: string } {
  const { t } = useTranslations();
  const sso = hasSso ? [t("oryCard.oidcPart")] : [];
  switch (flow) {
    case "login":
      return {
        title: t("oryCard.loginTitle"),
        subtitle: t("oryCard.loginSubtitle", {
          parts: joinWithOr(
            [t("oryCard.passwordPart", { identifierLabel: "email" }), ...sso],
            t("oryCard.or"),
          ),
        }),
        footer: `${t("oryCard.loginFooterLabel")} ${t("oryCard.loginFooterLink")}`,
      };
    case "registration":
      return {
        title: t("oryCard.registrationTitle"),
        subtitle: t("oryCard.registrationSubtitle", {
          parts: joinWithOr(
            [
              ...sso,
              t("oryCard.identifierPart", {
                identifierLabel: t("auth.profileEmailLabel"),
              }),
            ],
            t("oryCard.or"),
          ),
        }),
        footer: `${t("oryCard.registrationFooterLabel")} ${t("oryCard.registrationFooterLink")}`,
      };
    case "recovery":
      return {
        title: t("oryCard.recoveryTitle"),
        subtitle: t("oryCard.recoverySubtitle"),
      };
    case "verification":
      return {
        title: t("oryCard.verificationTitle"),
        subtitle: t("oryCard.verificationSubtitle"),
      };
  }
}

/**
 * Ory's flow card while its flow loads, at the ready card's geometry under the
 * dashboard's overrides (`style.css`): the header, the card's parts 24 px
 * apart, and the footer link when the card has one. The header and footer
 * reserve Ory's own words in its font and type scale, so they wrap to as many
 * lines as the card's do at any width and in every language (w7/092). Below
 * `sm` Ory's card is square with only its bottom border.
 */
function OryCardSkeleton({
  flow,
  hasSso = false,
  children,
}: {
  flow: OryCardFlow;
  hasSso?: boolean;
  children: React.ReactNode;
}) {
  const { title, subtitle, footer } = useOryCardCopy(flow, hasSso);
  return (
    <Region
      name="auth-widget"
      className="grid gap-6 border-b bg-card p-6 shadow-sm [font-family:var(--font-sans-default)] sm:rounded-xl sm:border sm:p-8"
    >
      <div data-skeleton-card-header className="flex flex-col gap-2">
        <span className="relative w-fit max-w-full text-lg leading-normal font-semibold">
          <ReservedText>{title}</ReservedText>
        </span>
        <span className="relative w-fit max-w-full text-base leading-normal">
          <ReservedText>{subtitle}</ReservedText>
        </span>
      </div>
      {children}
      {footer ? (
        <span
          data-skeleton-card-footer
          className="relative w-fit max-w-full text-base leading-normal"
        >
          <ReservedText>{footer}</ReservedText>
        </span>
      ) : null}
    </Region>
  );
}

/**
 * A labelled field as Ory's `DefaultLabel` draws it: the label's words and the
 * link Ory sets beside them (login's "Recover Account") at the row's two ends
 * in its 16 px type, then the h-9 input. The words are reserved, so at phone
 * width the row wraps to the lines the card's does (w7/093).
 */
function FieldSkeleton({
  label,
  action,
  inputClassName = "h-9",
}: {
  label: string;
  action?: string;
  inputClassName?: string;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="flex items-start justify-between text-base leading-normal">
        <span data-skeleton-field-label className="relative">
          <ReservedText>{label}</ReservedText>
        </span>
        {action ? (
          <span data-skeleton-field-action className="relative">
            <ReservedText>{action}</ReservedText>
          </span>
        ) : null}
      </span>
      <Skeleton className={cn("w-full", inputClassName)} />
    </div>
  );
}

/** The recovery and verification cards: the e-mail field and "Continue", 32 px
 *  apart, and no footer. */
export function AuthWidgetSkeleton({
  flow,
}: {
  flow: "recovery" | "verification";
}) {
  const { t } = useTranslations();
  return (
    <OryCardSkeleton flow={flow}>
      <div className="grid gap-8">
        <FieldSkeleton label={t("oryCard.emailLabel")} />
        <ReservedOryButton afterInput data-skeleton-submit>
          {t("oryCard.continue")}
        </ReservedOryButton>
      </div>
    </OryCardSkeleton>
  );
}

/**
 * The login or sign-up card, in Render's order as `sso-buttons.tsx` lays it
 * out: the labelled fields and the submit button 32 px apart, then "Or
 * continue with" (its words between two rules) and the installation's
 * provider buttons (Render's first row, then the "More options" line when it
 * hides GitLab and Bitbucket), and the footer link. The subtitle names "a
 * social provider" when the card offers one. `ssoProviders` is
 * `loadSsoProviders`' answer; null (not known) draws no providers.
 */
function OryAuthCardSkeleton({
  flow,
  ssoProviders,
  children,
}: {
  flow: "login" | "registration";
  ssoProviders: readonly string[] | null;
  children: React.ReactNode;
}) {
  const providers = ssoProviders ?? [];
  const { primary, more } = arrangeSsoProviders(providers);
  const hasSso = providers.length > 0;
  return (
    <OryCardSkeleton flow={flow} hasSso={hasSso}>
      <div className="grid gap-8">{children}</div>
      {hasSso ? (
        <>
          <div data-skeleton-sso-divider className="flex items-center gap-3">
            <div className="h-px flex-1 bg-border" />
            <Skeleton className="h-5 w-28" />
            <div className="h-px flex-1 bg-border" />
          </div>
          <div data-skeleton-sso={primary.length} className="grid gap-3">
            <div className={cn("grid gap-3", ssoRowColumns(primary.length))}>
              {primary.map((provider) => (
                <Skeleton key={provider} className="h-9 w-full" />
              ))}
            </div>
            {more.length > 0 ? (
              <Skeleton
                data-skeleton-sso-more
                className="h-5 w-24 justify-self-center"
              />
            ) : null}
          </div>
        </>
      ) : null}
    </OryCardSkeleton>
  );
}

/** Login: e-mail (with "Recover Account" beside its label), the password box
 *  (38 px with its show/hide toggle), and "Sign in with password". */
export function LoginWidgetSkeleton({
  ssoProviders,
}: {
  ssoProviders: readonly string[] | null;
}) {
  const { t } = useTranslations();
  return (
    <OryAuthCardSkeleton flow="login" ssoProviders={ssoProviders}>
      <FieldSkeleton
        label={t("auth.profileEmailLabel")}
        action={t("oryCard.recoverAccount")}
      />
      <FieldSkeleton
        label={t("orySettings.passwordLabel")}
        inputClassName="h-[38px]"
      />
      <ReservedOryButton afterInput data-skeleton-submit>
        {t("oryCard.loginSubmit")}
      </ReservedOryButton>
    </OryAuthCardSkeleton>
  );
}

/** Sign-up's first step: e-mail and name, then "Sign up" (Kratos asks for the
 *  password on the next one). */
export function RegistrationWidgetSkeleton({
  ssoProviders,
}: {
  ssoProviders: readonly string[] | null;
}) {
  const { t } = useTranslations();
  return (
    <OryAuthCardSkeleton flow="registration" ssoProviders={ssoProviders}>
      <FieldSkeleton label={t("auth.profileEmailLabel")} />
      <FieldSkeleton label={t("auth.profileNameLabel")} />
      <ReservedOryButton afterInput data-skeleton-submit>
        {t("oryCard.registrationSubmit")}
      </ReservedOryButton>
    </OryAuthCardSkeleton>
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
      {/* min-w-0: the page header bars must not widen the column past a
          narrow viewport. */}
      <div className="flex min-w-0 flex-1 items-center justify-center px-6 py-12 lg:px-8">
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

// The route's pending frame has no loader data yet: it draws the providers
// this browser last read from a login or sign-up flow.
export function LoginRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-login" featureRail>
      <LoginWidgetSkeleton ssoProviders={rememberedSsoProviders()} />
    </AuthRouteSkeleton>
  );
}

export function RegistrationRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-registration" featureRail>
      <RegistrationWidgetSkeleton ssoProviders={rememberedSsoProviders()} />
    </AuthRouteSkeleton>
  );
}

export function RecoveryRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-recovery">
      <AuthWidgetSkeleton flow="recovery" />
    </AuthRouteSkeleton>
  );
}

export function VerificationRouteSkeleton() {
  return (
    <AuthRouteSkeleton route="auth-verification">
      <AuthWidgetSkeleton flow="verification" />
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
                    {t("services.createFieldPlanCronHint", {
                      minimum: CRON_MINIMUM_USD,
                    })}
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
  commits,
}: {
  /** The auto-deploy-off banner the ready page shows above the list
   *  (w3/064): its own content is already known, so the pending state shows
   *  it as is and matches at every width. */
  banner?: React.ReactNode;
  /** False when the cached service is image-backed: no commit lines. */
  commits?: boolean;
} = {}) {
  return (
    <PendingFrame route="deploys-list" className="space-y-4">
      {banner ? <Region name="auto-deploy-off-banner">{banner}</Region> : null}
      <Region name="deploys-table">
        <DeploysListFrameSkeleton commits={commits} />
      </Region>
    </PendingFrame>
  );
}

export function DeployDetailSkeleton() {
  return (
    <PendingFrame route="deploy-detail" className="space-y-6">
      <Region name="deploy-header" className={DEPLOY_PAGE_TOP_CLASS}>
        <DeployBackLinkSkeleton />
        <DeployHeaderSkeleton />
      </Region>
      <div className={DEPLOY_BODY_CLASS}>
        <Region name="deploy-logs" className="space-y-3">
          <DeployLogToolbarSkeleton />
          <LogPanelSkeleton fillViewport="flush" />
        </Region>
        <Region name="deploy-timeline">
          <DeployTimelineCardSkeleton />
        </Region>
      </div>
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

// A typical feed's mix of detail lines per row — a restart's one, a finished
// deploy's commit and pre-deploy lines, a started deploy's one, then
// configuration changes with an outcome line each — and how many of its meta
// lines wrap onto a second line in a phone's narrower column.
const EVENT_ROW_BODY_LINES = [
  { lines: 1, phoneWraps: 0 },
  { lines: 3, phoneWraps: 0 },
  { lines: 1, phoneWraps: 1 },
  { lines: 2, phoneWraps: 0 },
  { lines: 2, phoneWraps: 1 },
  { lines: 2, phoneWraps: 1 },
];
const EVENT_BODY_BAR_WIDTHS = ["w-56", "w-44", "w-40"];

/**
 * The feed's rows while its first page loads, on the loaded rows' grid: the
 * icon disc, the title line, the time (a line under the title, or two lines
 * in its own column in the wide layout), the detail lines, then the Load
 * older slot.
 */
export function ServiceEventRowsSkeleton() {
  return (
    <>
      <div className="divide-y">
        {EVENT_ROW_BODY_LINES.map(({ lines, phoneWraps }, index) => (
          <div key={index} className={EVENT_ROW_CLASS}>
            <Skeleton className={EVENT_ICON_CELL} />
            <div className={EVENT_TITLE_CELL}>
              <Skeleton className="h-3.5 w-36" />
            </div>
            <div className={EVENT_TIME_CELL}>
              <div className="flex h-5 items-center">
                <Skeleton className="h-3 w-28" />
              </div>
              <div className="hidden h-5 items-center @2xl/page:flex">
                <Skeleton className="h-3 w-16" />
              </div>
            </div>
            <div className={EVENT_BODY_CELL}>
              {Array.from({ length: lines + phoneWraps }, (_, line) => (
                <div
                  key={line}
                  className={cn(
                    "flex h-5 items-center",
                    // The phone-only wraps come first: `space-y` spaces every
                    // line but the last, so a hidden last line would leave
                    // its neighbour's gap behind on a wider screen.
                    line < phoneWraps && "@md/page:hidden",
                  )}
                >
                  <Skeleton
                    className={`h-3 max-w-full ${EVENT_BODY_BAR_WIDTHS[line % EVENT_BODY_BAR_WIDTHS.length]}`}
                  />
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
      <div className={EVENT_FOOTER_CLASS}>
        <Skeleton className="h-8 w-40 max-lg:h-11 pointer-coarse:h-11" />
      </div>
    </>
  );
}

export function ServiceEventsSkeleton() {
  return (
    <PendingFrame route="service-events" className="space-y-6">
      <Region name="event-feed">
        <Card className="gap-0 overflow-hidden py-0">
          {/* The count on the left, the filter on the right, as loaded. */}
          <div className={EVENT_TOOLBAR_CLASS}>
            <Skeleton className="h-4 w-36" />
            <Skeleton className="h-8 w-32 max-lg:h-11 pointer-coarse:h-11" />
          </div>
          <ServiceEventRowsSkeleton />
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function ServiceLogsSkeleton() {
  return (
    <PendingFrame route="service-logs">
      <ServiceLogsBodySkeleton />
    </PendingFrame>
  );
}

// A filter-bar control's height: 32px, or the 44px touch target.
const LOG_CONTROL = "h-8 max-lg:h-11 pointer-coarse:h-11";

/**
 * The Logs tab's loading body, control for control: the filter bar at its
 * controls' widths and heights (two rows on a phone, as loaded, with
 * Maximize and Options at the end, w3/m509), then the log panel at its
 * screen-filling height. The route's pending frame and the tab's
 * own hold (while it checks the service is not a static site) both show it,
 * so the bar never appears late above a panel that then drops.
 */
export function ServiceLogsBodySkeleton() {
  const frameRef = useRef<HTMLDivElement>(null);
  useOpenDocked(frameRef);
  return (
    <div ref={frameRef} data-log-frame="" className="space-y-4">
      <Region
        name="log-filters"
        data-log-dock=""
        className={LOG_FILTER_BAR_CLASS}
      >
        <Skeleton className={cn(LOG_SELECT_WIDTH_CLASS, LOG_CONTROL)} />
        <Skeleton
          className={cn("min-w-48 flex-1 max-sm:min-w-0", LOG_CONTROL)}
        />
        <Skeleton className={cn(LOG_SELECT_WIDTH_CLASS, LOG_CONTROL)} />
        <div className={LOG_FILTER_TAIL_CLASS}>
          <Skeleton className={cn("w-[5.25rem] max-sm:flex-1", LOG_CONTROL)} />
          {/* The Live switch, then its word, in the control's own row. */}
          <div className={LIVE_TOGGLE_CLASS}>
            <Skeleton className="h-5 w-9 rounded-full" />
            <Skeleton className="h-3.5 w-7" />
          </div>
        </div>
        {/* Maximize (inside Options on a phone), then Options. */}
        <div className={LOG_FILTER_ACTIONS_CLASS}>
          <Skeleton className={cn(LOG_ICON_BUTTON_CLASS, "max-sm:hidden")} />
          <Skeleton className={LOG_ICON_BUTTON_CLASS} />
        </div>
      </Region>
      <Region name="log-panel">
        <LogPanelSkeleton fillViewport />
      </Region>
    </div>
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
      {/* The tab's name is the layout's heading; this row is its
          description and page action. */}
      <Region
        name="page-header"
        className={cn(
          "flex flex-col justify-between gap-3 sm:flex-row sm:items-center",
          SERVICE_TAB_DESCRIPTION_OFFSET,
        )}
      >
        <div className="w-full space-y-1 sm:w-auto">
          <Skeleton className="h-4 w-96 max-w-full" />
          <Skeleton className="h-5 w-3/4 sm:hidden" />
        </div>
        <Skeleton className="h-9 w-36" />
      </Region>
      <Region name="environment-editor" className="space-y-6">
        {/* Variables (two rows), then Secret Files (one row, no header
            actions), on the loaded rows' grid: key, value, then Reveal and
            copy — beside the key on a phone. */}
        {[2, 1].map((rows, index) => (
          <Card key={index}>
            <CardHeader className="grid-cols-1 grid-rows-none sm:grid-cols-[minmax(0,1fr)_auto] sm:grid-rows-[auto_auto]">
              <div className="space-y-2">
                <Skeleton className="h-5 w-40" />
                <div className="space-y-1">
                  <Skeleton className="h-4 w-72 max-w-full" />
                  <Skeleton className="h-4 w-2/3 sm:hidden" />
                </div>
              </div>
              {index === 0 ? (
                <div className="col-start-1 row-start-2 mt-2 flex gap-2 sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0">
                  <Skeleton className="h-8 w-24 max-lg:h-11 pointer-coarse:h-11" />
                  <Skeleton className="h-8 w-16 max-lg:h-11 pointer-coarse:h-11" />
                </div>
              ) : null}
            </CardHeader>
            <CardContent>
              <EnvRowsSkeleton rows={rows} />
            </CardContent>
          </Card>
        ))}
      </Region>
      <Region name="environment-groups">
        <Card>
          <CardHeader className="grid-cols-1 grid-rows-none sm:grid-cols-[minmax(0,1fr)_auto] sm:grid-rows-[auto_auto]">
            <div className="space-y-2">
              <Skeleton className="h-5 w-48" />
              <div className="space-y-1">
                <Skeleton className="h-4 w-4/5" />
                <Skeleton className="h-4 w-full sm:hidden" />
              </div>
            </div>
            <Skeleton className="col-start-1 row-start-2 mt-2 h-8 w-32 max-lg:h-11 pointer-coarse:h-11 sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0" />
          </CardHeader>
          {/* "Linked (n)" and one group card, then "Available to link"
              and its box. */}
          <CardContent className="space-y-3">
            <Skeleton className="h-5 w-20" />
            <Skeleton className="h-[7.25rem] w-full rounded-lg sm:h-[6.75rem]" />
            <Skeleton className="mt-6 h-5 w-28" />
            <Skeleton className="h-24 w-full rounded-lg" />
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
        // The loaded card's heights, its Edit now in the header's action
        // slot (measured: 228/148 with a repo, 44px less on a phone without).
        repo ? "h-[228px] sm:h-[148px]" : "h-[188px] sm:h-[148px]",
        "overflow-hidden",
      )}
    >
      <CardHeader>
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
        <CardAction>
          <Skeleton className="h-8 w-14" />
        </CardAction>
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

/**
 * The Compute page (w6/m483): the Compute section with its plan row, then —
 * for a scalable type — Autoscaling, Manual Scaling and Recent Metrics, the
 * order the ready page draws them in. A cron job has a compute plan but no
 * scaling, so its frame stops after the Compute section. An undecided type
 * (null) reserves the scaling sections every other compute type has.
 */
export function ServiceComputeSkeleton({
  serviceType = null,
  currentPlan,
}: {
  serviceType?: string | null;
  /** Undefined while unknown. */
  currentPlan?: string | null;
}) {
  const { currentWorkspace } = useWorkspace();
  const scalable =
    serviceType === null || supportsScalingType(deriveServiceType(serviceType));
  return (
    <PendingFrame route="service-compute" className="space-y-6">
      <Region name="compute-plan">
        <ComputePlanSectionSkeleton
          serviceType={serviceType}
          currentPlan={currentPlan}
        />
      </Region>
      {scalable ? (
        <>
          {/* The scaling cards' own loading frames, which hold the ready
              cards' geometry in the autoscaling-off state (w4/688). */}
          <Region name="autoscaling">
            <AutoscalingLoadingCard
              plan={currentPlan}
              workspacePlan={currentWorkspace?.plan}
            />
          </Region>
          <Region name="manual-scaling">
            <ManualScalingSkeleton plan={currentPlan} />
          </Region>
          <Region name="recent-metrics">
            <ScalingRecentMetricsSkeleton />
          </Region>
        </>
      ) : null}
    </PendingFrame>
  );
}

export function ServiceDiskSkeleton() {
  return (
    <PendingFrame route="service-disk" className="space-y-6">
      <Region
        name="disk-description"
        className={cn("space-y-1", SERVICE_TAB_DESCRIPTION_OFFSET)}
      >
        <Skeleton className="h-4 w-full max-w-xl" />
        <Skeleton className="h-4 w-2/3 sm:hidden" />
      </Region>
      <Region name="disk-card">
        <Card>
          <CardContent>
            <PanelCenteredStateSkeleton />
          </CardContent>
        </Card>
      </Region>
    </PendingFrame>
  );
}

export function ServiceShellSkeleton() {
  return (
    <PendingFrame route="service-shell" className="space-y-6">
      {/* The tab's name is the layout's heading; this is its description. */}
      <Region
        name="page-header"
        className={cn("space-y-1", SERVICE_TAB_DESCRIPTION_OFFSET)}
      >
        <Skeleton className="h-4 w-72 max-w-full" />
        <Skeleton className="h-5 w-3/4 sm:hidden" />
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

/** A ready muted help paragraph, wrapping as its translated copy does. */
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
      {/* DatastoreLogViewer's toolbar, control for control: range,
          instance and level, search, Live, then Maximize and Options
          (w3/m509), which a phone puts at the range's row's end. The page
          is no touch region, so only Live and the icon buttons grow on a
          touch layout. */}
      <Region name="log-filters" className="flex flex-wrap items-center gap-3">
        <Skeleton className="h-8 w-40 max-sm:order-first" />
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-9 min-w-56 flex-1" />
        <div className={LIVE_TOGGLE_CLASS}>
          <Skeleton className="h-5 w-9 rounded-full" />
          <Skeleton className="h-3.5 w-7" />
        </div>
        <div className={DATASTORE_LOG_ACTIONS_CLASS}>
          <Skeleton className={LOG_ICON_BUTTON_CLASS} />
          <Skeleton className={LOG_ICON_BUTTON_CLASS} />
        </div>
      </Region>
      <Region name="log-lines" className="min-h-64">
        <LogPanelSkeleton />
      </Region>
    </PendingFrame>
  );
}

// DatastoreMetricsPanel's sections, in its order.
const DATASTORE_METRIC_SECTIONS = {
  keyvalue: [
    "metrics.diskTitle",
    "metrics.diskActivityTitle",
    "metrics.datastoreCpuTitle",
    "metrics.memoryTitle",
    "metrics.connectionsTitle",
  ],
  database: [
    "metrics.diskTitle",
    "metrics.diskActivityTitle",
    "metrics.diskOperationsTitle",
    "metrics.datastoreCpuTitle",
    "metrics.memoryTitle",
    "metrics.connectionsTitle",
    "metrics.networkActivityTitle",
    "metrics.transactionVolumeTitle",
    "metrics.replicationLagTitle",
    "metrics.lockDelayedQueriesTitle",
  ],
} as const;

/**
 * DatastoreMetricsPanel before its resource resolves (w4/m209): the panel's
 * real title, description and section headings, its range picker's slot
 * (w7/072), and each plot at the populated chart's responsive 600:180 ratio,
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
            <Region
              name="metrics-range"
              className="flex flex-wrap items-center gap-3"
            >
              <Skeleton className="h-8 w-40" />
            </Region>
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
  | "compute"
  | "metrics"
  | "rules"
  | "settings"
  | "shell";

const SERVICE_ROUTE_SKELETON_BY_ID = {
  "/services/$serviceId": "deploys",
  "/services/$serviceId/": "deploys",
  "/services/$serviceId/builds": "deploys",
  "/services/$serviceId/compute": "compute",
  "/services/$serviceId/deploys/": "deploys",
  "/services/$serviceId/deploys/$deployId": "deploy-detail",
  "/services/$serviceId/disk": "disk",
  "/services/$serviceId/env": "environment",
  "/services/$serviceId/events": "events",
  "/services/$serviceId/headers": "rules",
  "/services/$serviceId/logs": "logs",
  "/services/$serviceId/metrics": "metrics",
  "/services/$serviceId/redirects": "rules",
  "/services/$serviceId/runs": "cron-runs",
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
  deploys,
}: {
  base: "/services" | "/static";
  /** The service's type when already known (cache), for type-shaped tabs. */
  serviceType?: string | null;
  /** The deploy history's own pending state, drawn from the cached service
   *  (its auto-deploy-off banner, w3/064), so this frame and the route's
   *  skeleton that replaces it match. */
  deploys?: React.ReactNode;
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
      return deploys ?? <DeploysListSkeleton />;
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
    case "compute":
      return <ServiceComputeSkeleton serviceType={serviceType} />;
    case "rules":
      return <StaticEdgeRulesSkeleton />;
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
