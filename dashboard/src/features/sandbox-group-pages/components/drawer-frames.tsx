import { Skeleton, SkeletonSpan } from "@/common/components/ui/skeleton";
import {
  ReservedBadge,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { SheetHeader } from "@/common/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger } from "@/common/components/ui/tabs";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import type {
  ImageDrawerTab,
  SandboxDrawerTab,
} from "@/features/sandbox-group-pages/lib/drawer-tabs";

type MessageKey = keyof typeof en;

/**
 * The loading frames of a sandbox group's drawers and of their lists (w1/m658
 * t009). Each is the ready markup with its copy laid out invisibly under a
 * bar, so a drawer keeps its header, tabs and rows where they will be when the
 * data lands. The production walk found one-line "Loading" texts there that
 * grew into lists, and drawer skeletons shorter than their ready headers.
 */

/**
 * A drawer's header: the title row (32 px when it holds a menu), then the id
 * with its 32 px copy button, or the image's one-line source reference.
 */
function HeaderFrame({
  titleRow,
  subtitle,
}: {
  titleRow: "menu" | "plain";
  subtitle: "id" | "source";
}) {
  return (
    <SheetHeader className="border-b pr-12">
      <div
        className={
          titleRow === "menu"
            ? "flex min-h-8 items-center gap-2"
            : "flex min-h-6 items-center gap-2"
        }
      >
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-[22px] w-16 rounded-md" />
      </div>
      <div
        className={
          subtitle === "id" ? "flex h-8 items-center" : "flex h-4 items-center"
        }
      >
        <Skeleton className="h-3 w-44" />
      </div>
    </SheetHeader>
  );
}

