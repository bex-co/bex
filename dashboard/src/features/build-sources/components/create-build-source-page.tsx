import { useMemo, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { AlertTriangle, ArrowLeft } from "lucide-react";
import {
  CREATE_INPUT_CLASS,
  CREATE_SELECT_CLASS,
  CreateActionBar,
  CreateAdvanced,
  CreateField,
  CreateForm,
  CreateSection,
  CreateSelectField,
  CreateTextField,
} from "@/common/components/create-form";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { LabelError } from "@/common/components/label-error";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { regionLabel } from "@/common/lib/region-label";
import { isInvalidLabel, isValidLabel } from "@/common/lib/utils/resource-label";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { PathList } from "@/features/services/components/build-deploy-section";
import { CreateEnvVarEditor } from "@/features/services/components/create-env-var-editor";
import { CreateSecretFileEditor } from "@/features/services/components/create-secret-file-editor";
import { CreateSourceSummary } from "@/features/services/components/create-source-summary";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import {
  ServiceSourcePicker,
  type SourceTab,
} from "@/features/services/components/service-source-picker";
import { useServices } from "@/features/services/hooks/use-services";
import { buildFilterPatternError } from "@/features/services/lib/build-settings-validation";
import {
  RUNTIME_COMMANDS,
  RUNTIME_DEFS,
  isNativeRuntime,
  type GitRuntime,
} from "@/features/services/lib/runtime";
import {
  SERVICE_TYPE_LABEL,
  deriveServiceType,
} from "@/features/services/lib/service-type";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCreateBuildSource } from "@/features/build-sources/hooks/use-create-build-source";
import {
  BUILD_SOURCE_REGIONS,
  buildCreateBuildSourceParams,
  getCreateBuildSourceFieldErrors,
  isBuildSourceSourceChosen,
  isCreateBuildSourceSubmittable,
  isGitCreateTab,
  isLinkableServiceType,
  suggestBuildSourceName,
  type BuildSourceCreateTab,
  type CreateBuildSourceFormState,
} from "@/features/build-sources/lib/create-build-source-input";

const NONE_VALUE = "__none__";
const DEFAULT_REGION_VALUE = "__default__";

export interface CreateBuildSourcePageProps {
  initialProjectId?: string;
}

function initialFormState(
  initialProjectId?: string,
): CreateBuildSourceFormState {
  return {
    tab: "github",
    selectedRepo: null,
    gitUrl: "",
    imageUrl: "",
    imageRegistryCredentialId: "",
    name: "",
    projectId: initialProjectId ?? null,
    runtime: "node",
    branch: "",
    region: "",
    buildCommand: "",
    rootDir: "",
    dockerfilePath: "",
    dockerContext: "",
    gitRegistryCredentialId: "",
    buildFilterPaths: [],
    buildFilterIgnored: [],
    envVars: [],
    secretFiles: [],
    envGroupIds: [],
    serviceIds: [],
  };
}

