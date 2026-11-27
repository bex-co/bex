import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  ArrowRight,
  Database,
  FileText,
  Info,
  Loader2,
  TriangleAlert,
} from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { LocalDateTime } from "@/common/components/local-time";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { cn } from "@/common/lib/utils/utils";
import { RESOURCE_POLL_INTERVAL_MS } from "@/common/lib/polling";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { placementErrorMessage } from "@/features/environments/lib/placement";
import { DatastoreLogViewer } from "@/features/logs/components/datastore-log-viewer";
import { LIVE_TAIL, type LogRange } from "@/features/logs/lib/log-range";
import { parseCustomRange } from "@/features/metrics/lib/range";
import {
  DatabaseRecoveryInfoDocument,
  RecoverDatabaseDocument,
} from "@/graphql/definitions";
import { DatabaseUpgradeFailedBanner } from "@/features/databases/components/database-upgrade-notices";
import { useDatabaseExtensions } from "@/features/databases/hooks/use-database-extensions";
import { useUpdateDatabaseVersion } from "@/features/databases/hooks/use-update-database-version";
import { deriveStatus } from "@/features/databases/lib/status";
import {
  UPGRADE_FAILED,
  UPGRADE_IN_PROGRESS,
  UPGRADE_SUCCEEDED,
  VERSION_UPGRADE_DOCS_HREF,
  isUpgradeClone,
  rememberUpgradeClone,
  releaseNotesHref,
  upgradeAvailable,
  upgradeCloneName,
  upgradeFinished,
  upgradeRunning,
  useUpgradeClonePair,
} from "@/features/databases/lib/version-upgrade";
import type {
  DatabaseDetailView,
  DatabaseUpgradeRunView,
} from "@/features/databases/types";

// A Postgres database's guided version-upgrade page (w3/m676), Render's
// VersionUpgrade page section for section (docs/render-artifacts/
// postgres-version-upgrade.md § The guided page): the current version, the
// steps to the latest major (a clone first, then this database), what the
// upgrade changes for good, the extensions that block it, its status and
// logs while it runs, and the last attempt once it ends.

/** How long after its start a finished run's logs window reaches. Render
 *  says an upgrade "might take up to one hour". */
const RUN_LOGS_WINDOW_MS = 2 * 60 * 60 * 1000;

