import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  ChevronDown,
  Github,
  KeyRound,
  Network,
  SquareTerminal,
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
import { RelativeAge, RelativeUntil } from "@/common/components/relative-time";
import { useTranslations } from "@/common/hooks/use-translations";
import { AddSshKeyCta } from "@/features/ssh-keys/components/add-ssh-key-cta";
import { RequiresSshKey } from "@/features/ssh-keys/components/requires-ssh-key";
import { ManualDeployButton } from "@/features/services/components/manual-deploy-button";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import { formatRepoLabel, repoBrowseUrl } from "@/features/services/lib/repo";
import {
  deriveServiceType,
  isCron,
  isPrivateService,
  isStaticSite,
  SERVICE_TYPE_ICON,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";
import { serviceBaseForType } from "@/features/services/lib/service-base";
import { isSleeping } from "@/features/services/lib/status";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
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
 * The action controls are Connect (SSH) and Manual Deploy (deploy/restart); the
 * header carries no "•••" lifecycle menu — restart lives in the Manual Deploy
 * dropdown (w2/m30) and Suspend/Resume moved to the Settings page.
 */
export function ServiceDetailHeader({
  service,
  latestDeploy,
  pending,
}: ServiceDetailHeaderProps) {
  const { t } = useTranslations();
  const { byID } = useInstanceTypes();
  const typeKey = deriveServiceType(service.type);
  const TypeIcon = SERVICE_TYPE_ICON[typeKey];
  // Header links point at the service's canonical base (a static_site lives
  // under /static, Render parity w5/m57). The Plan chip below stays on
  // /services — plan has no /static route and renders for compute types only
  // (a static_site is gated out: its "Free" chip would link to a nonexistent
  // /static/<id>/plan and 404, and Render's static header has no plan concept).
  const base = serviceBaseForType(service.type);
  const instanceType = byID(service.plan);
  const repoUrl = service.repo
    ? repoBrowseUrl(service.repo, service.branch)
    : null;

  return (
    <div className="space-y-2 border-b px-4 py-3 sm:px-6">
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
              to={`${base}/$serviceId/deploys/$deployId`}
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
          {instanceType && !isStaticSite(service) ? (
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
          {/* A static_site has no running instance to SSH into — Render's
              static header carries only Manual Deploy (w5/m48/t001). */}
          {!isStaticSite(service) && <ServiceConnectButton service={service} />}
          <ManualDeployButton service={service} pending={pending !== null} />
        </div>
      </div>

      <div className="space-y-0.5 text-sm">
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
          <>
            <div className="text-muted-foreground flex items-center gap-1.5">
              <span>{t("services.headerSchedule")}</span>
              <span className="text-foreground font-mono text-xs">
                {service.schedule || "—"}
              </span>
            </div>
            {service.lastSuccessfulRunAt ? (
              <div className="text-muted-foreground flex items-center gap-1.5">
                <span>{t("services.headerLastRun")}</span>
                <RelativeAge
                  value={service.lastSuccessfulRunAt}
                  className="text-foreground text-xs"
                />
              </div>
            ) : null}
            {service.nextRunAt ? (
              <div className="text-muted-foreground flex items-center gap-1.5">
                <span>{t("services.headerNextRun")}</span>
                <RelativeUntil
                  value={service.nextRunAt}
                  className="text-foreground text-xs"
                />
              </div>
            ) : null}
          </>
        ) : isPrivateService(service) && service.internalAddress ? (
          // A private service has no public URL — Render's header shows its
          // Service Address (the private-network `<slug>:<port>`) as copyable
          // text, never a link (the old cluster-internal href was dead from a
          // browser). ADR041 D4, w9/m58.
          <div className="flex min-w-0 items-center gap-1.5">
            <span className="text-muted-foreground">
              {t("services.headerServiceAddress")}
            </span>
            <span className="text-foreground truncate font-mono text-xs">
              {service.internalAddress}
            </span>
            <CopyButton
              value={service.internalAddress}
              label={t("services.internalCopy")}
              successText={t("services.internalCopied")}
              errorText={t("services.internalCopyError")}
            />
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
        ) : service.publicRoutingNotice ? (
          // No url, and the platform knows why (w7/m79). This slot used to
          // render nothing at all, which is how a service that would never be
          // publicly reachable looked identical to one still starting up.
          // The text is the operator's own diagnosis, rendered raw like a
          // deploy's failureReason — it names the missing piece and the fix.
          <p className="text-muted-foreground text-xs">
            {service.publicRoutingNotice}
          </p>
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
        {service.internalAddress ? (
          // Render's Connect → Internal tab: the private-network
          // `<slug>:<port>` sibling services dial (ADR041 D4, w9/m58).
          <>
            <DropdownMenuLabel className="flex items-center gap-2 px-0 pt-0">
              <Network className="size-4" />
              {t("services.connectInternal")}
            </DropdownMenuLabel>
            <ConnectCodeRow
              className="mb-2"
              value={service.internalAddress}
              copyLabel={t("services.internalCopy")}
              copiedText={t("services.internalCopied")}
              errorText={t("services.internalCopyError")}
            />
          </>
        ) : null}
        <DropdownMenuLabel className="flex items-center gap-2 px-0 pt-0">
          <Terminal className="size-4" />
          {t("services.connectSSH")}
        </DropdownMenuLabel>
        {command ? (
          // Gate the doomed `ssh …` command behind key registration (w2/m66).
          // With no key it fails off-surface; swap it for an add-key CTA — and,
          // since a paid running service has the in-browser Web Shell, offer that
          // as a second, zero-setup door (session-auth, needs no SSH key).
          <RequiresSshKey
            surface="service-ssh"
            fallback={
              <AddSshKeyCta
                surface="service-ssh"
                returnTo={`/services/${service.id}`}
                secondaryAction={
                  <Button
                    asChild
                    variant="outline"
                    size="sm"
                    className="w-full"
                  >
                    <Link
                      to="/services/$serviceId/shell"
                      params={{ serviceId: service.id }}
                    >
                      <SquareTerminal className="size-4" />
                      {t("services.openBrowserTerminal")}
                    </Link>
                  </Button>
                }
              />
            }
          >
            <ConnectCodeRow
              value={command}
              copyLabel={t("services.sshCopy")}
              copiedText={t("services.sshCopied")}
              errorText={t("services.sshCopyError")}
            />
          </RequiresSshKey>
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

/** One copyable code row in the Connect menu — shared by Internal and SSH. */
function ConnectCodeRow({
  value,
  copyLabel,
  copiedText,
  errorText,
  className,
}: {
  value: string;
  copyLabel: string;
  copiedText: string;
  errorText: string;
  className?: string;
}) {
  return (
    <div
      className={`bg-muted flex min-w-0 items-center gap-1 rounded-md py-1 pr-1 pl-2 ${className ?? ""}`}
    >
      <code className="min-w-0 flex-1 truncate text-xs" title={value}>
        {value}
      </code>
      <CopyButton
        value={value}
        label={copyLabel}
        successText={copiedText}
        errorText={errorText}
      />
    </div>
  );
}

/**
 * The bex-native facts the retired Overview tab used to list — instances, active
 * revision, age — as one muted, dot-separated line under Render's metadata
 * stack. Phase and suspension aren't repeated: the status badge beside the name
 * already encodes both. A static_site has no runtime instances (it serves from
 * the object store, docs/ADR029-static-sites.md), so its line omits Instances
 * (w5/m48/t001 — Render shows no instance concept for static sites).
 */
function HeaderFacts({ service }: { service: ServiceView }) {
  const { t } = useTranslations();

  const facts: { label: string; value: ReactNode }[] = [
    { label: t("services.colSlug"), value: service.slug || "—" },
    ...(isStaticSite(service)
      ? []
      : [
          {
            label: t("services.colInstances"),
            value: service.replicas != null ? String(service.replicas) : "—",
          },
        ]),
    { label: t("services.colRevision"), value: service.revision || "—" },
    {
      label: t("services.colCreated"),
      value: <RelativeAge value={service.createdAt} />,
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
export function ServiceDetailHeaderSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="space-y-2 border-b px-4 py-3 sm:px-6"
      data-skeleton-region="service-header"
    >
      <Skeleton className="h-4 w-24" />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Skeleton className="h-6 w-48 max-w-[50vw]" />
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-5 w-16 rounded-full" />
          <Skeleton className="h-5 w-20 rounded-full" />
        </div>
        <div className="flex items-center gap-2">
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-8 w-28" />
        </div>
      </div>
      <div className="space-y-0.5">
        <div className="flex h-7 items-center gap-1.5">
          <Skeleton className="h-4 w-64" />
          <Skeleton className="size-7" />
        </div>
        <div className="flex h-7 items-center gap-1.5">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-5 w-20 rounded-full" />
        </div>
        <div className="flex h-7 items-center gap-1.5">
          <Skeleton className="h-4 w-56" />
          <Skeleton className="size-7" />
        </div>
      </div>
      <div className="flex h-9 flex-wrap gap-x-4 gap-y-1 sm:h-4">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-4 w-16" />
      </div>
    </div>
  );
}
