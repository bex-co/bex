import { Link } from "@tanstack/react-router";
import { Github } from "lucide-react";
import { Badge } from "@/common/components/ui/badge.tsx";
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

export interface ServiceDetailHeaderProps {
  service: ServiceView;
  /** The lifecycle action in flight for this service, or null. */
  pending: LifecycleAction | null;
  onRun: (action: LifecycleAction, service: ServiceView) => void;
}

/**
 * The service-detail header, mirroring Render's service-page banner (captured
 * live from dashboard.render.com): an uppercase service-type eyebrow, the name
 * with its instance-type chip, Manual Deploy + the lifecycle actions, and a
 * metadata stack of Service ID / source repo / live URL. bex has no Overview tab
 * (neither does Render — the service root lands on Events), so this header is
 * where the identity facts live: it also carries the bex-native ones the retired
 * Overview panel showed — instances, revision, age.
 *
 * Reuses the list's `ServiceRowActions` (and, via it, m4's confirm + poll-to-
 * converge) so the lifecycle verbs behave identically on the list and here.
 */
export function ServiceDetailHeader({
  service,
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
          <ServiceStatusBadge service={service} />
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
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ManualDeployButton service={service} pending={pending !== null} />
          <ServiceRowActions
            service={service}
            pending={pending}
            onRun={onRun}
            hideRestart
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
    </div>
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