/** A drawer's tab bar as its ready one, its tabs not yet usable. */
function TabsFrame<T extends string>({
  value,
  tabs,
}: {
  value: T;
  tabs: readonly { key: T; label: MessageKey }[];
}) {
  const { t } = useTranslations();
  return (
    <Tabs value={value} className="gap-0">
      <TabsList variant="underline" className="justify-start px-2">
        {/* No panel is mounted while the drawer loads, so the tabs name none
            (an aria-controls to a missing id is invalid ARIA). */}
        {tabs.map(({ key, label }) => (
          <TabsTrigger
            key={key}
            value={key}
            disabled
            aria-controls={undefined}
            className="px-3 py-2 text-sm"
          >
            {t(label)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}

/** The screen reader's word while a list loads; the frame itself is hidden. */
function LoadingStatus({ label }: { label: MessageKey }) {
  const { t } = useTranslations();
  return (
    <span role="status" className="sr-only">
      {t(label)}
    </span>
  );
}

/**
 * DetailRow's box for each label: `tall` is a value with a 32 px button (a
 * copy button, Make default), `twoLine` a value over a text-xs line (a
 * timeout over its deadline).
 */
export function DetailRowsFrame({
  rows,
}: {
  rows: readonly { label: MessageKey; tall?: boolean; twoLine?: boolean }[];
}) {
  const { t } = useTranslations();
  return (
    <dl className="divide-y" aria-hidden="true">
      {rows.map(({ label, tall, twoLine }) => (
        <div
          key={label}
          className="flex flex-col gap-0.5 py-2 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
        >
          <dt className="text-muted-foreground relative w-fit text-sm">
            <ReservedText>{t(label)}</ReservedText>
          </dt>
          <dd className="flex min-w-0 flex-col text-sm sm:items-end">
            <div
              className={
                tall ? "flex h-8 items-center" : "flex h-5 items-center"
              }
            >
              <Skeleton className={tall ? "h-3 w-44" : "h-3.5 w-28"} />
            </div>
            {twoLine ? (
              <div className="flex h-4 items-center">
                <Skeleton className="h-3 w-36" />
              </div>
            ) : null}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * The Events tab while the events load: the two lifecycle events every
 * started sandbox has (Ready, then Created), each as the ready row lays out.
 */
export function EventsFrame() {
  const { t } = useTranslations();
  const rows: MessageKey[] = [
    "sandboxGroupPages.eventReady",
    "sandboxGroupPages.eventCreated",
  ];
  return (
    <div className="space-y-3" data-skeleton-region="events-list">
      <LoadingStatus label="sandboxGroupPages.loadingEvents" />
      <ol className="space-y-3" aria-hidden="true">
        {rows.map((label) => (
          <li key={label} className="flex gap-3">
            <span className="bg-accent mt-1.5 size-2 shrink-0 rounded-full" />
            <div className="min-w-0 space-y-0.5">
              <p className="relative w-fit text-sm font-medium">
                <ReservedText>{t(label)}</ReservedText>
              </p>
              <p className="text-muted-foreground relative w-fit text-xs">
                <ReservedText>
                  {t("sandboxGroupPages.triggerDashboard")}
                </ReservedText>
              </p>
              <span className="text-muted-foreground relative text-xs">
                <ReservedText>
                  {t("sandboxGroupPages.loadingEvents")}
                </ReservedText>
              </span>
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}

/** One of the sandbox drawer's snapshot rows: name, when and size, status. */
export function SnapshotRowsFrame() {
  const { t } = useTranslations();
  return (
    <div data-skeleton-region="snapshot-rows">
      <LoadingStatus label="sandboxGroupPages.loading" />
      <ul className="divide-y rounded-lg border" aria-hidden="true">
        <li className="flex items-center justify-between gap-3 p-3">
          <div className="min-w-0">
            <span className="relative block w-fit text-sm font-medium">
              <ReservedText>{t("sandboxGroupPages.colSnapshot")}</ReservedText>
            </span>
            <span className="text-muted-foreground relative text-xs">
              <ReservedText>{t("sandboxGroupPages.colCaptured")}</ReservedText>
            </span>
          </div>
          <ReservedBadge variant="secondary">
            {t("sandboxGroupPages.snapshotAvailable")}
          </ReservedBadge>
        </li>
      </ul>
    </div>
  );
}

/** One of StartedSandboxesList's rows: a sandbox's name and its status. */
export function StartedRowsFrame() {
  const { t } = useTranslations();
  return (
    <div data-skeleton-region="started-rows">
      <LoadingStatus label="sandboxGroupPages.loading" />
      <ul className="divide-y rounded-lg border" aria-hidden="true">
        <li className="flex items-center justify-between gap-2 p-2">
          <span className="relative truncate text-sm">
            <ReservedText>{t("sandboxGroupPages.colSandbox")}</ReservedText>
          </span>
          <ReservedBadge variant="secondary">
            {t("sandboxGroupPages.statusRunning")}
          </ReservedBadge>
        </li>
      </ul>
    </div>
  );
}

/** One of the image drawer's version rows: id and status, then its source. */
function VersionRowsFrame() {
  return (
    <ul className="divide-y rounded-lg border" aria-hidden="true">
      <li className="space-y-1 p-3">
        <div className="flex items-center justify-between gap-2">
          <span className="relative font-mono text-xs">
            <span className="invisible">sbb-00000000000000000000</span>
            <SkeletonSpan />
          </span>
          <Skeleton className="h-[22px] w-16 rounded-md" />
        </div>
        <div className="text-muted-foreground flex flex-wrap gap-x-3 text-xs">
          <span className="relative">
            <span className="invisible">0000000</span>
            <SkeletonSpan />
          </span>
          <span className="relative">
            <span className="invisible">00000000000000000000</span>
            <SkeletonSpan />
          </span>
        </div>
      </li>
    </ul>
  );
}

const SANDBOX_TABS = [
  { key: "events", label: "sandboxGroupPages.tabEvents" },
  { key: "snapshots", label: "sandboxGroupPages.tabSnapshots" },
  { key: "details", label: "sandboxGroupPages.tabDetails" },
] as const satisfies readonly { key: SandboxDrawerTab; label: MessageKey }[];

const SANDBOX_DETAIL_ROWS = [
  { label: "sandboxGroupPages.detailSandboxId", tall: true },
  { label: "sandboxGroupPages.detailGroup" },
  { label: "sandboxGroupPages.detailPlan" },
  { label: "sandboxGroupPages.detailCreated" },
  { label: "sandboxGroupPages.detailDuration" },
  // A running sandbox's timeout sits over its deadline.
  { label: "sandboxGroupPages.detailTimeout", twoLine: true },
  { label: "sandboxGroupPages.detailIdleTimeout" },
  { label: "sandboxGroupPages.detailNetwork" },
  { label: "sandboxGroupPages.detailRegion" },
  { label: "sandboxGroupPages.detailSource" },
  { label: "sandboxGroupPages.detailTrigger" },
  { label: "sandboxGroupPages.detailUser" },
] as const satisfies readonly {
  label: MessageKey;
  tall?: boolean;
  twoLine?: boolean;
}[];

/** The sandbox drawer while the sandbox loads: header, tabs, the open tab. */
export function SandboxDrawerSkeleton({ tab }: { tab: SandboxDrawerTab }) {
  return (
    <div data-skeleton-region="drawer-body">
      <div aria-hidden="true">
        <HeaderFrame titleRow="menu" subtitle="id" />
        <TabsFrame value={tab} tabs={SANDBOX_TABS} />
      </div>
      <div className="p-4">
        {tab === "events" ? (
          <EventsFrame />
        ) : tab === "snapshots" ? (
          <SnapshotRowsFrame />
        ) : (
          <>
            <LoadingStatus label="sandboxGroupPages.loading" />
            <DetailRowsFrame rows={SANDBOX_DETAIL_ROWS} />
          </>
        )}
      </div>
    </div>
  );
}

/** The snapshot drawer while a snapshot outside the list loads. */
export function SnapshotDrawerSkeleton() {
  const { t } = useTranslations();
  return (
    <div data-skeleton-region="drawer-body">
      <div aria-hidden="true">
        <HeaderFrame titleRow="plain" subtitle="id" />
      </div>
      <div className="space-y-6 p-4">
        <section className="space-y-2">
          <h3
            className="relative w-fit text-sm font-semibold"
            aria-hidden="true"
          >
            <ReservedText>{t("sandboxGroupPages.tabDetails")}</ReservedText>
          </h3>
          <DetailRowsFrame
            rows={[
              { label: "sandboxGroupPages.detailType" },
              { label: "sandboxGroupPages.detailResourceConstraint" },
              { label: "sandboxGroupPages.colSourceSandbox" },
              { label: "sandboxGroupPages.colSize" },
              { label: "sandboxGroupPages.colCaptured" },
              { label: "sandboxGroupPages.detailLifecycle" },
              { label: "sandboxGroupPages.colExpires" },
            ]}
          />
        </section>
        <section className="space-y-2">
          <h3
            className="relative w-fit text-sm font-semibold"
            aria-hidden="true"
          >
            <ReservedText>{t("sandboxGroupPages.navSandboxes")}</ReservedText>
          </h3>
          <StartedRowsFrame />
        </section>
      </div>
    </div>
  );
}

const IMAGE_TABS = [
  { key: "versions", label: "sandboxGroupPages.tabVersions" },
  { key: "details", label: "sandboxGroupPages.tabDetails" },
  { key: "sandboxes", label: "sandboxGroupPages.navSandboxes" },
] as const satisfies readonly { key: ImageDrawerTab; label: MessageKey }[];

/** The image drawer while its versions load: header, tabs, the open tab. */
export function ImageDrawerSkeleton({ tab }: { tab: ImageDrawerTab }) {
  return (
    <div data-skeleton-region="drawer-body">
      <div aria-hidden="true">
        <HeaderFrame titleRow="plain" subtitle="source" />
        <TabsFrame value={tab} tabs={IMAGE_TABS} />
      </div>
      <div className="p-4">
        {tab !== "sandboxes" ? (
          <LoadingStatus label="sandboxGroupPages.loading" />
        ) : null}
        {tab === "versions" ? (
          <VersionRowsFrame />
        ) : tab === "details" ? (
          <DetailRowsFrame
            rows={[
              { label: "sandboxGroupPages.detailLatestAvailable" },
              { label: "sandboxGroupPages.detailImageSource" },
              { label: "sandboxGroupPages.detailImage" },
              { label: "sandboxGroupPages.detailDigest" },
              { label: "sandboxGroupPages.detailVersionId" },
            ]}
          />
        ) : (
          <StartedRowsFrame />
        )}
      </div>
    </div>
  );
}
