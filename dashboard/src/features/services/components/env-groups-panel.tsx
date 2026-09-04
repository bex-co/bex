import { useState } from "react";
import {
  Plus,
  ShieldAlert,
  AlertTriangle,
  Loader2,
  Link2,
  Link2Off,
} from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { PanelCenteredState } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useEnvGroups,
  useEnvGroupMutations,
  classifyEnvGroupError,
} from "@/features/env-groups/hooks/use-env-groups";
import type { EnvGroupView } from "@/features/env-groups/types";
import { NewEnvGroupDialog } from "@/features/env-groups/components/new-env-group-dialog";
import { useEnvVarKeys } from "@/features/services/hooks/use-env-vars";
import { useServer } from "@/features/services/hooks/use-server";

/**
 * The service Environment tab's Environment Groups section (Render dashboard
 * shape): lists all env groups, shows each group's env-var keys + secret-file
 * names (read-only), and links/unlinks the CURRENT service to a group — all over
 * bex-api's env-groups GraphQL. A group is a reusable bundle shared across
 * services, so the list is service-independent; only membership is per-service.
 */
export function EnvGroupsPanel({
  serviceId,
  createOpen: createOpenProp,
  onCreateOpenChange,
}: {
  serviceId: string;
  createOpen?: boolean;
  onCreateOpenChange?: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  const { groups, loading, error, refetch } = useEnvGroups();
  const { linkGroup, unlinkGroup, busy } = useEnvGroupMutations(refetch);
  const { service, loading: serviceLoading } = useServer(serviceId, {
    poll: false,
  });
  // The service's own env-var keys, to flag linked-group keys the service
  // overrides at runtime (w6/067; last envFrom source wins — the service's).
  // Same query the environment editor on this page runs, so Apollo shares it.
  // Display-only: fail-open to "no markers" while loading or on error.
  const { keys: serviceEnvKeys } = useEnvVarKeys(serviceId);
  const serviceKeys = new Set(serviceEnvKeys.map((entry) => entry.key));
  const [internalCreateOpen, setInternalCreateOpen] = useState(false);
  const createOpen = createOpenProp ?? internalCreateOpen;
  const setCreateOpen = onCreateOpenChange ?? setInternalCreateOpen;

  const errorKind = classifyEnvGroupError(error);
  const initialLoading = loading && groups.length === 0 && !error;
  const gated = errorKind === "unavailable" || errorKind === "forbidden";
  const linked = groups.filter((group) =>
    group.serviceLinks.includes(serviceId),
  );
  const available = groups.filter(
    (group) => !group.serviceLinks.includes(serviceId),
  );

  return (
    <Card>
      <CardHeader className="grid-cols-1 grid-rows-none sm:grid-cols-[minmax(0,1fr)_auto] sm:grid-rows-[auto_auto]">
        <CardTitle>{t("services.envGroupsLinkedTitle")}</CardTitle>
        <CardDescription>{t("services.envGroupsDescription")}</CardDescription>
        <CardAction className="col-start-1 row-start-3 mt-2 justify-self-start sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end">
          <Button
            variant="outline"
            size="sm"
            disabled={gated || busy}
            onClick={() => setCreateOpen(true)}
          >
            <Plus /> {t("services.envGroupCreate")}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} />
        ) : initialLoading ? (
          <ListSkeleton />
        ) : (
          <div className="space-y-6">
            <section className="space-y-2">
              <h3 className="text-sm font-medium">
                {t("services.envGroupsLinkedCount", { count: linked.length })}
              </h3>
              {linked.length === 0 ? (
                <p className="text-muted-foreground rounded-md border border-dashed p-4 text-sm">
                  {t("services.envGroupsNoneLinked")}
                </p>
              ) : (
                <ul className="divide-y rounded-md border px-4">
                  {linked.map((group) => (
                    <EnvGroupItem
                      key={group.id}
                      group={group}
                      serviceId={serviceId}
                      serviceKeys={serviceKeys}
                      onLink={linkGroup}
                      onUnlink={unlinkGroup}
                      busy={busy}
                    />
                  ))}
                </ul>
              )}
            </section>
            <section className="space-y-2">
              <h3 className="text-sm font-medium">
                {t("services.envGroupsAvailableCount", {
                  count: available.length,
                })}
              </h3>
              {available.length === 0 ? (
                <p className="text-muted-foreground rounded-md border border-dashed p-4 text-sm">
                  {groups.length === 0
                    ? t("services.envGroupsNoneAvailableCreate")
                    : t("services.envGroupsNoneAvailable")}
                </p>
              ) : (
                <ul className="divide-y rounded-md border px-4">
                  {available.map((group) => (
                    <EnvGroupItem
                      key={group.id}
                      group={group}
                      serviceId={serviceId}
                      serviceKeys={serviceKeys}
                      onLink={linkGroup}
                      onUnlink={unlinkGroup}
                      busy={busy}
                    />
                  ))}
                </ul>
              )}
            </section>
          </div>
        )}
      </CardContent>
      <NewEnvGroupDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => void refetch()}
        refetch={refetch}
        services={service ? [service] : []}
        servicesLoading={serviceLoading}
        initialServiceIds={[serviceId]}
      />
    </Card>
  );
}

