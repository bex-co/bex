import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { CRON_MINIMUM_USD } from "@/common/lib/cron-minimum";
import { Loader2 } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Switch } from "@/common/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  CREATE_INPUT_CLASS,
  CREATE_SELECT_CLASS,
  CREATE_SWITCH_CLASS,
  CreateActionBar,
  CreateAdvanced,
  CreateCapLimitAlert,
  CreateField,
  CreateForm,
  CreateSelectField,
  CreateTextField,
  PlanSummary,
} from "@/common/components/create-form";
import { cn } from "@/common/lib/utils/utils";
import {
  buildSourceDisplayLabel,
  type BuildSourceOption,
} from "@/features/services/hooks/use-build-sources";
import {
  CREATE_PLAN_ROWS,
  ComputePlanList,
} from "@/common/components/compute-plan-list";
import { useCreateService } from "@/features/services/hooks/use-create-service";
import {
  cronScheduleProblem,
  describeCron,
} from "@/features/services/lib/cron";
import { ProjectEnvironmentSelector } from "@/features/environments/components/project-environment-selector";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import { PathList } from "@/features/services/components/build-deploy-section";
import { useNewServiceForm } from "@/features/services/hooks/use-new-service-form";
import { RUNTIME_DEFS, type GitRuntime } from "@/features/services/lib/runtime";
import { ServiceSourcePicker } from "@/features/services/components/service-source-picker";
import { CreateSourceSummary } from "@/features/services/components/create-source-summary";
import { ServiceCreateHeader } from "@/features/services/components/service-create-header";
import {
  DEFAULT_SERVICE_PORT,
  MAX_SERVICE_PORT,
  MIN_SERVICE_PORT,
} from "@/features/services/lib/port";
import { CreateEnvVarEditor } from "@/features/services/components/create-env-var-editor";
import { buildFilterPatternError } from "@/features/services/lib/build-settings-validation";
import { CreateSecretFileEditor } from "@/features/services/components/create-secret-file-editor";
import {
  buildCreateServiceInput,
  buildShape,
  createFormErrors,
  isSubmittable,
  nativeCommandsMissing,
  portDraft,
  portIsValid,
  secretFilesInvalid,
  sourceChosen,
} from "@/features/services/lib/create-service-input";
import {
  serviceTypeCreateCopy,
  type NewServiceSearch,
  type ServiceType,
} from "@/features/services/lib/create-context";
import { PlanTierSlots } from "@/common/components/datastore-plan-preview";
import { PlanCatalogUnavailable } from "@/common/components/plan-catalog-unavailable";
import { servicePlanSlotCount } from "@/features/services/lib/instance-type";

/**
 * Render's service create page (render.com/web/new and its siblings, live
 * captures 2026-11-21), one template for all five types at Render's own
 * paths (`/web/new`, `/pserv/new`, `/worker/new`, `/cron/new`, `/static/new`):
 * the type is the page's heading, switched from the breadcrumb ("New Web
 * Service ▾", dashboard-breadcrumbs.tsx); Source Code comes first and
 * the settings appear once a source is chosen; rows run Name, Project, the
 * type's build and run fields, Compute, Environment Variables, then a
 * collapsed Advanced card; the type-specific Deploy button and the plan's
 * price ride the sticky action bar.
 */
