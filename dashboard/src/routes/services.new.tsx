import { useState } from "react";
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
import { isValidGitUrl } from "@/common/lib/utils/git-url";
import { PlanCardGrid } from "@/common/components/plan-card-grid";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import {
  VALID_ENV_KEY,
  isValidSecretFileName,
} from "@/features/services/lib/environment-draft";
import { useCreateService } from "@/features/services/hooks/use-create-service";
import type {
  EnvVarEntry,
  SecretFileEntry,
} from "@/features/services/hooks/use-create-service";
import { isValidCron } from "@/features/services/lib/cron";
import type { RepoView } from "@/features/services/hooks/use-repos";
import { ProjectEnvironmentSelector } from "@/features/environments/components/project-environment-selector";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import { PathList } from "@/features/services/components/build-deploy-section";
import { ServiceTypePicker } from "@/features/services/components/service-type-picker";
import { useBuildRuntimeFields } from "@/features/services/hooks/use-build-runtime-fields";
import { useServiceNameDraft } from "@/features/services/hooks/use-service-name-draft";
import { RUNTIME_DEFS, type GitRuntime } from "@/features/services/lib/runtime";
import {
  ServiceSourcePicker,
  type SourceTab,
} from "@/features/services/components/service-source-picker";
import { CreateEnvVarEditor } from "@/features/services/components/create-env-var-editor";
import { CreateSecretFileEditor } from "@/features/services/components/create-secret-file-editor";
import {
  parseNewServiceSearch,
  type ServiceType,
} from "@/features/services/lib/create-context";

export const Route = createFileRoute("/services/new")({
  staticData: { chrome: true },
  component: NewServicePage,
  beforeLoad: requireAuth(),
  validateSearch: parseNewServiceSearch,
  head: ({ match }) => translatedTitleHead("services.createTitle", match),
});

