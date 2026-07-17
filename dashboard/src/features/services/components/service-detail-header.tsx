import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  ChevronDown,
  Github,
  KeyRound,
  Terminal,
} from "lucide-react";
import { Badge } from "@/common/components/ui/badge.tsx";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert.tsx";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { ManualDeployButton } from "@/features/services/components/manual-deploy-button";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import { formatRelativeAge } from "@/features/services/lib/format";
import { formatRepoLabel, repoBrowseUrl } from "@/features/services/lib/repo";
import {
  deriveServiceType,
  isCron,
  SERVICE_TYPE_ICON,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";
import { isSleeping } from "@/features/services/lib/status";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import type { RunServiceAction } from "@/features/services/hooks/use-service-lifecycle";
import { useRegistryCredentials } from "@/features/registry-credentials/hooks/use-registry-credentials";
import {
  deployStatusKey,
  deployStatusVariant,
} from "@/features/deploys/lib/deploy-status";
import type { LatestDeploySummary } from "@/features/deploys/hooks/use-latest-deploy";

export interface ServiceDetailHeaderProps {
  service: ServiceView;
  /** Newest control-plane deploy, named separately from operator/App phase. */
  latestDeploy?: LatestDeploySummary | null;
  /** The lifecycle action in flight for this service, or null. */
  pending: LifecycleAction | null;
  onRun: RunServiceAction;
}

/**
 * The service-detail header, mirroring Render's service-page banner (captured
 * live from dashboard.render.com): an uppercase service-type eyebrow, the name
 * with its instance-type chip, Manual Deploy + the lifecycle actions, and a
 * metadata stack of Service ID / source repo / live URL. bex has no Overview tab
 * (neither does Render — the service root lands on Deploys), so this header is
 * where the identity facts live: it also carries the bex-native ones the retired
 * Overview panel showed — instances, revision, age.
 *
 * Reuses the list's `ServiceRowActions` (and, via it, m4's confirm + poll-to-
 * converge) so the lifecycle verbs behave identically on the list and here.
 */
export function ServiceDetailHeader({
  service,
  latestDeploy,
  pending,
  onRun,
}: ServiceDetailHeaderProps) {
  const { t } = useTranslations();
  const { byID } = useInstanceTypes();
  const typeKey = deriveServiceType(service.type);
  const TypeIcon = SERVICE_TYPE_ICON[typeKey];
  const instanceType = byID(service.plan);
  const repoUrl = service.repo
    ? repoBrowseUrl(service.repo, service.branch)
    : null;

  return (
    <div className="space-y-3 border-b px-4 py-4 sm:px-6">
      <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium tracking-wide uppercase">
        <TypeIcon className="size-3.5" />
        {t(SERVICE_TYPE_LABEL[typeKey])}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h1 className="truncate text-xl font-semibold">{service.name}</h1>
          <span className="text-xs font-medium text-muted-foreground">
            {t("services.headerServicePhase")}
          </span>
          <ServiceStatusBadge service={service} />
          {latestDeploy ? (
            <Link
              to="/services/$serviceId/deploys/$deployId"
              params={{ serviceId: service.id, deployId: latestDeploy.id }}
              aria-label={`${t("services.headerLatestDeploy")}: ${t(
                deployStatusKey(latestDeploy.status) as Parameters<typeof t>[0],
              )}`}
              className="flex items-center gap-1.5 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <span className="text-xs font-medium text-muted-foreground">
                {t("services.headerLatestDeploy")}
              </span>
              <Badge variant={deployStatusVariant(latestDeploy.status)}>
                {t(
                  deployStatusKey(latestDeploy.status) as Parameters<
                    typeof t
                  >[0],
                )}
              </Badge>
            </Link>
          ) : null}
          {instanceType ? (
            <Link
              to="/services/$serviceId/plan"
              params={{ serviceId: service.id }}
            >
              <Badge variant="outline" className="hover:bg-accent">
                {instanceType.name}
              </Badge>
            </Link>
          ) : null}
          {service.runtime?.trim() ? (
            <Badge variant="secondary" className="gap-1">
              <span className="text-muted-foreground">
                {t("services.headerRuntime")}
              </span>
              <span className="capitalize">{service.runtime.trim()}</span>
            </Badge>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ServiceConnectButton service={service} />
          <ManualDeployButton service={service} pending={pending !== null} />
          <ServiceRowActions
            service={service}
            pending={pending}
            onRun={onRun}
            hideRestart
            hideSuspend
          />
        </div>
      </div>

      <div className="space-y-1 text-sm">
        <div className="text-muted-foreground flex items-center gap-1.5">
          <span>{t("services.headerServiceId")}</span>
          <span className="text-foreground font-mono text-xs">
            {service.id}
          </span>
          <CopyButton
            value={service.id}
            label={t("services.headerCopyServiceId")}
            successText={t("services.headerCopied")}
            errorText={t("services.headerCopyError")}
          />
        </div>

        {service.repo ? (
          <div className="text-muted-foreground flex min-w-0 items-center gap-1.5">
            <Github className="size-3.5 shrink-0" />
            {repoUrl ? (
              <a
                href={repoUrl}
                target="_blank"
                rel="noreferrer"
                className="text-foreground truncate hover:underline"
              >
                {formatRepoLabel(service.repo)}
              </a>
            ) : (
              <span className="text-foreground truncate">
                {formatRepoLabel(service.repo)}
              </span>
            )}
            {service.branch ? (
              <Badge variant="secondary" className="font-mono text-xs">
                {service.branch}
              </Badge>
            ) : null}
          </div>
        ) : null}

        {service.registryCredentialId ? (
          <RegistryCredentialFact id={service.registryCredentialId} />
        ) : null}

        {isCron(service) ? (
          <div className="text-muted-foreground flex items-center gap-1.5">
            <span>{t("services.headerSchedule")}</span>
            <span className="text-foreground font-mono text-xs">
              {service.schedule || "—"}
            </span>
          </div>
        ) : service.url ? (
          <div className="flex min-w-0 items-center gap-1.5">
            <a
              href={service.url}
              target="_blank"
              rel="noreferrer"
              className="text-primary truncate hover:underline"
            >
              {service.url}
            </a>
            <CopyButton
              value={service.url}
              label={t("services.headerCopyUrl")}
              successText={t("services.headerCopied")}
              errorText={t("services.headerCopyError")}
            />
          </div>
        ) : null}
      </div>

      <HeaderFacts service={service} />

      {isSleeping(service) ? (
        <p className="text-muted-foreground text-sm">
          {t("services.statusSleepingHint")}
        </p>
      ) : null}

      {service.maintenanceMode?.enabled ? (
        <Alert variant="destructive">
          <AlertTriangle />
          <AlertTitle>{t("services.maintenanceModeBannerTitle")}</AlertTitle>
          <AlertDescription>
            {t("services.maintenanceModeBannerBody")}
          </AlertDescription>
        </Alert>
      ) : null}
    </div>
  );
}

function RegistryCredentialFact({ id }: { id: string }) {
  const { t } = useTranslations();
  const { credentials } = useRegistryCredentials();
  const credential = credentials.find((item) => item.id === id);

  return (
    <div className="text-muted-foreground flex min-w-0 items-center gap-1.5">
      <KeyRound className="size-3.5 shrink-0" />
      <span>{t("services.headerRegistryCredential")}</span>
      <Link
        to="/settings"
        hash="registry-credentials"
        className="text-foreground truncate hover:underline"
      >
        {credential?.name || id}
      </Link>
    </div>
  );
}

function ServiceConnectButton({ service }: { service: ServiceView }) {
  const { t } = useTranslations();
  const command = service.sshAddress ? `ssh ${service.sshAddress}` : "";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          {t("services.connect")}
          <ChevronDown className="size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80 p-3">
        <DropdownMenuLabel className="flex items-center gap-2 px-0 pt-0">
          <Terminal className="size-4" />
          {t("services.connectSSH")}
        </DropdownMenuLabel>
        {command ? (
          <div className="bg-muted flex min-w-0 items-center gap-1 rounded-md py-1 pr-1 pl-2">
            <code className="min-w-0 flex-1 truncate text-xs" title={command}>
              {command}
            </code>
            <CopyButton
              value={command}
              label={t("services.sshCopy")}
              successText={t("services.sshCopied")}
              errorText={t("services.sshCopyError")}
            />
          </div>
        ) : (
          <p
            className="text-muted-foreground text-xs"
            title={t("services.sshUnavailableHint")}
          >
            {t("services.sshUnavailable")}
          </p>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * The bex-native facts the retired Overview tab used to list — instances, active
 * revision, age — as one muted, dot-separated line under Render's metadata
 * stack. Phase and suspension aren't repeated: the status badge beside the name
 * already encodes both.
 */
function HeaderFacts({ service }: { service: ServiceView }) {
  const { t } = useTranslations();

  const facts: { label: string; value: string }[] = [
    { label: t("services.colSlug"), value: service.slug || "—" },
    {
      label: t("services.colInstances"),
      value: service.replicas != null ? String(service.replicas) : "—",
    },
    { label: t("services.colRevision"), value: service.revision || "—" },
    {
      label: t("services.colCreated"),
      value: formatRelativeAge(service.createdAt),
    },
  ];

  return (
    <dl className="text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
      {facts.map((fact, i) => (
        <div key={fact.label} className="flex items-center gap-1.5">
          {i > 0 ? <span aria-hidden="true">·</span> : null}
          <dt>{fact.label}</dt>
          <dd className="text-foreground font-medium tabular-nums">
            {fact.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/** Placeholder header shown while `server(id)` is still loading. */
export function ServiceDetailHeaderSkeleton({ name }: { name: string }) {
  return (
    <div className="space-y-3 border-b px-4 py-4 sm:px-6">
      <Skeleton className="h-3 w-24" />
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-muted-foreground truncate text-xl font-semibold">
          {name}
        </h1>
        <Skeleton className="size-8 rounded-md" />
      </div>
      <div className="space-y-2">
        <Skeleton className="h-4 w-64" />
        <Skeleton className="h-4 w-48" />
      </div>
    </div>
  );
}
