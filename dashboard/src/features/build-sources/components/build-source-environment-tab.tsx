import { useEffect, useMemo } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  AlertTriangle,
  Link2,
  Link2Off,
  Loader2,
  RotateCw,
  ShieldAlert,
} from "lucide-react";
import { PanelCenteredState } from "@/common/components/panel-states";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import type { RefusalKind } from "@/common/lib/graphql-error";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { isAdminOnlyRefusal } from "@/features/capabilities/lib/admin-only";
import {
  useEnvGroupMutations,
  useEnvGroups,
} from "@/features/env-groups/hooks/use-env-groups";
import type { EnvGroupView } from "@/features/env-groups/types";
import {
  ENV_CARD_DESCRIPTION_CLASS,
  ENV_CARD_HEADER_CLASS,
} from "@/features/services/components/env-rows-skeleton";
import { EnvironmentEditor } from "@/features/services/components/service-environment-editor";
import { useEnvVarKeys } from "@/features/services/hooks/use-env-vars";
import { useEnvironmentDraftSave } from "@/features/services/hooks/use-environment-draft-save";
import { useSecretFileNames } from "@/features/services/hooks/use-secret-files";
import { classifySecretStoreError } from "@/features/services/lib/secret-store";
import { useBuildSourceDetailContext } from "@/features/build-sources/components/build-source-detail-context";
import {
  useRevealBuildSourceEnvVar,
  useRevealBuildSourceSecretFile,
} from "@/features/build-sources/hooks/use-build-source-env";

/**
 * Save-only Build-time Environment tab (`/build-sources/$buildSourceId/environment`,
 * `w2/m614/t005`): git Build Sources only. Image-backed Build Sources redirect
 * cleanly back to `/build-sources/$buildSourceId`.
 */
export function BuildSourceEnvironmentTab() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { buildSource } = useBuildSourceDetailContext();
  const isImage = buildSource.kind === "image";

  useEffect(() => {
    if (!isImage) return;
    void navigate({
      to: "/build-sources/$buildSourceId",
      params: { buildSourceId: buildSource.id },
      replace: true,
    });
  }, [buildSource.id, isImage, navigate]);

  if (isImage) {
    return (
      <Card data-region="build-environment" className="py-6">
        <CardContent className="space-y-2 px-4 text-center">
          <p className="text-sm font-medium">
            {t("buildSources.envImageUnsupportedTitle")}
          </p>
          <p className="text-muted-foreground text-sm">
            {t("buildSources.envImageUnsupportedBody")}
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <GitBuildSourceEnvironment
      buildSourceId={buildSource.id}
      dockerBuild={buildSource.git?.runtime === "docker"}
    />
  );
}

function GitBuildSourceEnvironment({
  buildSourceId,
  dockerBuild,
}: {
  buildSourceId: string;
  dockerBuild: boolean;
}) {
  const { t } = useTranslations();
  const env = useEnvVarKeys(buildSourceId);
  const files = useSecretFileNames(buildSourceId);
  const revealEnv = useRevealBuildSourceEnvVar(buildSourceId);
  const revealFile = useRevealBuildSourceSecretFile(buildSourceId);
  const { save, saving } = useEnvironmentDraftSave();

  return (
    <div data-region="build-environment" className="space-y-6">
      <p className="text-muted-foreground text-sm">
        {t("buildSources.envSaveOnlyNote")}
      </p>

      <EnvironmentEditor
        key={buildSourceId}
        headingLevel="h3"
        resourceId={buildSourceId}
        saveOnly
        copy={{
          envDescription: t("buildSources.envVarsDescription"),
          secretFilesDescription: t("buildSources.secretFilesDescription"),
          saveOnlyLabel: t("buildSources.settingsSaveButton"),
          ...(dockerBuild
            ? {
                secretFileDialogDescription: t(
                  "services.secretFileContentDialogDescriptionDocker",
                ),
              }
            : {}),
        }}
        envKeys={env.keys}
        envKeysPending={env.loading}
        secretFileNames={files.names}
        loading={
          (env.loading && env.keys.length === 0) ||
          (files.loading && files.names.length === 0)
        }
        errorKind={
          classifySecretStoreError(env.error) ??
          classifySecretStoreError(files.error)
        }
        refusalBody={
          isAdminOnlyRefusal(env.error ?? files.error)
            ? t("capabilities.actionAdminOnly")
            : undefined
        }
        onRetry={() => {
          void env.refetch().catch(() => undefined);
          void files.refetch().catch(() => undefined);
        }}
        revealEnv={revealEnv}
        revealFile={revealFile}
        saving={saving}
        save={async (patch) => {
          const { refreshFailed } = await save(
            buildSourceId,
            patch,
            "save_only",
          );
          return { affectedServiceIds: [], refreshFailed };
        }}
        retryRollout={async () => true}
      />

      <BuildSourceEnvGroupsPanel buildSourceId={buildSourceId} />
    </div>
  );
}

/**
 * Workspace-level environment groups linked to this Build Source (save-only,
 * never triggers a build). Environment-scoped groups are ineligible on Render.
 */
