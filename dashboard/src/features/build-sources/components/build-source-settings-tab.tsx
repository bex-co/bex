import { useMemo, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  AlertTriangle,
  ExternalLink,
  FolderKanban,
  Loader2,
  Save,
} from "lucide-react";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { CopyButton } from "@/common/components/copy-button";
import { LabelError } from "@/common/components/label-error";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { regionLabel } from "@/common/lib/region-label";
import { isValidDockerImageRef } from "@/common/lib/utils/docker-image";
import { isValidGitUrl, parseGitHubPage } from "@/common/lib/utils/git-url";
import { isInvalidLabel, isValidLabel } from "@/common/lib/utils/resource-label";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { PathList } from "@/features/services/components/build-deploy-section";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import {
  buildFilterPatternError,
  relativePathError,
  rootDirError,
} from "@/features/services/lib/build-settings-validation";
import {
  RUNTIME_COMMANDS,
  RUNTIME_DEFS,
  isNativeRuntime,
  type GitRuntime,
} from "@/features/services/lib/runtime";
import { useBuildSourceDetailContext } from "@/features/build-sources/components/build-source-detail-context";
import { useDeleteBuildSource } from "@/features/build-sources/hooks/use-delete-build-source";
import { useUpdateBuildSource } from "@/features/build-sources/hooks/use-update-build-source";
import { buildSourceSudoDeletePhrase } from "@/features/build-sources/lib/build-status";
import type { BuildSourceView } from "@/features/build-sources/types";
import type { UpdateBuildSourceMutationVariables } from "@/graphql/definitions";

function initialDraft(source: BuildSourceView) {
  return {
    name: source.name,
    repoUrl: source.git?.repoUrl ?? "",
    branch: source.git?.branch ?? "",
    runtime: (source.git?.runtime ?? "node") as GitRuntime,
    buildCommand: source.git?.buildCommand ?? "",
    rootDir: source.git?.rootDir ?? "",
    dockerfilePath: source.git?.dockerfilePath ?? "",
    dockerContext: source.git?.dockerContext ?? "",
    gitRegistryCredentialId: source.git?.registryCredentialId ?? "",
    buildFilterPaths: source.git?.buildFilter?.paths ?? [],
    buildFilterIgnored: source.git?.buildFilter?.ignoredPaths ?? [],
    imageUrl: source.image?.imageUrl ?? "",
    imageRegistryCredentialId: source.image?.registryCredentialId ?? "",
  };
}

/**
 * Settings & Safe Delete tab (`/build-sources/$buildSourceId/settings`,
 * `w2/m614/t006`): persists edits via `updateBuildSource` without triggering a
 * build, displays immutable identity fields, and guards deletion while any
 * service remains linked.
 */
export function BuildSourceSettingsTab() {
  const { buildSource } = useBuildSourceDetailContext();
  return <BuildSourceSettingsForm key={buildSource.id} source={buildSource} />;
}