/** One env-group list item: name, contents preview, and service-local link state. */
function EnvGroupItem({
  group,
  serviceId,
  serviceKeys,
  onLink,
  onUnlink,
  busy,
}: {
  group: EnvGroupView;
  serviceId: string;
  /** The service's own env-var keys — a linked group's matching key is
   *  overridden at runtime (service wins), so it's marked here (w6/067). */
  serviceKeys: ReadonlySet<string>;
  onLink: (id: string, serviceId: string) => Promise<boolean>;
  onUnlink: (id: string, serviceId: string) => Promise<boolean>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const linked = group.serviceLinks.includes(serviceId);
  // Only a *linked* group actually feeds the service, so only there can a
  // duplicate key be shadowed by the service's own variable.
  const overridden = linked
    ? group.envVarKeys.filter((key) => serviceKeys.has(key))
    : [];

  return (
    <li className="flex flex-col items-stretch gap-4 py-4 first:pt-4 last:pb-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0 space-y-2">
        <div className="flex items-center gap-2">
          <Button
            asChild
            variant="link"
            className="h-auto min-w-0 justify-start p-0 font-medium"
          >
            <a href={`/env-groups/${group.id}`}>
              <span className="break-all">{group.name}</span>
            </a>
          </Button>
          {linked && (
            <Badge variant="success">{t("services.envGroupLinked")}</Badge>
          )}
        </div>
        {group.envVarKeys.length === 0 && group.secretFileNames.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {t("services.envGroupEmptyContents")}
          </p>
        ) : (
          <>
            <div className="flex flex-wrap gap-1">
              {group.envVarKeys.map((key) =>
                overridden.includes(key) ? (
                  <Badge
                    key={`k-${key}`}
                    variant="outline"
                    className="text-muted-foreground font-mono"
                    title={t("services.envGroupKeyOverridden", { key })}
                  >
                    <s>{key}</s>
                  </Badge>
                ) : (
                  <Badge
                    key={`k-${key}`}
                    variant="secondary"
                    className="font-mono"
                  >
                    {key}
                  </Badge>
                ),
              )}
              {group.secretFileNames.map((name) => (
                <Badge
                  key={`f-${name}`}
                  variant="outline"
                  className="font-mono"
                >
                  {name}
                </Badge>
              ))}
            </div>
            {overridden.length > 0 && (
              <p className="text-muted-foreground text-xs">
                {t("services.envGroupOverriddenNote")}
              </p>
            )}
          </>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-1 self-end sm:self-auto">
        <Button
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={() =>
            void (linked
              ? onUnlink(group.id, serviceId)
              : onLink(group.id, serviceId))
          }
        >
          {busy ? (
            <Loader2 className="animate-spin" />
          ) : linked ? (
            <Link2Off />
          ) : (
            <Link2 />
          )}
          {linked ? t("services.envGroupUnlink") : t("services.envGroupLink")}
        </Button>
      </div>
    </li>
  );
}

function ListSkeleton() {
  return (
    <div className="space-y-4">
      {[0, 1].map((i) => (
        <div key={i} className="flex items-center justify-between gap-4">
          <div className="flex-1 space-y-2">
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-4 w-64" />
          </div>
          <Skeleton className="h-8 w-20" />
        </div>
      ))}
    </div>
  );
}

/** The unavailable (503) / forbidden (403) / generic error states. */
function StatePanel({
  kind,
}: {
  kind: "unavailable" | "forbidden" | "generic";
}) {
  const { t } = useTranslations();
  const copy = {
    unavailable: {
      icon: <AlertTriangle />,
      title: t("services.envGroupsUnavailableTitle"),
      body: t("services.envGroupsUnavailableBody"),
    },
    forbidden: {
      icon: <ShieldAlert />,
      title: t("services.envGroupsForbiddenTitle"),
      body: t("services.envGroupsForbiddenBody"),
    },
    generic: {
      icon: <AlertTriangle />,
      title: t("services.envGroupsErrorTitle"),
      body: t("services.envGroupsErrorBody"),
    },
  }[kind];
  return (
    <PanelCenteredState icon={copy.icon} title={copy.title} body={copy.body} />
  );
}
