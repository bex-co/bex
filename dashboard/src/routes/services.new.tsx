import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader2, ArrowUpRight } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Skeleton } from "@/common/components/ui/skeleton";
import { TextField } from "@/common/components/text-field";
import { PlanCardGrid } from "@/common/components/plan-card-grid";
import { useCreateService } from "@/features/services/hooks/use-create-service";
import { describeCron, isValidCron } from "@/features/services/lib/cron";
import { ProjectEnvironmentSelector } from "@/features/environments/components/project-environment-selector";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import { PathList } from "@/features/services/components/build-deploy-section";
import { ServiceTypePicker } from "@/features/services/components/service-type-picker";
import { useNewServiceForm } from "@/features/services/hooks/use-new-service-form";
import { RUNTIME_DEFS, type GitRuntime } from "@/features/services/lib/runtime";
import { ServiceSourcePicker } from "@/features/services/components/service-source-picker";
import { CreateEnvVarEditor } from "@/features/services/components/create-env-var-editor";
import { CreateSecretFileEditor } from "@/features/services/components/create-secret-file-editor";
import {
  buildCreateServiceInput,
  buildShape,
  isSubmittable,
} from "@/features/services/lib/create-service-input";
import {
  parseNewServiceSearch,
  serviceTypeCreateCopy,
} from "@/features/services/lib/create-context";
import { ServiceCreatePageSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/new")({
  staticData: { chrome: true },
  component: NewServicePage,
  pendingComponent: ServiceCreatePageSkeleton,
  beforeLoad: requireAuth(),
  validateSearch: parseNewServiceSearch,
  // Same resolver as the on-page heading, so tab title and <h1> always agree —
  // including on a bare /services/new, where both fall to DEFAULT_SERVICE_TYPE.
  head: ({ match }) =>
    translatedTitleHead(
      serviceTypeCreateCopy(match.search?.type).titleKey,
      match,
    ),
});