function BuildSourceEnvGroupsPanel({
  buildSourceId,
}: {
  buildSourceId: string;
}) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const { canCreate } = capabilities;
  const createReasonKey = capabilities.reasonKey("can_create");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const { groups, loading, error, refetch } = useEnvGroups();
  const { linkGroup, unlinkGroup, busy } = useEnvGroupMutations(refetch);

  const errorKind = classifySecretStoreError(error);
  const initialLoading = loading && groups.length === 0 && !error;

  const workspaceGroups = useMemo(
    () => groups.filter((g) => g.environmentId == null),
    [groups],
  );
  const linkedGroups = useMemo(
    () =>
      workspaceGroups.filter((g) => g.serviceLinks.includes(buildSourceId)),
    [workspaceGroups, buildSourceId],
  );
  const availableGroups = useMemo(
    () =>
      workspaceGroups.filter((g) => !g.serviceLinks.includes(buildSourceId)),
    [workspaceGroups, buildSourceId],
  );

  return (
    <Card data-region="build-env-groups">
      <CardHeader className={ENV_CARD_HEADER_CLASS}>
        <div>
          <CardTitle as="h3">{t("buildSources.fieldEnvGroups")}</CardTitle>
          <CardDescription className={ENV_CARD_DESCRIPTION_CLASS}>
            {t("buildSources.envGroupsPanelDescription")}
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <EnvGroupsErrorPanel
            kind={errorKind}
            onRetry={() => void refetch().catch(() => undefined)}
          />
        ) : initialLoading ? (
          <div className="space-y-3" aria-hidden="true">
            <Skeleton className="h-12 w-full rounded-md" />
            <Skeleton className="h-12 w-full rounded-md" />
          </div>
        ) : (
          <div className="space-y-6">
            <section className="space-y-2">
              <h4 className="text-sm font-medium">
                {t("services.envGroupsLinkedCount", {
                  count: linkedGroups.length,
                })}
              </h4>
              {linkedGroups.length === 0 ? (
                <p className="text-muted-foreground rounded-md border border-dashed p-4 text-sm">
                  {t("buildSources.envGroupsNoneLinked")}
                </p>
              ) : (
                <ul className="divide-y rounded-md border px-4">
                  {linkedGroups.map((group) => (
                    <BuildSourceEnvGroupRow
                      key={group.id}
                      group={group}
                      linked
                      canCreate={canCreate}
                      createReason={createReason}
                      busy={busy}
                      onToggle={() => void unlinkGroup(group.id, buildSourceId)}
                    />
                  ))}
                </ul>
              )}
            </section>

            <section className="space-y-2">
              <h4 className="text-sm font-medium">
                {t("services.envGroupsAvailableCount", {
                  count: availableGroups.length,
                })}
              </h4>
              {availableGroups.length === 0 ? (
                <p className="text-muted-foreground rounded-md border border-dashed p-4 text-sm">
                  {t("buildSources.fieldEnvGroupsEmpty")}
                </p>
              ) : (
                <ul className="divide-y rounded-md border px-4">
                  {availableGroups.map((group) => (
                    <BuildSourceEnvGroupRow
                      key={group.id}
                      group={group}
                      linked={false}
                      canCreate={canCreate}
                      createReason={createReason}
                      busy={busy}
                      onToggle={() => void linkGroup(group.id, buildSourceId)}
                    />
                  ))}
                </ul>
              )}
            </section>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function BuildSourceEnvGroupRow({
  group,
  linked,
  canCreate,
  createReason,
  busy,
  onToggle,
}: {
  group: EnvGroupView;
  linked: boolean;
  canCreate: boolean;
  createReason?: string;
  busy: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslations();

  return (
    <li className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2 py-4">
      <div className="min-w-0 space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <Link
            to="/env-groups/$groupId"
            params={{ groupId: group.id }}
            className="font-medium break-all hover:underline"
          >
            {group.name}
          </Link>
          {linked ? (
            <Badge variant="success">{t("services.envGroupLinked")}</Badge>
          ) : null}
        </div>
        {group.envVarKeys.length === 0 && group.secretFileNames.length === 0 ? (
          <p className="text-muted-foreground text-xs">
            {t("services.envGroupEmptyContents")}
          </p>
        ) : (
          <div className="flex flex-wrap gap-1">
            {group.envVarKeys.map((key) => (
              <Badge key={`k-${key}`} variant="secondary" className="font-mono">
                {key}
              </Badge>
            ))}
            {group.secretFileNames.map((name) => (
              <Badge key={`f-${name}`} variant="outline" className="font-mono">
                {name}
              </Badge>
            ))}
          </div>
        )}
      </div>

      <PermissionTooltip reason={createReason}>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!canCreate || busy}
          aria-label={t(
            linked
              ? "services.envGroupUnlinkAria"
              : "services.envGroupLinkAria",
            { name: group.name },
          )}
          onClick={onToggle}
        >
          {busy ? (
            <Loader2 className="animate-spin" aria-hidden="true" />
          ) : linked ? (
            <Link2Off aria-hidden="true" />
          ) : (
            <Link2 aria-hidden="true" />
          )}
          {linked ? t("services.envGroupUnlink") : t("services.envGroupLink")}
        </Button>
      </PermissionTooltip>
    </li>
  );
}

function EnvGroupsErrorPanel({
  kind,
  onRetry,
}: {
  kind: RefusalKind;
  onRetry: () => void;
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
    <div role="alert">
      <PanelCenteredState
        icon={copy.icon}
        title={copy.title}
        body={copy.body}
        action={
          kind === "generic" ? (
            <Button variant="outline" size="sm" onClick={onRetry}>
              <RotateCw aria-hidden="true" /> {t("common.tryAgain")}
            </Button>
          ) : undefined
        }
      />
    </div>
  );
}