export function CreateBuildSourcePage({
  initialProjectId,
}: CreateBuildSourcePageProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const { canCreate, loaded: capabilitiesLoaded } = useCapabilities();
  const { projects } = useProjects({ poll: false });
  const { services } = useServices({ poll: false });
  const { groups: envGroups } = useEnvGroups();

  const {
    create,
    busy,
    nameTaken,
    clearNameTaken,
    partiallyLinked,
    clearPartiallyLinked,
  } = useCreateBuildSource();

  const [form, setForm] = useState<CreateBuildSourceFormState>(() =>
    initialFormState(initialProjectId),
  );
  const [nameTouched, setNameTouched] = useState(false);
  const [editingSource, setEditingSource] = useState(false);

  const isGit = isGitCreateTab(form.tab);
  const isDocker = isGit && form.runtime === "docker";
  const sourceChosen = isBuildSourceSourceChosen(form);
  const fieldErrors = getCreateBuildSourceFieldErrors(form);
  const showNameInvalid = isInvalidLabel(form.name);
  const showNameTaken = isValidLabel(form.name) && nameTaken;
  const submittable =
    canCreate &&
    !busy &&
    !showNameTaken &&
    isCreateBuildSourceSubmittable(form);

  const advancedInvalid = Boolean(
    fieldErrors.buildFilter ||
      fieldErrors.envVarsInvalid ||
      fieldErrors.secretFilesInvalid ||
      fieldErrors.secretFilesLimit,
  );

  const linkableServices = useMemo(
    () => services.filter((svc) => isLinkableServiceType(svc.type)),
    [services],
  );
  const serviceNameById = useMemo(
    () => new Map(services.map((svc) => [svc.id, svc.name])),
    [services],
  );
  // Render only allows workspace-level env groups (not environment-scoped ones)
  // to be linked into a Build Source's build-time environment.
  const workspaceEnvGroups = useMemo(
    () => envGroups.filter((group) => group.environmentId == null),
    [envGroups],
  );

  function patch(
    update:
      | Partial<CreateBuildSourceFormState>
      | ((prev: CreateBuildSourceFormState) => Partial<CreateBuildSourceFormState>),
  ) {
    setForm((prev) => ({
      ...prev,
      ...(typeof update === "function" ? update(prev) : update),
    }));
  }

  function maybeAutofillName(candidate: string) {
    if (nameTouched) return;
    const suggested = suggestBuildSourceName(candidate);
    if (suggested) patch({ name: suggested });
  }

  function handleTabChange(nextTab: SourceTab) {
    if (nextTab === "build-source") return;
    clearPartiallyLinked();
    patch({ tab: nextTab as BuildSourceCreateTab });
  }

  function toggleEnvGroup(id: string) {
    patch((prev) => ({
      envGroupIds: prev.envGroupIds.includes(id)
        ? prev.envGroupIds.filter((item) => item !== id)
        : [...prev.envGroupIds, id],
    }));
  }

  function toggleService(id: string) {
    patch((prev) => ({
      serviceIds: prev.serviceIds.includes(id)
        ? prev.serviceIds.filter((item) => item !== id)
        : [...prev.serviceIds, id],
    }));
  }

  const patternError = (value: string) => {
    const reason = buildFilterPatternError(value);
    return reason ? t(reason) : null;
  };

  const blocked: { reason: string; fieldId?: string } = !sourceChosen
    ? { reason: t("buildSources.createBlockedSource") }
    : !isValidLabel(form.name)
      ? { reason: t("common.createBlockedName"), fieldId: "bds-name" }
      : showNameTaken
        ? { reason: t("common.createBlockedNameTaken"), fieldId: "bds-name" }
        : { reason: t("common.createBlockedFields") };

  async function handleSubmit() {
    if (!submittable || currentWorkspaceId == null) return;
    const created = await create(buildCreateBuildSourceParams(form));
    if (!created) return;
    void navigate({
      to: "/build-sources/$buildSourceId",
      params: { buildSourceId: created.id },
    });
  }

  const header = (
    <PageHeader
      leading={
        <Button variant="ghost" size="icon" asChild>
          <Link to="/build-sources" aria-label={t("buildSources.backToList")}>
            <ArrowLeft />
          </Link>
        </Button>
      }
      title={t("buildSources.createTitle")}
      description={t("buildSources.createDescription")}
    />
  );

  // Fail-closed capability gate: Viewers/Contributors never see the create form.
  if (!canCreate) {
    return (
      <DashboardLayout>
        <PageFrame width="standard" header={header}>
          <Card>
            <CardContent className="space-y-4 py-6">
              {capabilitiesLoaded ? (
                <>
                  <p className="text-muted-foreground text-sm" role="status">
                    {t("buildSources.createPermissionDenied")}
                  </p>
                  <Button asChild size="sm" variant="outline">
                    <Link to="/build-sources">
                      <ArrowLeft aria-hidden="true" />
                      {t("buildSources.backToList")}
                    </Link>
                  </Button>
                </>
              ) : (
                <div className="space-y-2" aria-hidden="true">
                  <Skeleton className="h-4 w-72" />
                  <Skeleton className="h-9 w-36 rounded-md" />
                </div>
              )}
            </CardContent>
          </Card>
        </PageFrame>
      </DashboardLayout>
    );
  }

  const buildCommandPlaceholder = isNativeRuntime(form.runtime)
    ? RUNTIME_COMMANDS[form.runtime].build
    : "";

  return (
    <DashboardLayout>
      <PageFrame
        width="standard"
        header={header}
        footer={
          <CreateActionBar
            submitLabel={t("buildSources.createSubmit")}
            onSubmit={() => void handleSubmit()}
            canSubmit={submittable}
            busy={busy}
            cancelLabel={t("common.cancel")}
            onCancel={() => void navigate({ to: "/build-sources" })}
            blocked={blocked}
          />
        }
      >
        {partiallyLinked ? (
          <Alert variant="destructive" role="alert">
            <AlertTriangle className="size-4" aria-hidden="true" />
            <AlertTitle>{t("buildSources.partiallyLinkedTitle")}</AlertTitle>
            <AlertDescription className="space-y-2">
              <p>
                {t("buildSources.partiallyLinkedDescription", {
                  id: partiallyLinked.buildSourceId,
                })}
              </p>
              {partiallyLinked.linkedServiceIds.length > 0 ? (
                <p className="text-xs">
                  {t("buildSources.partiallyLinkedSucceeded", {
                    services: partiallyLinked.linkedServiceIds
                      .map((id) => serviceNameById.get(id) ?? id)
                      .join(", "),
                  })}
                </p>
              ) : null}
              {partiallyLinked.failedServiceId ? (
                <p className="text-xs">
                  {t("buildSources.partiallyLinkedFailed", {
                    service:
                      serviceNameById.get(partiallyLinked.failedServiceId) ??
                      partiallyLinked.failedServiceId,
                  })}
                </p>
              ) : null}
              <div className="pt-1">
                <Button asChild size="sm" variant="outline">
                  {partiallyLinked.buildSourceId ? (
                    <Link
                      to="/build-sources/$buildSourceId"
                      params={{ buildSourceId: partiallyLinked.buildSourceId }}
                    >
                      {t("buildSources.viewBuildSource")}
                    </Link>
                  ) : (
                    <Link to="/build-sources">
                      {t("buildSources.viewBuildSources")}
                    </Link>
                  )}
                </Button>
              </div>
            </AlertDescription>
          </Alert>
        ) : null}

        <CreateForm>
          <div data-region="source-picker">
            <CreateField
              label={t("buildSources.fieldSource")}
              description={t("buildSources.fieldSourceDescription")}
              required
            >
              {form.tab === "github" && form.selectedRepo && !editingSource ? (
                <CreateSourceSummary
                  repo={form.selectedRepo}
                  branch={form.branch || form.selectedRepo.defaultBranch}
                  onEdit={() => setEditingSource(true)}
                />
              ) : (
                <ServiceSourcePicker
                  titleKey={null}
                  idPrefix="bds"
                  tab={form.tab}
                  onTabChange={handleTabChange}
                  selectedRepo={form.selectedRepo}
                  onSelectRepo={(selectedRepo) => {
                    patch({
                      selectedRepo,
                      branch: form.branch || selectedRepo.defaultBranch,
                    });
                    maybeAutofillName(selectedRepo.fullName);
                    setEditingSource(false);
                  }}
                  gitUrl={form.gitUrl}
                  onGitUrlChange={(gitUrl) => {
                    patch({ gitUrl });
                    maybeAutofillName(gitUrl);
                  }}
                  onGitHubPage={(page) =>
                    patch((current) => ({
                      branch: current.branch || page.branch,
                      rootDir: current.rootDir || page.rootDir,
                    }))
                  }
                  image={{
                    value: form.imageUrl,
                    onChange: (imageUrl) => {
                      patch({ imageUrl });
                      maybeAutofillName(imageUrl);
                    },
                    registryCredentialId: form.imageRegistryCredentialId,
                    onRegistryCredentialChange: (imageRegistryCredentialId) =>
                      patch({ imageRegistryCredentialId }),
                    showPortHint: false,
                  }}
                />
              )}
            </CreateField>
          </div>

          <div data-region="details" className="space-y-8">
            <CreateField
              label={t("buildSources.fieldName")}
              htmlFor="bds-name"
              description={t("buildSources.fieldNameDescription")}
              required
            >
              <Input
                id="bds-name"
                value={form.name}
                onChange={(event) => {
                  setNameTouched(true);
                  clearNameTaken();
                  patch({ name: event.target.value });
                }}
                placeholder={t("buildSources.fieldNamePlaceholder")}
                autoComplete="off"
                aria-invalid={showNameInvalid || showNameTaken}
                aria-describedby={
                  showNameInvalid || showNameTaken ? "bds-name-error" : undefined
                }
                className={CREATE_INPUT_CLASS}
              />
              {showNameInvalid ? (
                <LabelError id="bds-name-error" value={form.name} />
              ) : showNameTaken ? (
                <p
                  id="bds-name-error"
                  role="alert"
                  className="text-destructive text-sm"
                >
                  {t("buildSources.errorNameTaken", { name: form.name.trim() })}
                </p>
              ) : null}
            </CreateField>

            <CreateField
              label={t("buildSources.fieldProject")}
              htmlFor="bds-project"
              optional
              description={t("buildSources.fieldProjectDescription")}
            >
              <div>
                <Select
                  value={form.projectId ?? NONE_VALUE}
                  onValueChange={(value) =>
                    patch({ projectId: value === NONE_VALUE ? null : value })
                  }
                >
                  <SelectTrigger
                    id="bds-project"
                    className={CREATE_SELECT_CLASS}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE_VALUE}>
                      {t("buildSources.fieldProjectNone")}
                    </SelectItem>
                    {projects.map((project) => (
                      <SelectItem key={project.id} value={project.id}>
                        {project.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </CreateField>
          </div>

          {isGit ? (
            <div data-region="build-settings" className="space-y-8">
              <CreateSelectField
                id="bds-runtime"
                label={t("buildSources.fieldRuntime")}
                description={t("buildSources.fieldRuntimeDescription")}
                value={form.runtime}
                onValueChange={(value) =>
                  patch({ runtime: value as GitRuntime })
                }
              >
                {RUNTIME_DEFS.map(({ id, labelKey }) => (
                  <SelectItem key={id} value={id}>
                    {t(labelKey)}
                  </SelectItem>
                ))}
              </CreateSelectField>

              <CreateTextField
                id="bds-branch"
                label={t("buildSources.fieldBranch")}
                optional
                value={form.branch}
                onChange={(branch) => patch({ branch })}
                placeholder={
                  (form.tab === "github" &&
                    form.selectedRepo?.defaultBranch) ||
                  t("services.createFieldBranchPlaceholder")
                }
                description={t("buildSources.fieldBranchDescription")}
              />

              <CreateField
                label={t("buildSources.fieldRegion")}
                htmlFor="bds-region"
                optional
                description={t("buildSources.fieldRegionDescription")}
              >
                <div>
                  <Select
                    value={form.region || DEFAULT_REGION_VALUE}
                    onValueChange={(value) =>
                      patch({
                        region: value === DEFAULT_REGION_VALUE ? "" : value,
                      })
                    }
                  >
                    <SelectTrigger
                      id="bds-region"
                      className={CREATE_SELECT_CLASS}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={DEFAULT_REGION_VALUE}>
                        {t("buildSources.fieldRegionDefault")}
                      </SelectItem>
                      {BUILD_SOURCE_REGIONS.map((region) => (
                        <SelectItem key={region} value={region}>
                          {regionLabel(region, t)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </CreateField>

              {!isDocker ? (
                <CreateTextField
                  id="bds-build-command"
                  label={t("services.createFieldBuildCommand")}
                  optional
                  mono
                  value={form.buildCommand}
                  onChange={(buildCommand) => patch({ buildCommand })}
                  placeholder={buildCommandPlaceholder}
                  description={t(
                    "services.createFieldBuildCommandDescription",
                  )}
                />
              ) : (
                <>
                  <CreateTextField
                    id="bds-dockerfile-path"
                    label={t("services.dockerfilePathLabel")}
                    optional
                    mono
                    value={form.dockerfilePath}
                    onChange={(dockerfilePath) => patch({ dockerfilePath })}
                    placeholder="./Dockerfile"
                    description={t("services.dockerfilePathHint")}
                    error={
                      fieldErrors.dockerfilePath
                        ? t(fieldErrors.dockerfilePath)
                        : undefined
                    }
                  />
                  <CreateTextField
                    id="bds-docker-context"
                    label={t("services.dockerContextLabel")}
                    optional
                    mono
                    value={form.dockerContext}
                    onChange={(dockerContext) => patch({ dockerContext })}
                    placeholder="."
                    description={t("services.dockerContextHint")}
                    error={
                      fieldErrors.dockerContext
                        ? t(fieldErrors.dockerContext)
                        : undefined
                    }
                  />
                  <CreateField
                    label={t("services.createRegistryCredentialLabel")}
                    htmlFor="bds-git-registry-credential"
                    optional
                    description={t("buildSources.fieldGitRegistryDescription")}
                  >
                    <RegistryCredentialSelect
                      id="bds-git-registry-credential"
                      hideLabel
                      value={form.gitRegistryCredentialId}
                      onValueChange={(gitRegistryCredentialId) =>
                        patch({ gitRegistryCredentialId })
                      }
                      description={t(
                        "services.createRegistryCredentialDescription",
                      )}
                    />
                  </CreateField>
                </>
              )}

              <CreateTextField
                id="bds-root-dir"
                label={t("services.createFieldRootDir")}
                optional
                mono
                value={form.rootDir}
                onChange={(rootDir) => patch({ rootDir })}
                placeholder={t("services.createFieldRootDirPlaceholder")}
                description={t("services.createFieldRootDirHint")}
                error={
                  fieldErrors.rootDir ? t(fieldErrors.rootDir) : undefined
                }
              />

              <CreateAdvanced forceOpen={advancedInvalid}>
                <CreateField
                  label={t("services.buildFilterLabel")}
                  optional
                  description={t("services.buildFilterHint")}
                >
                  <div className="space-y-4">
                    <PathList
                      title={t("services.buildFilterIncludedTitle")}
                      hint={t("services.buildFilterIncludedHint")}
                      placeholder={t("services.buildFilterIncludedPlaceholder")}
                      addLabel={t("services.buildFilterAddIncluded")}
                      removeLabel={t("services.buildFilterRemoveIncluded")}
                      values={form.buildFilterPaths}
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
                      values={form.buildFilterIgnored}
                      onChange={(buildFilterIgnored) =>
                        patch({ buildFilterIgnored })
                      }
                      validate={patternError}
                    />
                  </div>
                </CreateField>

                <CreateEnvVarEditor
                  rows={form.envVars}
                  onChange={(envVars) => patch({ envVars })}
                />

                <CreateSecretFileEditor
                  rows={form.secretFiles}
                  onChange={(secretFiles) => patch({ secretFiles })}
                />

                <CreateField
                  label={t("buildSources.fieldEnvGroups")}
                  optional
                  description={t("buildSources.fieldEnvGroupsDescription")}
                >
                  {workspaceEnvGroups.length === 0 ? (
                    <p className="text-muted-foreground text-sm">
                      {t("buildSources.fieldEnvGroupsEmpty")}
                    </p>
                  ) : (
                    <div className="space-y-2 rounded-md border p-3">
                      {workspaceEnvGroups.map((group) => {
                        const checked = form.envGroupIds.includes(group.id);
                        const checkboxId = `bds-env-group-${group.id}`;
                        return (
                          <div
                            key={group.id}
                            className="flex items-center gap-2.5"
                          >
                            <Checkbox
                              id={checkboxId}
                              checked={checked}
                              onCheckedChange={() => toggleEnvGroup(group.id)}
                            />
                            <Label
                              htmlFor={checkboxId}
                              className="min-w-0 flex-1 cursor-pointer font-normal"
                            >
                              <span className="truncate font-medium">
                                {group.name}
                              </span>
                            </Label>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </CreateField>
              </CreateAdvanced>
            </div>
          ) : null}

          <div data-region="link-services">
            <CreateSection
              title={t("buildSources.linkServicesTitle")}
              description={t("buildSources.linkServicesDescription")}
            >
              {linkableServices.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  {t("buildSources.linkServicesEmpty")}
                </p>
              ) : (
                <div className="space-y-2">
                  {linkableServices.map((service) => {
                    const checked = form.serviceIds.includes(service.id);
                    const checkboxId = `bds-service-${service.id}`;
                    const typeKey = deriveServiceType(service.type);
                    return (
                      <div
                        key={service.id}
                        className="flex items-center gap-3 rounded-md border px-3 py-2.5"
                      >
                        <Checkbox
                          id={checkboxId}
                          checked={checked}
                          onCheckedChange={() => toggleService(service.id)}
                        />
                        <Label
                          htmlFor={checkboxId}
                          className="flex min-w-0 flex-1 cursor-pointer flex-wrap items-center justify-between gap-2 font-normal"
                        >
                          <span className="truncate font-medium">
                            {service.name}
                          </span>
                          <span className="flex shrink-0 items-center gap-1.5">
                            <Badge variant="outline">
                              {t(SERVICE_TYPE_LABEL[typeKey])}
                            </Badge>
                            {service.region ? (
                              <Badge variant="secondary">
                                {regionLabel(service.region, t)}
                              </Badge>
                            ) : null}
                          </span>
                        </Label>
                      </div>
                    );
                  })}
                </div>
              )}
            </CreateSection>
          </div>
        </CreateForm>
      </PageFrame>
    </DashboardLayout>
  );
}