export function NewServicePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const search = Route.useSearch();
  const { create, busy, capLimit, nameConflict, clearNameConflict } =
    useCreateService();
  const { form, set, setTab, build, name, instanceTypes } =
    useNewServiceForm(search);
  const shape = buildShape(form);
  const createCopy = serviceTypeCreateCopy(form.serviceType);

  const scheduleError =
    shape.isCronType &&
    form.schedule.trim() !== "" &&
    !isValidCron(form.schedule);
  const scheduleDescription = shape.isCronType
    ? describeCron(form.schedule)
    : null;
  const showNameError = name.name.length > 0 && !name.nameValid;
  const showNameTaken = name.nameValid && (name.nameTaken || nameConflict);
  const canSubmit =
    name.nameValid && !showNameTaken && !busy && isSubmittable(form);

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
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>
                <h1 className="text-xl">{t(createCopy.titleKey)}</h1>
              </CardTitle>
              <CardDescription>{t(createCopy.descriptionKey)}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-3">
                <Label>{t("services.createTypePickerTitle")}</Label>
                <ServiceTypePicker
                  value={form.serviceType}
                  onChange={(serviceType) => set({ serviceType })}
                />
              </div>

              <ServiceSourcePicker
                tab={form.tab}
                onTabChange={setTab}
                selectedRepo={form.selectedRepo}
                onSelectRepo={(selectedRepo) => set({ selectedRepo })}
                gitUrl={form.gitUrl}
                onGitUrlChange={(gitUrl) => set({ gitUrl })}
                image={{
                  value: form.image,
                  onChange: (image) => set({ image }),
                  registryCredentialId: form.registryCredentialId,
                  onRegistryCredentialChange: (registryCredentialId) =>
                    set({ registryCredentialId }),
                  showPortHint: shape.showPortHint,
                }}
              />

              <div className="space-y-4">
                <p className="text-base font-semibold">
                  {t("services.createSettingsTitle")}
                </p>

                <div className="space-y-2">
                  <Label htmlFor="svc-name">
                    {t("services.createFieldName")}
                  </Label>
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
                  />
                  {showNameError ? (
                    <p className="text-sm text-destructive">
                      {t("services.createFieldNameError")}
                    </p>
                  ) : showNameTaken ? (
                    <p className="flex flex-wrap items-center gap-2 text-sm text-destructive">
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
                  ) : name.checkingName ? (
                    <p className="text-sm text-muted-foreground">
                      {t("services.createFieldNameChecking")}
                    </p>
                  ) : null}
                </div>

                {shape.isGitSource ? (
                  <>
                    <TextField
                      id="svc-branch"
                      label={t("services.createFieldBranch")}
                      value={form.branch}
                      onChange={(branch) => set({ branch })}
                      placeholder={t("services.createFieldBranchPlaceholder")}
                    />
                    <TextField
                      id="svc-rootdir"
                      label={t("services.createFieldRootDir")}
                      value={form.rootDir}
                      onChange={(rootDir) => set({ rootDir })}
                      placeholder={t("services.createFieldRootDirPlaceholder")}
                      hint={t("services.createFieldRootDirHint")}
                    />
                    {shape.isBuildableGit ? (
                      <>
                        <div className="space-y-2">
                          <Label htmlFor="svc-runtime">
                            {t("services.createFieldRuntime")}
                          </Label>
                          <Select
                            value={form.runtime}
                            onValueChange={(value) =>
                              build.setRuntime(value as GitRuntime)
                            }
                          >
                            <SelectTrigger id="svc-runtime" className="w-full">
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
                        {shape.isNativeBuild ? (
                          <>
                            <TextField
                              id="svc-build-command"
                              label={t("services.createFieldBuildCommand")}
                              value={form.buildCommand}
                              onChange={build.setBuildCommand}
                            />
                            {!shape.isCronType ? (
                              <TextField
                                id="svc-start-command"
                                label={t("services.createFieldStartCommand")}
                                value={form.startCommand}
                                onChange={build.setStartCommand}
                              />
                            ) : null}
                          </>
                        ) : (
                          <>
                            <TextField
                              id="svc-dockerfile-path"
                              label={t("services.createFieldDockerfilePath")}
                              value={form.dockerfilePath}
                              onChange={build.setDockerfilePath}
                              placeholder={t(
                                "services.createFieldDockerfilePathPlaceholder",
                              )}
                              hint={t("services.createFieldDockerfilePathHint")}
                            />
                            <RegistryCredentialSelect
                              id="svc-registry-credential-docker"
                              value={form.registryCredentialId}
                              onValueChange={(registryCredentialId) =>
                                set({ registryCredentialId })
                              }
                              description={t(
                                "services.createRegistryCredentialDescription",
                              )}
                            />
                            {!shape.isCronType ? (
                              <TextField
                                id="svc-docker-command"
                                label={t("services.createFieldDockerCommand")}
                                value={form.startCommand}
                                onChange={build.setStartCommand}
                                placeholder={t(
                                  "services.createFieldDockerCommandPlaceholder",
                                )}
                              />
                            ) : null}
                          </>
                        )}
                      </>
                    ) : null}
                  </>
                ) : null}

                {shape.isCronType ? (
                  <>
                    <TextField
                      id="svc-schedule"
                      label={t("services.createFieldSchedule")}
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
                        scheduleError
                          ? t("services.createFieldScheduleError")
                          : undefined
                      }
                    />
                    <TextField
                      id="svc-command"
                      label={t("services.createFieldStartCommand")}
                      value={form.command}
                      onChange={(command) => set({ command })}
                      placeholder={t("services.createFieldCommandPlaceholder")}
                      hint={t("services.createFieldCommandHint")}
                    />
                  </>
                ) : null}

                {shape.isStaticBuild ? (
                  <>
                    <TextField
                      id="svc-static-build-command"
                      label={t("services.createFieldBuildCommand")}
                      value={form.staticBuildCommand}
                      onChange={(staticBuildCommand) =>
                        set({ staticBuildCommand })
                      }
                      placeholder={t("services.buildCommandPlaceholder")}
                      hint={t("services.buildCommandHint")}
                    />
                    <div className="space-y-4 rounded-md border p-4">
                      <div>
                        <div className="text-sm font-medium">
                          {t("services.buildFilterLabel")}
                        </div>
                        <div className="mt-1 text-xs text-muted-foreground">
                          {t("services.buildFilterHint")}
                        </div>
                      </div>
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
                      />
                    </div>
                  </>
                ) : null}

                {shape.isStaticType ? (
                  <TextField
                    id="svc-publish-path"
                    label={t("services.createFieldPublishPath")}
                    value={form.publishPath}
                    onChange={(publishPath) => set({ publishPath })}
                    placeholder={t(
                      "services.createFieldPublishPathPlaceholder",
                    )}
                    hint={t("services.createFieldPublishPathHint")}
                  />
                ) : null}

                {shape.showNoUrlNote ? (
                  <p className="text-sm text-muted-foreground">
                    {t("services.createNoPublicUrlNote")}
                  </p>
                ) : null}

                {shape.showPlan ? (
                  <div className="space-y-2">
                    <Label>{t("services.createFieldPlan")}</Label>
                    {instanceTypes.length === 0 ? (
                      <Skeleton className="h-20 w-full" />
                    ) : (
                      <PlanCardGrid
                        instanceTypes={instanceTypes}
                        value={form.plan}
                        onChange={(planOverride) => set({ planOverride })}
                      />
                    )}
                  </div>
                ) : null}

                <ProjectEnvironmentSelector
                  projectId={form.projectId}
                  environmentId={form.environmentId}
                  onProjectChange={(projectId) => set({ projectId })}
                  onEnvironmentChange={(environmentId) =>
                    set({ environmentId })
                  }
                />

                {shape.isGitSource ? (
                  <div className="flex items-center justify-between rounded-md border p-3">
                    <div className="space-y-0.5 pr-4">
                      <Label htmlFor="svc-autodeploy">
                        {t("services.createFieldAutoDeploy")}
                      </Label>
                      <p className="text-sm text-muted-foreground">
                        {t("services.createFieldAutoDeployHint")}
                      </p>
                    </div>
                    <Switch
                      id="svc-autodeploy"
                      checked={form.autoDeploy}
                      onCheckedChange={(autoDeploy) => set({ autoDeploy })}
                    />
                  </div>
                ) : null}

                <CreateEnvVarEditor
                  rows={form.envVars}
                  onChange={(envVars) => set({ envVars })}
                />
                <CreateSecretFileEditor
                  rows={form.secretFiles}
                  onChange={(secretFiles) => set({ secretFiles })}
                />
              </div>

              {capLimit ? (
                <Alert variant="destructive">
                  <AlertTitle>{t("services.capLimitTitle")}</AlertTitle>
                  <AlertDescription className="flex flex-col gap-2">
                    <span>{capLimit}</span>
                    <Button
                      variant="outline"
                      size="sm"
                      className="self-start"
                      onClick={() =>
                        void navigate({
                          to: "/workspace/settings",
                          search: { plan: "change" },
                        })
                      }
                    >
                      <ArrowUpRight className="size-3.5" />
                      {t("services.capLimitUpgrade")}
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : null}

              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  onClick={() => void navigate({ to: "/" })}
                  disabled={busy}
                >
                  {t("services.createCancel")}
                </Button>
                <Button
                  onClick={() => void handleSubmit()}
                  disabled={!canSubmit}
                >
                  {busy ? <Loader2 className="animate-spin" /> : null}
                  {t("services.createSubmit")}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </DashboardLayout>
  );
}