function BuildSourceSettingsForm({ source }: { source: BuildSourceView }) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const capabilities = useCapabilities();
  const { canCreate } = capabilities;
  const createReasonKey = capabilities.reasonKey("can_create");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const { projects } = useProjects({ poll: false });
  const { update, busy: saving, nameTaken, clearNameTaken } =
    useUpdateBuildSource();
  const { remove, busy: deleting } = useDeleteBuildSource();

  const [draft, setDraft] = useState(() => initialDraft(source));
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [confirmText, setConfirmText] = useState("");

  const isGit = source.kind === "git";
  const isDocker = isGit && draft.runtime === "docker";
  const project = source.projectId
    ? projects.find((p) => p.id === source.projectId)
    : undefined;
  const region = source.git?.region ? regionLabel(source.git.region, t) : null;

  const patch = (next: Partial<typeof draft>) =>
    setDraft((prev) => ({ ...prev, ...next }));

  const showNameInvalid = isInvalidLabel(draft.name);
  const showNameTaken = isValidLabel(draft.name) && nameTaken;

  const gitRepoValid =
    !isGit ||
    (draft.repoUrl.trim() !== "" &&
      isValidGitUrl(draft.repoUrl.trim()) &&
      parseGitHubPage(draft.repoUrl.trim())?.file !== true);
  const imageValid =
    isGit ||
    (draft.imageUrl.trim() !== "" &&
      isValidDockerImageRef(draft.imageUrl.trim()));
  const rootDirErr =
    isGit && draft.rootDir.trim() ? rootDirError(draft.rootDir) : null;
  const dockerfileErr =
    isDocker && draft.dockerfilePath.trim()
      ? relativePathError(draft.dockerfilePath)
      : null;
  const dockerContextErr =
    isDocker && draft.dockerContext.trim()
      ? relativePathError(draft.dockerContext)
      : null;
  const buildFilterInvalid =
    isGit &&
    [...draft.buildFilterPaths, ...draft.buildFilterIgnored].some(
      (p) => p.trim() !== "" && buildFilterPatternError(p) !== null,
    );

  const dirty = useMemo(() => {
    const base = initialDraft(source);
    if (draft.name.trim() !== base.name.trim()) return true;
    if (isGit) {
      return (
        draft.repoUrl.trim() !== base.repoUrl.trim() ||
        draft.branch.trim() !== base.branch.trim() ||
        draft.runtime !== base.runtime ||
        draft.buildCommand.trim() !== base.buildCommand.trim() ||
        draft.rootDir.trim() !== base.rootDir.trim() ||
        draft.dockerfilePath.trim() !== base.dockerfilePath.trim() ||
        draft.dockerContext.trim() !== base.dockerContext.trim() ||
        draft.gitRegistryCredentialId.trim() !==
          base.gitRegistryCredentialId.trim() ||
        JSON.stringify(draft.buildFilterPaths) !==
          JSON.stringify(base.buildFilterPaths) ||
        JSON.stringify(draft.buildFilterIgnored) !==
          JSON.stringify(base.buildFilterIgnored)
      );
    }
    return (
      draft.imageUrl.trim() !== base.imageUrl.trim() ||
      draft.imageRegistryCredentialId.trim() !==
        base.imageRegistryCredentialId.trim()
    );
  }, [draft, isGit, source]);

  const valid =
    isValidLabel(draft.name) &&
    !showNameTaken &&
    gitRepoValid &&
    imageValid &&
    !rootDirErr &&
    !dockerfileErr &&
    !dockerContextErr &&
    !buildFilterInvalid;

  const canSave = canCreate && dirty && valid && !saving;

  const patternError = (value: string) => {
    const reason = buildFilterPatternError(value);
    return reason ? t(reason) : null;
  };

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!canSave) return;

    const variables: UpdateBuildSourceMutationVariables = {
      id: source.id,
      ...(draft.name.trim() !== source.name
        ? { name: draft.name.trim() }
        : {}),
    };

    if (isGit) {
      const paths = draft.buildFilterPaths.map((p) => p.trim()).filter(Boolean);
      const ignoredPaths = draft.buildFilterIgnored
        .map((p) => p.trim())
        .filter(Boolean);
      variables.git = {
        repoUrl: draft.repoUrl.trim(),
        branch: draft.branch.trim() || undefined,
        runtime: draft.runtime,
        ...(draft.rootDir.trim() ? { rootDir: draft.rootDir.trim() } : {}),
        ...(!isDocker && draft.buildCommand.trim()
          ? { buildCommand: draft.buildCommand.trim() }
          : {}),
        ...(isDocker && draft.dockerfilePath.trim()
          ? { dockerfilePath: draft.dockerfilePath.trim() }
          : {}),
        ...(isDocker && draft.dockerContext.trim()
          ? { dockerContext: draft.dockerContext.trim() }
          : {}),
        ...(isDocker
          ? { registryCredentialId: draft.gitRegistryCredentialId.trim() }
          : {}),
        buildFilter: { paths, ignoredPaths },
      };
    } else {
      variables.image = {
        imageUrl: draft.imageUrl.trim(),
        registryCredentialId: draft.imageRegistryCredentialId.trim(),
      };
    }

    const updated = await update(variables);
    if (updated) {
      setDraft(initialDraft(updated));
    }
  }

  const expectedPhrase = buildSourceSudoDeletePhrase(source.name);
  const hasLinkedServices = source.serviceLinks.length > 0;
  const deleteMatches = confirmText === expectedPhrase;

  async function handleDelete() {
    if (!deleteMatches || deleting || hasLinkedServices || !canCreate) return;
    const deleted = await remove(source.id, source.name);
    if (!deleted) return;
    setDeleteOpen(false);
    void navigate({ to: "/build-sources", replace: true });
  }

  const buildCommandPlaceholder = isNativeRuntime(draft.runtime)
    ? RUNTIME_COMMANDS[draft.runtime].build
    : "";

  return (
    <div data-region="build-settings" className="space-y-6">
      {/* Immutable identity metadata */}
      <Card>
        <CardHeader>
          <CardTitle as="h3">{t("buildSources.settingsIdentityTitle")}</CardTitle>
          <CardDescription>
            {t("buildSources.settingsIdentityDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-3">
            <div className="space-y-1">
              <dt className="text-muted-foreground text-xs">
                {t("buildSources.settingsFieldId")}
              </dt>
              <dd className="inline-flex items-center gap-1 font-mono text-xs font-medium">
                <span className="break-all">{source.id}</span>
                <CopyButton
                  value={source.id}
                  label={t("buildSources.copyId")}
                  successText={t("buildSources.copied")}
                  errorText={t("buildSources.copyError")}
                />
              </dd>
            </div>

            {region ? (
              <div className="space-y-1">
                <dt className="text-muted-foreground text-xs">
                  {t("buildSources.colRegion")}
                </dt>
                <dd className="font-medium">{region}</dd>
              </div>
            ) : null}

            <div className="space-y-1">
              <dt className="text-muted-foreground text-xs">
                {t("buildSources.colProject")}
              </dt>
              <dd className="font-medium">
                {source.projectId ? (
                  <Link
                    to="/project/$projectId"
                    params={{ projectId: source.projectId }}
                    className="inline-flex items-center gap-1 hover:underline"
                  >
                    <FolderKanban className="size-3.5" aria-hidden="true" />
                    {project?.name ?? source.projectId}
                  </Link>
                ) : (
                  t("buildSources.workspaceScope")
                )}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      {/* Editable configuration */}
      <form onSubmit={(e) => void handleSave(e)}>
        <Card>
          <CardHeader>
            <CardTitle as="h3">
              {t("buildSources.settingsConfigTitle")}
            </CardTitle>
            <CardDescription>
              {t("buildSources.settingsNoBuildNote")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="bds-settings-name">
                {t("buildSources.fieldName")}
              </Label>
              <Input
                id="bds-settings-name"
                value={draft.name}
                disabled={!canCreate || saving}
                onChange={(e) => {
                  clearNameTaken();
                  patch({ name: e.target.value });
                }}
                aria-invalid={showNameInvalid || showNameTaken}
                autoComplete="off"
              />
              {showNameInvalid ? (
                <LabelError id="bds-settings-name-error" value={draft.name} />
              ) : showNameTaken ? (
                <p
                  id="bds-settings-name-error"
                  role="alert"
                  className="text-destructive text-sm"
                >
                  {t("buildSources.errorNameTaken", {
                    name: draft.name.trim(),
                  })}
                </p>
              ) : null}
            </div>

            {isGit ? (
              <>
                <div className="space-y-2">
                  <Label htmlFor="bds-settings-repo">
                    {t("buildSources.settingsFieldRepoUrl")}
                  </Label>
                  <Input
                    id="bds-settings-repo"
                    value={draft.repoUrl}
                    disabled={!canCreate || saving}
                    onChange={(e) => patch({ repoUrl: e.target.value })}
                    className="font-mono"
                    aria-invalid={!gitRepoValid}
                    autoComplete="off"
                  />
                  {!gitRepoValid ? (
                    <p role="alert" className="text-destructive text-xs">
                      {t("buildSources.settingsInvalidRepoUrl")}
                    </p>
                  ) : null}
                </div>

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="bds-settings-branch">
                      {t("buildSources.fieldBranch")}
                    </Label>
                    <Input
                      id="bds-settings-branch"
                      value={draft.branch}
                      disabled={!canCreate || saving}
                      onChange={(e) => patch({ branch: e.target.value })}
                      className="font-mono"
                      autoComplete="off"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="bds-settings-runtime">
                      {t("buildSources.fieldRuntime")}
                    </Label>
                    <Select
                      value={draft.runtime}
                      disabled={!canCreate || saving}
                      onValueChange={(value) =>
                        patch({ runtime: value as GitRuntime })
                      }
                    >
                      <SelectTrigger id="bds-settings-runtime">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {RUNTIME_DEFS.map(({ id, labelKey }) => (
                          <SelectItem key={id} value={id}>
                            {t(labelKey)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                {!isDocker ? (
                  <div className="space-y-2">
                    <Label htmlFor="bds-settings-build-command">
                      {t("services.createFieldBuildCommand")}
                    </Label>
                    <Input
                      id="bds-settings-build-command"
                      value={draft.buildCommand}
                      disabled={!canCreate || saving}
                      onChange={(e) => patch({ buildCommand: e.target.value })}
                      placeholder={buildCommandPlaceholder}
                      className="font-mono"
                      autoComplete="off"
                    />
                  </div>
                ) : (
                  <>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <div className="space-y-2">
                        <Label htmlFor="bds-settings-dockerfile">
                          {t("services.dockerfilePathLabel")}
                        </Label>
                        <Input
                          id="bds-settings-dockerfile"
                          value={draft.dockerfilePath}
                          disabled={!canCreate || saving}
                          onChange={(e) =>
                            patch({ dockerfilePath: e.target.value })
                          }
                          placeholder="./Dockerfile"
                          className="font-mono"
                          aria-invalid={Boolean(dockerfileErr)}
                          autoComplete="off"
                        />
                        {dockerfileErr ? (
                          <p role="alert" className="text-destructive text-xs">
                            {t(dockerfileErr)}
                          </p>
                        ) : null}
                      </div>

                      <div className="space-y-2">
                        <Label htmlFor="bds-settings-docker-context">
                          {t("services.dockerContextLabel")}
                        </Label>
                        <Input
                          id="bds-settings-docker-context"
                          value={draft.dockerContext}
                          disabled={!canCreate || saving}
                          onChange={(e) =>
                            patch({ dockerContext: e.target.value })
                          }
                          placeholder="."
                          className="font-mono"
                          aria-invalid={Boolean(dockerContextErr)}
                          autoComplete="off"
                        />
                        {dockerContextErr ? (
                          <p role="alert" className="text-destructive text-xs">
                            {t(dockerContextErr)}
                          </p>
                        ) : null}
                      </div>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="bds-settings-git-registry">
                        {t("services.createRegistryCredentialLabel")}
                      </Label>
                      <RegistryCredentialSelect
                        id="bds-settings-git-registry"
                        hideLabel
                        disabled={!canCreate || saving}
                        value={draft.gitRegistryCredentialId}
                        onValueChange={(gitRegistryCredentialId) =>
                          patch({ gitRegistryCredentialId })
                        }
                        description={t(
                          "buildSources.fieldGitRegistryDescription",
                        )}
                      />
                    </div>
                  </>
                )}

                <div className="space-y-2">
                  <Label htmlFor="bds-settings-root-dir">
                    {t("services.createFieldRootDir")}
                  </Label>
                  <Input
                    id="bds-settings-root-dir"
                    value={draft.rootDir}
                    disabled={!canCreate || saving}
                    onChange={(e) => patch({ rootDir: e.target.value })}
                    placeholder={t("services.createFieldRootDirPlaceholder")}
                    className="font-mono"
                    aria-invalid={Boolean(rootDirErr)}
                    autoComplete="off"
                  />
                  {rootDirErr ? (
                    <p role="alert" className="text-destructive text-xs">
                      {t(rootDirErr)}
                    </p>
                  ) : null}
                </div>

                <div className="space-y-3 border-t pt-4">
                  <div>
                    <p className="text-sm font-medium">
                      {t("services.buildFilterLabel")}
                    </p>
                    <p className="text-muted-foreground text-xs">
                      {t("services.buildFilterHint")}
                    </p>
                  </div>
                  <div className="space-y-4">
                    <PathList
                      title={t("services.buildFilterIncludedTitle")}
                      hint={t("services.buildFilterIncludedHint")}
                      placeholder={t("services.buildFilterIncludedPlaceholder")}
                      addLabel={t("services.buildFilterAddIncluded")}
                      removeLabel={t("services.buildFilterRemoveIncluded")}
                      values={draft.buildFilterPaths}
                      disabled={!canCreate || saving}
                      onChange={(buildFilterPaths) =>
                        patch({ buildFilterPaths })
                      }
                      validate={patternError}
                    />
                    <PathList
                      title={t("services.buildFilterIgnoredTitle")}
                      hint={t("services.buildFilterIgnoredHint")}
                      placeholder={t("services.buildFilterIgnoredPlaceholder")}
                      addLabel={t("services.buildFilterAddIgnored")}
                      removeLabel={t("services.buildFilterRemoveIgnored")}
                      values={draft.buildFilterIgnored}
                      disabled={!canCreate || saving}
                      onChange={(buildFilterIgnored) =>
                        patch({ buildFilterIgnored })
                      }
                      validate={patternError}
                    />
                  </div>
                </div>
              </>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="bds-settings-image-url">
                    {t("services.sourceImageLabel")}
                  </Label>
                  <Input
                    id="bds-settings-image-url"
                    value={draft.imageUrl}
                    disabled={!canCreate || saving}
                    onChange={(e) => patch({ imageUrl: e.target.value })}
                    className="font-mono"
                    aria-invalid={!imageValid}
                    autoComplete="off"
                  />
                  {!imageValid ? (
                    <p role="alert" className="text-destructive text-xs">
                      {t("buildSources.settingsInvalidImageUrl")}
                    </p>
                  ) : null}
                </div>

                <div className="space-y-2">
                  <Label htmlFor="bds-settings-image-registry">
                    {t("services.createRegistryCredentialLabel")}
                  </Label>
                  <RegistryCredentialSelect
                    id="bds-settings-image-registry"
                    hideLabel
                    disabled={!canCreate || saving}
                    value={draft.imageRegistryCredentialId}
                    onValueChange={(imageRegistryCredentialId) =>
                      patch({ imageRegistryCredentialId })
                    }
                    description={t(
                      "services.createRegistryCredentialDescription",
                    )}
                  />
                </div>
              </>
            )}
          </CardContent>
          <CardFooter className="flex flex-wrap items-center justify-between gap-2 border-t pt-4">
            {createReason ? (
              <p className="text-muted-foreground text-xs">{createReason}</p>
            ) : (
              <span />
            )}
            <div className="flex items-center gap-2">
              {dirty ? (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={saving}
                  onClick={() => {
                    clearNameTaken();
                    setDraft(initialDraft(source));
                  }}
                >
                  {t("common.cancel")}
                </Button>
              ) : null}
              <PermissionTooltip reason={createReason}>
                <Button type="submit" size="sm" disabled={!canSave}>
                  {saving ? (
                    <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                  ) : (
                    <Save className="size-4" aria-hidden="true" />
                  )}
                  {saving
                    ? t("buildSources.settingsSaving")
                    : t("buildSources.settingsSaveButton")}
                </Button>
              </PermissionTooltip>
            </div>
          </CardFooter>
        </Card>
      </form>

      {/* Danger Zone — Delete Build Source */}
      <Card className="border-destructive/50" data-region="build-danger-zone">
        <CardHeader>
          <CardTitle as="h3" className="text-destructive">
            {t("buildSources.dangerZoneTitle")}
          </CardTitle>
          <CardDescription>
            {t("buildSources.dangerZoneDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {hasLinkedServices ? (
            <Alert data-testid="delete-blocked-linked-services">
              <AlertTriangle className="size-4" aria-hidden="true" />
              <AlertTitle>
                {t("buildSources.deleteBlockedTitle", {
                  count: source.serviceLinks.length,
                })}
              </AlertTitle>
              <AlertDescription className="space-y-2">
                <p>{t("buildSources.deleteBlockedDescription")}</p>
                <ul className="space-y-1 text-xs">
                  {source.serviceLinks.map((svc) => (
                    <li key={svc.id}>
                      <Link
                        to="/services/$serviceId/settings"
                        params={{ serviceId: svc.id }}
                        className="text-primary inline-flex items-center gap-1 font-medium hover:underline"
                      >
                        {svc.name}
                        <ExternalLink className="size-3" aria-hidden="true" />
                      </Link>
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          ) : null}

          <PermissionTooltip reason={createReason}>
            <Button
              type="button"
              variant="destructive"
              disabled={!canCreate || hasLinkedServices || deleting}
              onClick={() => {
                setConfirmText("");
                setDeleteOpen(true);
              }}
            >
              {t("buildSources.deleteButton")}
            </Button>
          </PermissionTooltip>
        </CardContent>

        <ConfirmDialog
          open={deleteOpen}
          onOpenChange={(next) => {
            setDeleteOpen(next);
            if (!next) setConfirmText("");
          }}
          title={t("buildSources.deleteConfirmTitle", { name: source.name })}
          description={t("buildSources.deleteConfirmBody")}
          cancelLabel={t("common.cancel")}
          confirmLabel={
            <>
              {deleting ? (
                <Loader2 className="animate-spin" aria-hidden="true" />
              ) : null}
              {t("buildSources.deleteConfirmButton")}
            </>
          }
          confirmDisabled={!deleteMatches || deleting}
          pending={deleting}
          closeOnConfirm={false}
          onConfirm={() => void handleDelete()}
        >
          <SudoCommandField
            id="build-source-delete-confirm"
            promptKey="buildSources.deleteConfirmPrompt"
            phrase={expectedPhrase}
            value={confirmText}
            onValueChange={setConfirmText}
          />
        </ConfirmDialog>
      </Card>
    </div>
  );
}