export function NewServicePage({
  serviceType,
  search,
}: {
  /** The page's path names the type (Render's /web/new …). */
  serviceType: ServiceType;
  /** The Project/Environment context a contextual link passes. */
  search: NewServiceSearch;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const {
    create,
    busy,
    capLimit,
    nameConflict,
    clearNameConflict,
    imageRefusal,
    clearImageRefusal,
  } = useCreateService();
  const {
    form,
    set,
    setTab,
    setServiceType,
    selectBuildSource,
    buildSources,
    build,
    name,
    instanceTypes,
    catalog,
  } = useNewServiceForm({ ...search, type: serviceType });

  // The breadcrumb's type switcher moves between the five create paths, which
  // share one layout route: the page stays mounted and the draft survives the
  // switch (w6/045); the form follows the path's type.
  if (form.serviceType !== serviceType) setServiceType(serviceType);

  // The settings appear once a source is chosen and stay: correcting the
  // source later must not fold away what was already filled in.
  const chosen = sourceChosen(form);
  const [revealed, setRevealed] = useState(chosen);
  if (chosen && !revealed) setRevealed(true);
  const [editingSource, setEditingSource] = useState(false);

  const shape = buildShape(form);
  const fieldErrors = createFormErrors(form);
  const patternError = (value: string) => {
    const reason = buildFilterPatternError(value);
    return reason ? t(reason) : null;
  };
  const createCopy = serviceTypeCreateCopy(form.serviceType);

  const scheduleProblem =
    shape.isCronType && form.schedule.trim() !== ""
      ? cronScheduleProblem(form.schedule)
      : null;
  const scheduleDescription = shape.isCronType
    ? describeCron(form.schedule, t)
    : null;
  // Only a port-binding type reads the field, so a stale draft under a worker
  // never shows an error (or blocks Submit) for a value nothing will send.
  const portError = shape.showPort && !portIsValid(form, shape);
  const port = portDraft(form, shape);
  const publishPathMissing = form.publishPath.trim() === "";
  const showNameError = name.name.length > 0 && !name.nameValid;
  const showNameTaken = name.nameValid && (name.nameTaken || nameConflict);
  const canSubmit =
    name.nameValid && !showNameTaken && !busy && isSubmittable(form);
  const selectedPlan = instanceTypes.find((it) => it.id === form.plan);
  // An invalid field inside Advanced holds the card open, so the reason
  // Deploy is held is never folded away.
  const advancedInvalid = Boolean(
    fieldErrors.dockerfilePath ||
    fieldErrors.buildFilter ||
    fieldErrors.secretFilesLimit ||
    secretFilesInvalid(form),
  );
  // The first thing holding Deploy, in page order: its words for the bar and
  // the field the bar's reason takes the user to, so the bar never points at
  // "the fields above" without saying which.
  const blocked: { reason: string; fieldId?: string } = !chosen
    ? { reason: t("services.createBlockedSource") }
    : !name.nameValid
      ? { reason: t("common.createBlockedName"), fieldId: "svc-name" }
      : showNameTaken
        ? { reason: t("common.createBlockedNameTaken"), fieldId: "svc-name" }
        : nativeCommandsMissing(form, shape)
          ? {
              reason: t("services.createBlockedCommands"),
              fieldId: shape.isNativeBuildSource
                ? shape.isCronType
                  ? "svc-command"
                  : "svc-build-source-start-command"
                : form.buildCommand.trim() === ""
                  ? "svc-build-command"
                  : shape.isCronType
                    ? "svc-command"
                    : "svc-start-command",
            }
          : shape.isCronType &&
              (form.schedule.trim() === "" || scheduleProblem !== null)
            ? {
                reason: t("services.createBlockedSchedule"),
                fieldId: "svc-schedule",
              }
            : shape.isStaticType && publishPathMissing
              ? {
                  reason: t("services.createBlockedPublishPath"),
                  fieldId: "svc-publish-path",
                }
              : portError
                ? {
                    reason: t("services.createBlockedPort"),
                    fieldId: "svc-port",
                  }
                : // Waiting on the catalog is no fault of the user's.
                  shape.showPlan && form.plan === "" && catalog.loading
                  ? { reason: t("common.createBlockedLoadingPlans") }
                  : { reason: t("common.createBlockedFields") };

  async function handleSubmit() {
    if (!canSubmit) return;
    const result = await create(buildCreateServiceInput(form));
    if (!result) return;
    void (result.deployId
      ? navigate({
          to: "/services/$serviceId/deploys/$deployId",
          params: { serviceId: result.id, deployId: result.deployId },
        })
      : navigate({
          to: "/services/$serviceId",
          params: { serviceId: result.id },
        }));
  }

  return (
    <DashboardLayout>
      <PageFrame
        width="standard"
        header={<ServiceCreateHeader serviceType={form.serviceType} />}
        footer={
          revealed ? (
            <CreateActionBar
              submitLabel={t(createCopy.submitKey)}
              onSubmit={() => void handleSubmit()}
              canSubmit={canSubmit}
              busy={busy}
              cancelLabel={t("services.createCancel")}
              onCancel={() => void navigate({ to: "/" })}
              summary={
                shape.showPlan ? (
                  <PlanSummary
                    plan={selectedPlan}
                    runTimePricing={shape.isCronType}
                    loading={catalog.loading}
                  />
                ) : null
              }
              blocked={blocked}
            />
          ) : undefined
        }
      >
        <CreateForm>
          <CreateField label={t("services.createSourceTitle")}>
            {form.tab === "github" && form.selectedRepo && !editingSource ? (
              <CreateSourceSummary
                repo={form.selectedRepo}
                branch={form.branch || form.selectedRepo.defaultBranch}
                onEdit={() => setEditingSource(true)}
              />
            ) : (
              <ServiceSourcePicker
                titleKey={null}
                tab={form.tab}
                onTabChange={setTab}
                selectedRepo={form.selectedRepo}
                onSelectRepo={(selectedRepo) => {
                  set({ selectedRepo });
                  setEditingSource(false);
                }}
                gitUrl={form.gitUrl}
                onGitUrlChange={(gitUrl) => set({ gitUrl })}
                onGitHubPage={(page) =>
                  set((current) => ({
                    branch: current.branch || page.branch,
                    rootDir: current.rootDir || page.rootDir,
                  }))
                }
                // A static site has no image source (ADR029): omitting the
                // prop is what removes the Existing Image tab, same as the
                // Blueprint caller.
                image={
                  shape.isStaticType
                    ? undefined
                    : {
                        value: form.image,
                        onChange: (image) => {
                          clearImageRefusal();
                          set({ image });
                        },
                        serverError: imageRefusal,
                        registryCredentialId: form.registryCredentialId,
                        onRegistryCredentialChange: (registryCredentialId) =>
                          set({ registryCredentialId }),
                        // The Port row below explains the image's port.
                        showPortHint: false,
                      }
                }
                buildSource={
                  shape.isStaticType ? undefined : (
                    <BuildSourceCreatePicker
                      sources={buildSources.sources}
                      loading={buildSources.loading}
                      failed={buildSources.error !== undefined}
                      value={form.buildSourceId ?? ""}
                      selected={form.selectedBuildSource ?? null}
                      onValueChange={selectBuildSource}
                    />
                  )
                }
              />
            )}
          </CreateField>

          {revealed ? (
            <>
              <CreateField
                label={t("services.createFieldName")}
                htmlFor="svc-name"
                description={t("services.createFieldNameDescription")}
              >
                {/* The availability check runs as the name is typed: its
                    progress is a spinner inside the input and a status for
                    assistive technology, so the rows below never jump while
                    it runs; only a result that needs action takes a line. */}
                <div className="relative">
                  <Input
                    id="svc-name"
                    value={name.name}
                    onChange={(e) => {
                      name.editName(e.target.value);
                      clearNameConflict();
                    }}
                    placeholder={t("services.createFieldNamePlaceholder")}
                    autoComplete="off"
                    aria-invalid={showNameError || showNameTaken}
                    aria-describedby={
                      showNameError || showNameTaken
                        ? "svc-name-error"
                        : undefined
                    }
                    className={`${CREATE_INPUT_CLASS} pr-9`}
                  />
                  {name.checkingName && !showNameError && !showNameTaken ? (
                    <Loader2
                      aria-hidden
                      className="text-muted-foreground absolute top-1/2 right-3 size-4 -translate-y-1/2 animate-spin motion-reduce:animate-none"
                    />
                  ) : null}
                </div>
                <span role="status" className="sr-only">
                  {name.checkingName && !showNameError && !showNameTaken
                    ? t("services.createFieldNameChecking")
                    : ""}
                </span>
                {name.resourceNamePreview !== null ? (
                  <p className="text-sm text-muted-foreground">
                    {t("services.createFieldNameResourceName", {
                      name: name.resourceNamePreview,
                    })}
                  </p>
                ) : null}
                {showNameError ? (
                  <p
                    id="svc-name-error"
                    role="alert"
                    className="text-sm text-destructive"
                  >
                    {t("services.createFieldNameError")}
                  </p>
                ) : showNameTaken ? (
                  <p
                    id="svc-name-error"
                    role="alert"
                    className="flex flex-wrap items-center gap-2 text-sm text-destructive"
                  >
                    <span>{t("services.createFieldNameTaken")}</span>
                    {name.nameTaken && name.nameSuggestion ? (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-6 px-2 text-xs"
                        onClick={name.acceptSuggestion}
                      >
                        {t("services.createFieldNameUseSuggestion", {
                          name: name.nameSuggestion,
                        })}
                      </Button>
                    ) : null}
                  </p>
                ) : null}
              </CreateField>

              <ProjectEnvironmentSelector
                projectId={form.projectId}
                environmentId={form.environmentId}
                onProjectChange={(projectId) => set({ projectId })}
                onEnvironmentChange={(environmentId) => set({ environmentId })}
              />

              {shape.isBuildableGit ? (
                <CreateSelectField
                  id="svc-runtime"
                  label={t("services.createFieldRuntime")}
                  description={t("services.createFieldRuntimeDescription")}
                  value={form.runtime}
                  onValueChange={(value) =>
                    build.setRuntime(value as GitRuntime)
                  }
                >
                  {RUNTIME_DEFS.map(({ id, labelKey }) => (
                    <SelectItem key={id} value={id}>
                      {t(labelKey)}
                    </SelectItem>
                  ))}
                </CreateSelectField>
              ) : null}

              {shape.isGitSource ? (
                <>
                  <CreateTextField
                    id="svc-branch"
                    label={t("services.createFieldBranch")}
                    optional
                    value={form.branch}
                    onChange={(branch) => set({ branch })}
                    placeholder={
                      (form.tab === "github" &&
                        form.selectedRepo?.defaultBranch) ||
                      t("services.createFieldBranchPlaceholder")
                    }
                    description={t("services.createFieldBranchDescription")}
                  />
                  <CreateTextField
                    id="svc-rootdir"
                    label={t("services.createFieldRootDir")}
                    optional
                    mono
                    value={form.rootDir}
                    onChange={(rootDir) => set({ rootDir })}
                    placeholder={t("services.createFieldRootDirPlaceholder")}
                    description={t("services.createFieldRootDirHint")}
                    error={
                      fieldErrors.rootDir ? t(fieldErrors.rootDir) : undefined
                    }
                  />
                </>
              ) : null}

              {shape.isNativeBuild ? (
                <>
                  <CreateTextField
                    id="svc-build-command"
                    label={t("services.createFieldBuildCommand")}
                    mono
                    value={form.buildCommand}
                    onChange={build.setBuildCommand}
                    description={t(
                      "services.createFieldBuildCommandDescription",
                    )}
                  />
                  {!shape.isCronType ? (
                    <CreateTextField
                      id="svc-start-command"
                      label={t("services.createFieldStartCommand")}
                      mono
                      value={form.startCommand}
                      onChange={build.setStartCommand}
                      description={t(
                        "services.createFieldStartCommandDescription",
                      )}
                    />
                  ) : null}
                </>
              ) : null}

              {shape.isNativeBuildSource && !shape.isCronType ? (
                <CreateTextField
                  id="svc-build-source-start-command"
                  label={t("services.createFieldStartCommand")}
                  mono
                  value={form.buildSourceStartCommand ?? ""}
                  onChange={(buildSourceStartCommand) =>
                    set({ buildSourceStartCommand })
                  }
                  description={t("services.sourceBuildSourceStartHint")}
                />
              ) : null}

              {shape.isCronType ? (
                <>
                  <CreateTextField
                    id="svc-schedule"
                    label={t("services.createFieldSchedule")}
                    mono
                    value={form.schedule}
                    onChange={(schedule) => set({ schedule })}
                    placeholder={t("services.createFieldSchedulePlaceholder")}
                    hint={
                      scheduleDescription
                        ? t("services.createFieldSchedulePreview", {
                            description: scheduleDescription,
                          })
                        : t("services.createFieldScheduleHint")
                    }
                    error={
                      scheduleProblem === "never_fires"
                        ? t("services.cronScheduleNeverFires")
                        : scheduleProblem === "format"
                          ? t("services.createFieldScheduleError")
                          : undefined
                    }
                  />
                  <CreateTextField
                    id="svc-command"
                    label={t("services.createFieldStartCommand")}
                    // A native build (git or native Build Source) needs it; an
                    // image or Dockerfile has its own default command.
                    optional={
                      !(shape.isNativeBuild || shape.isNativeBuildSource)
                    }
                    mono
                    value={form.command}
                    onChange={(command) => set({ command })}
                    placeholder={t("services.createFieldCommandPlaceholder")}
                    description={t(
                      shape.isNativeBuildSource
                        ? "services.sourceBuildSourceStartHint"
                        : "services.createFieldCommandHint",
                    )}
                  />
                </>
              ) : null}

              {shape.isStaticBuild ? (
                <CreateTextField
                  id="svc-static-build-command"
                  label={t("services.createFieldBuildCommand")}
                  optional
                  mono
                  value={form.staticBuildCommand}
                  onChange={(staticBuildCommand) => set({ staticBuildCommand })}
                  placeholder={t("services.buildCommandPlaceholder")}
                  description={t("services.buildCommandHint")}
                />
              ) : null}

              {shape.isStaticType ? (
                <CreateTextField
                  id="svc-publish-path"
                  label={t("services.createFieldPublishPath")}
                  mono
                  value={form.publishPath}
                  onChange={(publishPath) => set({ publishPath })}
                  placeholder={t("services.createFieldPublishPathPlaceholder")}
                  required
                  description={t("services.createFieldPublishPathHint")}
                  error={
                    fieldErrors.publishPath
                      ? t(fieldErrors.publishPath)
                      : undefined
                  }
                />
              ) : null}

              {/* Port (w4/m121/t003): the container port bex routes to and
                  injects as $PORT — bex's own row, kept in the main flow
                  beside the start command because a wrong port fails the
                  deploy. Only the port-binding types (web/private) get it:
                  `shape.showPort` also gates the image tab's $PORT hint, so
                  a worker, a cron job and a static site submit no `port`. */}
              {shape.showPort ? (
                <CreateTextField
                  id="svc-port"
                  label={t("services.createFieldPort")}
                  optional
                  value={port}
                  onChange={(port) => set({ port })}
                  placeholder={
                    shape.imagePorts
                      ? t("services.createFieldPortFromImage")
                      : String(DEFAULT_SERVICE_PORT)
                  }
                  description={
                    shape.imagePorts
                      ? t(
                          form.serviceType === "web_service"
                            ? "services.createFieldPortImageHintWeb"
                            : "services.createFieldPortImageHint",
                        )
                      : t("services.createFieldPortHint", {
                          port: port.trim() || String(DEFAULT_SERVICE_PORT),
                        })
                  }
                  error={
                    portError
                      ? t("services.portRangeError", {
                          min: String(MIN_SERVICE_PORT),
                          max: String(MAX_SERVICE_PORT),
                        })
                      : undefined
                  }
                />
              ) : null}

              {shape.showPlan ? (
                <CreateField
                  label={t("services.createFieldPlan")}
                  description={
                    shape.isCronType
                      ? t("services.createFieldPlanCronHint", {
                          minimum: CRON_MINIMUM_USD,
                        })
                      : t("services.createFieldPlanDescription")
                  }
                >
                  {instanceTypes.length === 0 && !catalog.loading ? (
                    <PlanCatalogUnavailable
                      onRetry={() => void catalog.retry()}
                      retrying={catalog.retrying}
                    />
                  ) : instanceTypes.length === 0 ? (
                    <PlanTierSlots
                      count={servicePlanSlotCount(form.serviceType)}
                      collapsedCount={CREATE_PLAN_ROWS}
                    />
                  ) : (
                    <ComputePlanList
                      instanceTypes={instanceTypes}
                      value={form.plan}
                      ariaLabel={t("services.createFieldPlan")}
                      onChange={(planOverride) => set({ planOverride })}
                      runTimePricing={shape.isCronType}
                      collapsedCount={CREATE_PLAN_ROWS}
                    />
                  )}
                </CreateField>
              ) : null}

              <CreateEnvVarEditor
                rows={form.envVars}
                onChange={(envVars) => set({ envVars })}
              />

              <CreateAdvanced forceOpen={advancedInvalid}>
                <CreateSecretFileEditor
                  rows={form.secretFiles}
                  onChange={(secretFiles) => set({ secretFiles })}
                />

                {shape.isBuildableGit && !shape.isNativeBuild ? (
                  <>
                    <CreateTextField
                      id="svc-dockerfile-path"
                      label={t("services.createFieldDockerfilePath")}
                      mono
                      value={form.dockerfilePath}
                      onChange={build.setDockerfilePath}
                      placeholder={t(
                        "services.createFieldDockerfilePathPlaceholder",
                      )}
                      description={t("services.createFieldDockerfilePathHint")}
                      error={
                        fieldErrors.dockerfilePath
                          ? t(fieldErrors.dockerfilePath)
                          : undefined
                      }
                    />
                    <CreateField
                      label={t("services.createRegistryCredentialLabel")}
                      htmlFor="svc-registry-credential-docker"
                    >
                      <RegistryCredentialSelect
                        id="svc-registry-credential-docker"
                        hideLabel
                        value={form.registryCredentialId}
                        onValueChange={(registryCredentialId) =>
                          set({ registryCredentialId })
                        }
                        description={t(
                          "services.createRegistryCredentialDescription",
                        )}
                      />
                    </CreateField>
                    {!shape.isCronType ? (
                      <CreateTextField
                        id="svc-docker-command"
                        label={t("services.createFieldDockerCommand")}
                        mono
                        value={form.startCommand}
                        onChange={build.setStartCommand}
                        placeholder={t(
                          "services.createFieldDockerCommandPlaceholder",
                        )}
                        description={t(
                          "services.createFieldDockerCommandDescription",
                        )}
                      />
                    ) : null}
                  </>
                ) : null}

                {/* Cron jobs set theirs through Command above. */}
                {shape.showImageCommand ? (
                  <CreateTextField
                    id="svc-image-docker-command"
                    label={t("services.createFieldDockerCommand")}
                    mono
                    value={form.imageCommand}
                    onChange={(imageCommand) => set({ imageCommand })}
                    placeholder={t("services.imageCommandPlaceholder")}
                    description={t(
                      "services.createFieldDockerCommandDescription",
                    )}
                  />
                ) : null}

                {shape.showBuildSourceDockerCommand ? (
                  <CreateTextField
                    id="svc-build-source-docker-command"
                    label={t("services.createFieldDockerCommand")}
                    optional
                    mono
                    value={form.buildSourceDockerCommand ?? ""}
                    onChange={(buildSourceDockerCommand) =>
                      set({ buildSourceDockerCommand })
                    }
                    placeholder={t("services.imageCommandPlaceholder")}
                    description={t(
                      "services.createFieldDockerCommandDescription",
                    )}
                  />
                ) : null}

                {shape.isGitSource || shape.isBuildSource ? (
                  <CreateField
                    label={t("services.createFieldAutoDeploy")}
                    htmlFor="svc-autodeploy"
                    description={t(
                      shape.isBuildSource
                        ? "services.createBuildSourceAutoDeployHint"
                        : "services.createFieldAutoDeployHint",
                    )}
                  >
                    <Switch
                      className={CREATE_SWITCH_CLASS}
                      id="svc-autodeploy"
                      checked={form.autoDeploy}
                      onCheckedChange={(autoDeploy) => set({ autoDeploy })}
                    />
                  </CreateField>
                ) : null}

                {shape.isStaticBuild ? (
                  <CreateField
                    label={t("services.buildFilterLabel")}
                    description={t("services.buildFilterHint")}
                  >
                    <div className="space-y-6">
                      <PathList
                        title={t("services.buildFilterIncludedTitle")}
                        hint={t("services.buildFilterIncludedHint")}
                        placeholder={t(
                          "services.buildFilterIncludedPlaceholder",
                        )}
                        addLabel={t("services.buildFilterAddIncluded")}
                        removeLabel={t("services.buildFilterRemoveIncluded")}
                        values={form.buildFilterPaths}
                        onChange={(buildFilterPaths) =>
                          set({ buildFilterPaths })
                        }
                        validate={patternError}
                      />
                      <PathList
                        title={t("services.buildFilterIgnoredTitle")}
                        hint={t("services.buildFilterIgnoredHint")}
                        placeholder={t(
                          "services.buildFilterIgnoredPlaceholder",
                        )}
                        addLabel={t("services.buildFilterAddIgnored")}
                        removeLabel={t("services.buildFilterRemoveIgnored")}
                        values={form.buildFilterIgnored}
                        onChange={(buildFilterIgnored) =>
                          set({ buildFilterIgnored })
                        }
                        validate={patternError}
                      />
                    </div>
                  </CreateField>
                ) : null}
              </CreateAdvanced>

              {capLimit ? (
                <CreateCapLimitAlert
                  title={t("services.capLimitTitle")}
                  message={capLimit}
                  upgradeLabel={t("services.capLimitUpgrade")}
                />
              ) : null}
            </>
          ) : null}
        </CreateForm>
      </PageFrame>
    </DashboardLayout>
  );
}

/**
 * Build Source selector inside `/services/new`'s `ServiceSourcePicker`
 * (`build-source` tab, w2/m612 t003). Shows workspace Build Sources with kind
 * and repo/image info, plus a region-match notice for git-backed sources.
 */
function BuildSourceCreatePicker({
  sources,
  loading,
  failed,
  value,
  selected,
  onValueChange,
}: {
  sources: BuildSourceOption[];
  loading: boolean;
  failed: boolean;
  value: string;
  selected: BuildSourceOption | null;
  onValueChange: (id: string) => void;
}) {
  const { t } = useTranslations();
  if (loading && sources.length === 0) {
    return (
      <div className="space-y-2" aria-busy="true">
        <Skeleton className="h-4 w-24" />
        <Skeleton className={cn(CREATE_INPUT_CLASS, "w-full rounded-md")} />
      </div>
    );
  }
  if (sources.length === 0) {
    return (
      <p
        role={failed ? "alert" : undefined}
        className="rounded-lg border p-4 text-sm text-muted-foreground"
      >
        {t(
          failed
            ? "services.sourceBuildSourceLoadError"
            : "services.sourceBuildSourceEmpty",
        )}
      </p>
    );
  }
  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label htmlFor="svc-build-source">
          {t("services.sourceBuildSourceLabel")}
        </Label>
        <Select value={value} onValueChange={onValueChange}>
          <SelectTrigger id="svc-build-source" className={CREATE_SELECT_CLASS}>
            <SelectValue
              placeholder={t("services.sourceBuildSourcePlaceholder")}
            />
          </SelectTrigger>
          <SelectContent>
            {sources.map((source) => (
              <SelectItem key={source.id} value={source.id}>
                <span>{source.name}</span>
                {source.source ? (
                  <span className="ml-2 font-mono text-xs text-muted-foreground">
                    {buildSourceDisplayLabel(source)}
                  </span>
                ) : null}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {selected ? (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
          <span>
            {t(
              selected.kind === "git"
                ? "services.buildSourceKindGit"
                : "services.buildSourceKindImage",
            )}
          </span>
          {selected.source ? (
            <span className="font-mono break-all">
              {buildSourceDisplayLabel(selected)}
            </span>
          ) : null}
          <span className="font-mono">{selected.id}</span>
        </div>
      ) : null}
      {selected?.kind === "git" && selected.region ? (
        <p
          className="text-xs text-muted-foreground"
          data-testid="build-source-region-notice"
        >
          {t("services.createBuildSourceRegionNotice", {
            region: selected.region,
          })}
        </p>
      ) : null}
    </div>
  );
}