export function DatabaseVersionUpgrade({
  database,
}: {
  database: DatabaseDetailView;
}) {
  const { t } = useTranslations();
  const run = database.lastUpgradeRun ?? null;
  // The one major an in-place upgrade targets, the latest bex runs.
  const target = database.versionUpgradeAvailable ?? null;
  const showSteps =
    target !== null && (upgradeAvailable(database) || upgradeRunning(run));
  useRunResultToast(run);

  return (
    <div className="space-y-6">
      <DatabaseUpgradeFailedBanner database={database} />
      <Card>
        <CardHeader>
          <CardTitle>{t("databases.versionUpgradeHeading")}</CardTitle>
          <CardDescription>
            <Badge variant="secondary" className="mt-1">
              <Database aria-hidden="true" />
              {t("databases.versionUpgradeCurrent", {
                version: database.version ?? "—",
              })}
            </Badge>
          </CardDescription>
        </CardHeader>
        <CardContent>
          {showSteps ? (
            <UpgradeSteps database={database} target={target} run={run} />
          ) : (
            <p className="text-sm">{t("databases.versionUpgradeLatest")}</p>
          )}
        </CardContent>
      </Card>
      {run && upgradeFinished(run) ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("databases.versionUpgradeLastAttempt")}</CardTitle>
          </CardHeader>
          <CardContent>
            <UpgradeRunStatus databaseId={database.id} run={run} />
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}

/**
 * Render's result toasts: an upgrade that ends while the page is open says
 * how it ended. A run already over when the page opened says nothing; its
 * result is the banner and Last Upgrade Attempt.
 */
function useRunResultToast(run: DatabaseUpgradeRunView | null) {
  const { t } = useTranslations();
  const seen = useRef<{ runId: string; state: string } | null>(null);
  useEffect(() => {
    const previous = seen.current;
    seen.current = run ? { runId: run.runId, state: run.state } : null;
    if (
      !run ||
      previous?.runId !== run.runId ||
      previous.state !== UPGRADE_IN_PROGRESS
    ) {
      return;
    }
    if (run.state === UPGRADE_SUCCEEDED) {
      toast.success(t("databases.versionUpgradeSucceededTitle"), {
        description: t("databases.versionUpgradeSucceededBody", {
          version: run.toVersion,
        }),
      });
    } else if (run.state === UPGRADE_FAILED) {
      toast.error(t("databases.versionUpgradeFailedTitle"), {
        description: t("databases.versionUpgradeFailedBody"),
      });
    }
  }, [run, t]);
}

/**
 * Whether the database's point-in-time recovery has a window to restore
 * from, re-read every 30 seconds until it does, as Render's page waits on
 * its recovery status.
 */
function usePitrReady(id: string, enabled: boolean) {
  const { data, loading, startPolling, stopPolling } = useQuery(
    DatabaseRecoveryInfoDocument,
    {
      variables: { id },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skip: !enabled,
    },
  );
  const info = data?.databaseRecoveryInfo;
  const ready = Boolean(
    info?.enabled && info.earliestRecoveryTime && info.latestRecoveryTime,
  );
  useEffect(() => {
    if (!enabled || ready) {
      stopPolling();
      return;
    }
    // The baseline cadence, kept clear of the HTTP/3 idle timeout (Render
    // re-reads every 30 s).
    startPolling(RESOURCE_POLL_INTERVAL_MS);
    return stopPolling;
  }, [enabled, ready, startPolling, stopPolling]);
  return { ready, loading: enabled && loading && !data };
}

function UpgradeSteps({
  database,
  target,
  run,
}: {
  database: DatabaseDetailView;
  target: string;
  run: DatabaseUpgradeRunView | null;
}) {
  const { t } = useTranslations();
  const { extensions, loading: extensionsLoading } = useDatabaseExtensions(
    database.id,
  );
  const pitr = usePitrReady(database.id, database.backupsEnabled);
  const blocking = extensions
    .filter((e) => e.blocksUpgradeDatabase)
    .map((e) => e.name);
  const blocked = blocking.length > 0;
  // Render offers the clone way only where PITR can make one, and never on a
  // clone itself.
  const cloneStep = database.backupsEnabled && !isUpgradeClone(database.name);
  const [docsBefore, docsAfter] = aroundPlaceholder(
    t("databases.versionUpgradeStepsDocs", { docs: PLACEHOLDER_SENTINEL }),
  );
  const steps: { key: string; title: string; body: ReactNode }[] = [];
  if (cloneStep) {
    steps.push({
      key: "clone",
      title: t("databases.versionUpgradeCloneStep"),
      body: (
        <CloneStep
          database={database}
          pitrReady={pitr.ready}
          disabled={blocked || extensionsLoading || upgradeRunning(run)}
        />
      ),
    });
  }
  steps.push({
    key: "upgrade",
    title: t("databases.versionUpgradeThisStep"),
    body: (
      <UpgradeStep
        database={database}
        target={target}
        run={run}
        pitrReady={pitr.ready}
        disabled={blocked || extensionsLoading || pitr.loading}
      />
    ),
  });

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h3 className="font-semibold">
          {t("databases.versionUpgradeStepsTitle")}
        </h3>
        <p className="text-sm text-muted-foreground">
          {docsBefore}
          <a
            href={VERSION_UPGRADE_DOCS_HREF}
            target="_blank"
            rel="noreferrer"
            className="font-medium text-foreground underline underline-offset-2"
          >
            {t("databases.versionUpgradeDocsLink")}
          </a>
          {docsAfter}
        </p>
      </header>
      <Notice tone="neutral" title={t("databases.versionUpgradeChangesTitle")}>
        <ul className="mt-1 list-inside list-disc space-y-0.5">
          {database.backupsEnabled ? (
            <li>{t("databases.versionUpgradeChangePitr")}</li>
          ) : null}
          <li>
            {t("databases.versionUpgradeChangeNoDowngrade", {
              version: database.version ?? "—",
            })}
          </li>
        </ul>
      </Notice>
      {blocked ? (
        <Notice tone="info" title={t("databases.versionUpgradeBlockedTitle")}>
          <p>{t("databases.versionUpgradeBlockedBody")}</p>
          <ul className="mt-1 list-inside list-disc">
            {blocking.map((name) => (
              <li key={name}>
                <code>{name}</code>
              </li>
            ))}
          </ul>
        </Notice>
      ) : null}
      <ol aria-label={t("databases.versionUpgradeStepsLabel")}>
        {steps.map((step, index) => (
          <li key={step.key} className="group flex gap-4">
            <div className="flex flex-col items-center">
              <span
                aria-hidden="true"
                className="flex size-7 shrink-0 items-center justify-center rounded-full border text-sm font-medium"
              >
                {index + 1}
              </span>
              <span className="w-px flex-1 bg-border group-last:hidden" />
            </div>
            <div className="min-w-0 flex-1 space-y-2 pb-6 group-last:pb-0">
              <h4 className="pt-1 font-medium">{step.title}</h4>
              {step.body}
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}

function Notice({
  tone,
  title,
  children,
}: {
  tone: "neutral" | "info";
  title: string;
  children: ReactNode;
}) {
  const Icon = tone === "info" ? Info : TriangleAlert;
  return (
    <div
      className={cn(
        "flex items-start gap-3 rounded-lg border px-4 py-3 text-sm",
        tone === "info"
          ? "border-sky-500/40 bg-sky-500/10"
          : "border-amber-500/40 bg-amber-500/10",
      )}
    >
      <Icon
        aria-hidden="true"
        className={cn(
          "mt-0.5 size-4 shrink-0",
          tone === "info"
            ? "text-sky-600 dark:text-sky-400"
            : "text-amber-600 dark:text-amber-400",
        )}
      />
      <div className="min-w-0 space-y-1">
        <p className="font-medium">{title}</p>
        {children}
      </div>
    </div>
  );
}

/**
 * Render's first way: restore a point-in-time clone named
 * `<name>-upgrade-clone-<4>`, then upgrade the clone from its own page. The
 * pairing is kept in this browser, as Render keeps it, so the clone's banner
 * can name its source and this step can link the clone again.
 */
function CloneStep({
  database,
  pitrReady,
  disabled,
}: {
  database: DatabaseDetailView;
  pitrReady: boolean;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const pair = useUpgradeClonePair(database.id);
  const [created, setCreated] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { denied, reason } = useCapabilityGate("canCreate");
  const [recover, { loading }] = useMutation(RecoverDatabaseDocument);
  const cloneId = created ?? pair?.id ?? null;

  async function cloneDatabase() {
    setError(null);
    const name = upgradeCloneName(database.name);
    try {
      const result = await recover({ variables: { id: database.id, name } });
      const id = result.data?.recoverDatabase?.id;
      if (!id) throw new Error("recoverDatabase returned no id");
      setCreated(id);
      rememberUpgradeClone(
        { id, name },
        { id: database.id, name: database.name },
      );
      toast.success(t("databases.versionUpgradeCloneCreated", { name }));
    } catch (err) {
      setError(
        placementErrorMessage(err, t("databases.versionUpgradeCloneError"), t),
      );
    }
  }

  const tooltip =
    reason ?? (pitrReady ? undefined : t("databases.versionUpgradeClonePitr"));
  return (
    <>
      <p className="text-sm text-muted-foreground">
        {t("databases.versionUpgradeCloneBody")}
        <br />
        {t("databases.versionUpgradeCloneBilling")}
      </p>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
        <PermissionTooltip reason={tooltip}>
          <Button
            type="button"
            variant="outline"
            disabled={disabled || denied || !pitrReady || loading}
            onClick={() => void cloneDatabase()}
          >
            {loading ? <Loader2 className="animate-spin" /> : null}
            {t("databases.versionUpgradeCloneAction")}
          </Button>
        </PermissionTooltip>
        {cloneId ? (
          <Link
            to="/databases/$databaseId/version-upgrade"
            params={{ databaseId: cloneId }}
            className="inline-flex items-center gap-1 text-sm font-medium underline underline-offset-2"
          >
            {t("databases.versionUpgradeCloneView")}
            <ArrowRight aria-hidden="true" className="size-4" />
          </Link>
        ) : null}
      </div>
      {error ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </>
  );
}

/**
 * Render's second way: upgrade this database to the latest major, with
 * today's guarded verb. Render waits for PITR before it (a clone needs no
 * wait); bex's own guards answer inline, and while the upgrade runs its
 * status and logs show here.
 */
function UpgradeStep({
  database,
  target,
  run,
  pitrReady,
  disabled,
}: {
  database: DatabaseDetailView;
  target: string;
  run: DatabaseUpgradeRunView | null;
  pitrReady: boolean;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const { updateVersion, busy, error } = useUpdateDatabaseVersion();
  const { denied, reason } = useCapabilityGate("canCreate");
  const status = deriveStatus(database).key;
  const running = upgradeRunning(run) || status === "upgrading";
  // A database with no backups has no PITR to wait for, as Render's legacy
  // plans have none; bex's server then needs no completed backup either.
  const ready =
    status === "available" &&
    (pitrReady || !database.backupsEnabled || isUpgradeClone(database.name));
  const tooltip =
    reason ??
    (ready || running ? undefined : t("databases.versionUpgradeNotReady"));

  return (
    <>
      <p className="text-sm text-muted-foreground">
        {t("databases.versionUpgradeDowntime")}
        <br />
        {t("databases.versionUpgradeDowntimeDetail")}
      </p>
      <form
        className="space-y-2"
        onSubmit={(event) => {
          event.preventDefault();
          void updateVersion(database.id, target);
        }}
      >
        <PermissionTooltip reason={tooltip}>
          <Button
            type="submit"
            disabled={disabled || denied || !ready || busy || running}
          >
            {busy || running ? <Loader2 className="animate-spin" /> : null}
            {busy || running
              ? t("databases.versionUpgradeRunning")
              : t("databases.versionUpgradeConfirm", { version: target })}
          </Button>
        </PermissionTooltip>
        {error ? (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        ) : null}
      </form>
      <a
        href={releaseNotesHref(target)}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-1.5 text-sm font-medium underline underline-offset-2"
      >
        <FileText aria-hidden="true" className="size-4" />
        {t("databases.versionUpgradeReleaseNotes")}
      </a>
      {run && upgradeRunning(run) ? (
        <div className="pt-4">
          <UpgradeRunStatus databaseId={database.id} run={run} />
        </div>
      ) : null}
    </>
  );
}

type Translate = ReturnType<typeof useTranslations>["t"];

/** A run state's badge: Render's state words, lower-cased. */
function stateBadge(
  state: string,
  t: Translate,
): { label: string; variant: "secondary" | "success" | "destructive" } {
  switch (state) {
    case UPGRADE_SUCCEEDED:
      return {
        label: t("databases.versionUpgradeStateSucceeded"),
        variant: "success",
      };
    case UPGRADE_FAILED:
      return {
        label: t("databases.versionUpgradeStateFailed"),
        variant: "destructive",
      };
    default:
      return {
        label: t("databases.versionUpgradeStateInProgress"),
        variant: "secondary",
      };
  }
}

/**
 * Render's "Upgrade status and logs": when the run started, its state, and
 * the database's logs from then on — live while it runs, its first two hours
 * once it has ended.
 */
export function UpgradeRunStatus({
  databaseId,
  run,
}: {
  databaseId: string;
  run: DatabaseUpgradeRunView;
}) {
  const { t } = useTranslations();
  const badge = stateBadge(run.state, t);
  const running = upgradeRunning(run);
  return (
    <section
      aria-label={t("databases.versionUpgradeStatusLabel")}
      className="space-y-4"
    >
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="font-semibold">
          <LocalDateTime value={run.startedAt} />
        </h3>
        <Badge variant={badge.variant}>
          {running ? <Loader2 className="animate-spin" /> : null}
          {badge.label}
        </Badge>
      </div>
      <UpgradeRunLogs
        key={`${run.runId}:${run.state}`}
        databaseId={databaseId}
        startedAt={run.startedAt}
        running={running}
      />
    </section>
  );
}

function UpgradeRunLogs({
  databaseId,
  startedAt,
  running,
}: {
  databaseId: string;
  startedAt: string;
  running: boolean;
}) {
  const { t } = useTranslations();
  const [range, setRange] = useState<LogRange>(() => {
    if (running) return LIVE_TAIL;
    const start = Date.parse(startedAt);
    return (
      parseCustomRange(
        startedAt,
        new Date(start + RUN_LOGS_WINDOW_MS).toISOString(),
      ) ?? LIVE_TAIL
    );
  });
  return (
    <DatastoreLogViewer
      kind="databases"
      resource={databaseId}
      range={range}
      onRangeChange={setRange}
      emptyBody={
        running
          ? t("databases.versionUpgradeLogsEmptyRunning")
          : t("databases.versionUpgradeLogsEmpty")
      }
    />
  );
}