export function NewServicePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const search = Route.useSearch();
  const { instanceTypes } = useInstanceTypes();
  const { create, busy, capLimit, nameConflict, clearNameConflict } =
    useCreateService();
  const [serviceType, setServiceType] = useState<ServiceType>(
    search.type ?? "web_service",
  );
  const [tab, setTab] = useState<SourceTab>("github");
  const [selectedRepo, setSelectedRepo] = useState<RepoView | null>(null);
  const [gitUrl, setGitUrl] = useState("");
  const [imageVal, setImageVal] = useState("");
  const [registryCredentialId, setRegistryCredentialId] = useState("");
  const [branch, setBranch] = useState("");
  const [rootDir, setRootDir] = useState("");
  const {
    runtime,
    setRuntime,
    buildCommand,
    setBuildCommand,
    startCommand,
    setStartCommand,
    dockerfilePath,
    setDockerfilePath,
  } = useBuildRuntimeFields();
  const [planOverride, setPlanOverride] = useState<string | null>(null);
  const [autoDeploy, setAutoDeploy] = useState(true);
  const [schedule, setSchedule] = useState("");
  const [command, setCommand] = useState("");
  const [publishPath, setPublishPath] = useState("");
  const [staticBuildCommand, setStaticBuildCommand] = useState("");
  const [buildFilterPaths, setBuildFilterPaths] = useState<string[]>([]);
  const [buildFilterIgnored, setBuildFilterIgnored] = useState<string[]>([]);
  const [envVars, setEnvVars] = useState<EnvVarEntry[]>([]);
  const [secretFiles, setSecretFiles] = useState<SecretFileEntry[]>([]);
  const [projectId, setProjectId] = useState<string | null>(
    search.projectId ?? null,
  );
  const [environmentId, setEnvironmentId] = useState<string | null>(
    search.environmentId ?? null,
  );

  const isCronType = serviceType === "cron_job";
  const isStaticType = serviceType === "static_site";
  const showNoUrlNote =
    serviceType === "private_service" || serviceType === "background_worker";
  const showPlan = !isStaticType;

  const plan = planOverride ?? instanceTypes[0]?.id ?? "";
  const isImageSource = tab === "image";
  const isGitSource = tab === "github" || tab === "git";
  // The four build shapes the form submits. Named once here because both the
  // field JSX below and the create() payload have to agree on them exactly —
  // re-deriving each condition inline is how a static site ends up submitting
  // a dockerfilePath.
  const isBuildableGit = isGitSource && !isStaticType;
  const isDockerBuild = isBuildableGit && runtime === "docker";
  const isNativeBuild = isBuildableGit && runtime !== "docker";
  const isStaticBuild = isGitSource && isStaticType;
  const usesRegistryCredential = isImageSource || isDockerBuild;

  const scheduleError =
    isCronType && schedule.trim() !== "" && !isValidCron(schedule);

  const {
    name,
    nameValid,
    nameTaken,
    nameSuggestion,
    checkingName,
    editName,
    acceptSuggestion,
  } = useServiceNameDraft({
    tab,
    selectedRepo,
    gitUrl,
    image: imageVal,
    onRepoDefaultBranch: (b) => setBranch((cur) => cur || b),
  });

  const showNameError = name.length > 0 && !nameValid;
  const showNameTaken = nameValid && (nameTaken || nameConflict);

  const sourceValid =
    (tab === "github" && selectedRepo != null) ||
    (tab === "git" && isValidGitUrl(gitUrl)) ||
    (isImageSource && imageVal.trim().length > 0);

  // Blank rows are the editors' "add" placeholder — they never submit, so they
  // don't block submission either. Everything else must be valid, and the same
  // list that passes the gate is the one that goes over the wire.
  const submittableEnvVars = envVars.filter((r) => r.key !== "");
  const submittableSecretFiles = secretFiles.filter((f) => f.name !== "");
  const envVarsValid = submittableEnvVars.every((r) =>
    VALID_ENV_KEY.test(r.key),
  );
  const secretFilesValid = submittableSecretFiles.every((f) =>
    isValidSecretFileName(f.name),
  );
  const nativeCommandsValid =
    !isNativeBuild ||
    (buildCommand.trim() !== "" &&
      (isCronType ? command.trim() !== "" : startCommand.trim() !== ""));
  const canSubmit =
    nameValid &&
    !showNameTaken &&
    sourceValid &&
    !busy &&
    envVarsValid &&
    secretFilesValid &&
    nativeCommandsValid &&
    (isStaticType || plan !== "") &&
    (!isCronType || (schedule.trim() !== "" && !scheduleError));

  async function handleSubmit() {
    if (!canSubmit) return;
    let repo: string | undefined;
    let image: string | undefined;
    let branchVal: string | undefined;

    if (tab === "github" && selectedRepo) {
      repo = selectedRepo.cloneUrl;
      branchVal = branch || selectedRepo.defaultBranch || undefined;
    } else if (tab === "git") {
      repo = gitUrl.trim();
      branchVal = branch || undefined;
    } else {
      image = imageVal.trim();
    }

    const hasBuildFilter =
      buildFilterPaths.some((p) => p.trim()) ||
      buildFilterIgnored.some((p) => p.trim());
    const result = await create({
      name,
      type: serviceType,
      environmentId: environmentId || undefined,
      repo,
      image,
      registryCredentialId: usesRegistryCredential
        ? registryCredentialId
        : undefined,
      branch: branchVal,
      rootDir: rootDir || undefined,
      runtime: isImageSource ? "image" : isBuildableGit ? runtime : undefined,
      buildCommand: isStaticBuild
        ? staticBuildCommand.trim() || undefined
        : isNativeBuild
          ? buildCommand.trim()
          : undefined,
      startCommand: isBuildableGit
        ? isCronType
          ? command.trim()
          : startCommand.trim() || undefined
        : undefined,
      dockerfilePath: isDockerBuild
        ? dockerfilePath.trim() || undefined
        : undefined,
      buildFilter:
        isStaticBuild && hasBuildFilter
          ? {
              paths: buildFilterPaths.map((p) => p.trim()).filter(Boolean),
              ignoredPaths: buildFilterIgnored
                .map((p) => p.trim())
                .filter(Boolean),
            }
          : undefined,
      plan: showPlan ? plan || undefined : undefined,
      autoDeploy: isGitSource ? autoDeploy : undefined,
      schedule: isCronType ? schedule.trim() || undefined : undefined,
      command: isCronType ? command.trim() || undefined : undefined,
      publishPath: isStaticType ? publishPath.trim() || undefined : undefined,
      envVars: submittableEnvVars.length ? submittableEnvVars : undefined,
      secretFiles: submittableSecretFiles.length
        ? submittableSecretFiles
        : undefined,
    });
    if (result) {
      if (result.deployId) {
        void navigate({
          to: "/services/$serviceId/deploys/$deployId",
          params: { serviceId: result.id, deployId: result.deployId },
        });
      } else {
        void navigate({
          to: "/services/$serviceId",
          params: { serviceId: result.id },
        });
      }
    }
  }

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>
                <h1 className="text-xl font-semibold">
                  {t("services.createTitle")}
                </h1>
              </CardTitle>
              <CardDescription>
                {t("services.createDescription")}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-3">
                <Label>{t("services.createTypePickerTitle")}</Label>
                <ServiceTypePicker
                  value={serviceType}
                  onChange={setServiceType}
                />
              </div>

              <ServiceSourcePicker
                tab={tab}
                onTabChange={(next) => {
                  setTab(next);
                  setSelectedRepo(null);
                  setBranch("");
                }}
                selectedRepo={selectedRepo}
                onSelectRepo={setSelectedRepo}
                gitUrl={gitUrl}
                onGitUrlChange={setGitUrl}
                image={imageVal}
                onImageChange={setImageVal}
                registryCredentialId={registryCredentialId}
                onRegistryCredentialChange={setRegistryCredentialId}
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
                    value={name}
                    onChange={(e) => {
                      editName(e.target.value);
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
                      {nameTaken && nameSuggestion ? (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="h-6 px-2 text-xs"
                          onClick={acceptSuggestion}
                        >
                          {t("services.createFieldNameUseSuggestion", {
                            name: nameSuggestion,
                          })}
                        </Button>
                      ) : null}
                    </p>
                  ) : checkingName ? (
                    <p className="text-sm text-muted-foreground">
                      {t("services.createFieldNameChecking")}
                    </p>
                  ) : null}
                </div>

                {isGitSource ? (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="svc-branch">
                        {t("services.createFieldBranch")}
                      </Label>
                      <Input
                        id="svc-branch"
                        value={branch}
                        onChange={(e) => setBranch(e.target.value)}
                        placeholder={t("services.createFieldBranchPlaceholder")}
                        autoComplete="off"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="svc-rootdir">
                        {t("services.createFieldRootDir")}
                      </Label>
                      <Input
                        id="svc-rootdir"
                        value={rootDir}
                        onChange={(e) => setRootDir(e.target.value)}
                        placeholder={t(
                          "services.createFieldRootDirPlaceholder",
                        )}
                        autoComplete="off"
                      />
                      <p className="text-sm text-muted-foreground">
                        {t("services.createFieldRootDirHint")}
                      </p>
                    </div>
                    {isBuildableGit ? (
                      <>
                        <div className="space-y-2">
                          <Label htmlFor="svc-runtime">
                            {t("services.createFieldRuntime")}
                          </Label>
                          <Select
                            value={runtime}
                            onValueChange={(value) =>
                              setRuntime(value as GitRuntime)
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
                        {isNativeBuild ? (
                          <>
                            <div className="space-y-2">
                              <Label htmlFor="svc-build-command">
                                {t("services.createFieldBuildCommand")}
                              </Label>
                              <Input
                                id="svc-build-command"
                                value={buildCommand}
                                onChange={(e) =>
                                  setBuildCommand(e.target.value)
                                }
                                autoComplete="off"
                              />
                            </div>
                            {!isCronType ? (
                              <div className="space-y-2">
                                <Label htmlFor="svc-start-command">
                                  {t("services.createFieldStartCommand")}
                                </Label>
                                <Input
                                  id="svc-start-command"
                                  value={startCommand}
                                  onChange={(e) =>
                                    setStartCommand(e.target.value)
                                  }
                                  autoComplete="off"
                                />
                              </div>
                            ) : null}
                          </>
                        ) : (
                          <>
                            <div className="space-y-2">
                              <Label htmlFor="svc-dockerfile-path">
                                {t("services.createFieldDockerfilePath")}
                              </Label>
                              <Input
                                id="svc-dockerfile-path"
                                value={dockerfilePath}
                                onChange={(event) =>
                                  setDockerfilePath(event.target.value)
                                }
                                placeholder={t(
                                  "services.createFieldDockerfilePathPlaceholder",
                                )}
                                autoComplete="off"
                              />
                              <p className="text-sm text-muted-foreground">
                                {t("services.createFieldDockerfilePathHint")}
                              </p>
                            </div>
                            <RegistryCredentialSelect
                              id="svc-registry-credential-docker"
                              value={registryCredentialId}
                              onValueChange={setRegistryCredentialId}
                              description={t(
                                "services.createRegistryCredentialDescription",
                              )}
                            />
                            {!isCronType ? (
                              <div className="space-y-2">
                                <Label htmlFor="svc-docker-command">
                                  {t("services.createFieldDockerCommand")}
                                </Label>
                                <Input
                                  id="svc-docker-command"
                                  value={startCommand}
                                  onChange={(event) =>
                                    setStartCommand(event.target.value)
                                  }
                                  placeholder={t(
                                    "services.createFieldDockerCommandPlaceholder",
                                  )}
                                  autoComplete="off"
                                />
                              </div>
                            ) : null}
                          </>
                        )}
                      </>
                    ) : null}
                  </>
                ) : null}

                {isCronType ? (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="svc-schedule">
                        {t("services.createFieldSchedule")}
                      </Label>
                      <Input
                        id="svc-schedule"
                        value={schedule}
                        onChange={(e) => setSchedule(e.target.value)}
                        placeholder={t(
                          "services.createFieldSchedulePlaceholder",
                        )}
                        autoComplete="off"
                        aria-invalid={scheduleError}
                      />
                      {scheduleError ? (
                        <p className="text-sm text-destructive">
                          {t("services.createFieldScheduleError")}
                        </p>
                      ) : (
                        <p className="text-sm text-muted-foreground">
                          {t("services.createFieldScheduleHint")}
                        </p>
                      )}
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="svc-command">
                        {t("services.createFieldStartCommand")}
                      </Label>
                      <Input
                        id="svc-command"
                        value={command}
                        onChange={(e) => setCommand(e.target.value)}
                        placeholder={t(
                          "services.createFieldCommandPlaceholder",
                        )}
                        autoComplete="off"
                      />
                      <p className="text-sm text-muted-foreground">
                        {t("services.createFieldCommandHint")}
                      </p>
                    </div>
                  </>
                ) : null}

                {isStaticBuild ? (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="svc-static-build-command">
                        {t("services.createFieldBuildCommand")}
                      </Label>
                      <Input
                        id="svc-static-build-command"
                        value={staticBuildCommand}
                        onChange={(e) => setStaticBuildCommand(e.target.value)}
                        placeholder={t("services.buildCommandPlaceholder")}
                        autoComplete="off"
                      />
                      <p className="text-sm text-muted-foreground">
                        {t("services.buildCommandHint")}
                      </p>
                    </div>
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
                        values={buildFilterPaths}
                        onChange={setBuildFilterPaths}
                      />
                      <PathList
                        title={t("services.buildFilterIgnoredTitle")}
                        hint={t("services.buildFilterIgnoredHint")}
                        placeholder={t(
                          "services.buildFilterIgnoredPlaceholder",
                        )}
                        addLabel={t("services.buildFilterAddIgnored")}
                        removeLabel={t("services.buildFilterRemoveIgnored")}
                        values={buildFilterIgnored}
                        onChange={setBuildFilterIgnored}
                      />
                    </div>
                  </>
                ) : null}

                {isStaticType ? (
                  <div className="space-y-2">
                    <Label htmlFor="svc-publish-path">
                      {t("services.createFieldPublishPath")}
                    </Label>
                    <Input
                      id="svc-publish-path"
                      value={publishPath}
                      onChange={(e) => setPublishPath(e.target.value)}
                      placeholder={t(
                        "services.createFieldPublishPathPlaceholder",
                      )}
                      autoComplete="off"
                    />
                    <p className="text-sm text-muted-foreground">
                      {t("services.createFieldPublishPathHint")}
                    </p>
                  </div>
                ) : null}

                {showNoUrlNote ? (
                  <p className="text-sm text-muted-foreground">
                    {t("services.createNoPublicUrlNote")}
                  </p>
                ) : null}

                {showPlan ? (
                  <div className="space-y-2">
                    <Label>{t("services.createFieldPlan")}</Label>
                    {instanceTypes.length === 0 ? (
                      <Skeleton className="h-20 w-full" />
                    ) : (
                      <PlanCardGrid
                        instanceTypes={instanceTypes}
                        value={plan}
                        onChange={setPlanOverride}
                      />
                    )}
                  </div>
                ) : null}

                <ProjectEnvironmentSelector
                  projectId={projectId}
                  environmentId={environmentId}
                  onProjectChange={setProjectId}
                  onEnvironmentChange={setEnvironmentId}
                />

                {isGitSource ? (
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
                      checked={autoDeploy}
                      onCheckedChange={setAutoDeploy}
                    />
                  </div>
                ) : null}

                <CreateEnvVarEditor rows={envVars} onChange={setEnvVars} />
                <CreateSecretFileEditor
                  rows={secretFiles}
                  onChange={setSecretFiles}
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
