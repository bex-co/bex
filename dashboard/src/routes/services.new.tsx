import { useEffect, useMemo, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Github, GitBranch, Box, Loader2, Globe, Lock, Cpu, Clock, Layers } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { isValidDnsLabel } from "@/common/lib/utils/dns-label";
import { repoNameSlug, gitUrlSlug, imageSlug } from "@/common/lib/utils/slug";
import { cn } from "@/common/lib/utils/utils";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";
import { useCreateService } from "@/features/services/hooks/use-create-service";
import { useRepos } from "@/features/services/hooks/use-repos";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { isValidCron } from "@/features/services/lib/cron";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";

type SourceTab = "github" | "git" | "image";
type ServiceType =
  | "web_service"
  | "private_service"
  | "background_worker"
  | "cron_job"
  | "static_site";

export const Route = createFileRoute("/services/new")({
  component: NewServicePage,
  beforeLoad: requireAuth("/services/new"),
  head: () => ({
    meta: [{ title: "New Service · bex dashboard" }],
  }),
});

function isValidGitUrl(url: string): boolean {
  return /^(https?:\/\/|git@|git:\/\/)/.test(url.trim());
}


/**
 * The service-creation plan picker for the create wizard — a radio-group of
 * tier cards. Identical visual pattern to KeyValuePlanPicker but typed for
 * InstanceTypeView (service plans, not KV plans).
 */
function ServicePlanPicker({
  instanceTypes,
  value,
  onChange,
}: {
  instanceTypes: InstanceTypeView[];
  value: string;
  onChange: (id: string) => void;
}) {
  if (instanceTypes.length === 0) {
    return <Skeleton className="h-20 w-full" />;
  }
  return (
    <div role="radiogroup" className="grid grid-cols-1 gap-3 sm:grid-cols-3">
      {instanceTypes.map((it) => {
        const selected = it.id === value;
        return (
          <button
            key={it.id}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(it.id)}
            className={cn(
              "rounded-lg border p-3 text-left transition-colors",
              selected
                ? "border-primary ring-1 ring-primary"
                : "border-border hover:border-muted-foreground/50",
            )}
          >
            <div className="font-medium">{it.name}</div>
            <div className="text-sm text-muted-foreground">
              {formatInstanceMemory(it.memory)} RAM · {formatInstanceCPU(it.cpu)}
            </div>
          </button>
        );
      })}
    </div>
  );
}

const SERVICE_TYPE_DEFS: {
  type: ServiceType;
  icon: React.ReactNode;
  labelKey: string;
  descKey: string;
}[] = [
  {
    type: "web_service",
    icon: <Globe className="size-4" />,
    labelKey: "services.typeWeb",
    descKey: "services.createTypeWebDesc",
  },
  {
    type: "private_service",
    icon: <Lock className="size-4" />,
    labelKey: "services.typePrivate",
    descKey: "services.createTypePrivateDesc",
  },
  {
    type: "background_worker",
    icon: <Cpu className="size-4" />,
    labelKey: "services.typeWorker",
    descKey: "services.createTypeWorkerDesc",
  },
  {
    type: "cron_job",
    icon: <Clock className="size-4" />,
    labelKey: "services.typeCron",
    descKey: "services.createTypeCronDesc",
  },
  {
    type: "static_site",
    icon: <Layers className="size-4" />,
    labelKey: "services.typeStatic",
    descKey: "services.createTypeStaticDesc",
  },
];

function ServiceTypePicker({
  value,
  onChange,
}: {
  value: ServiceType;
  onChange: (type: ServiceType) => void;
}) {
  const { t } = useTranslations();
  return (
    <div role="radiogroup" className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {SERVICE_TYPE_DEFS.map(({ type, icon, labelKey, descKey }) => {
        const selected = type === value;
        return (
          <button
            key={type}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(type)}
            className={cn(
              "flex items-start gap-3 rounded-lg border p-3 text-left transition-colors",
              selected
                ? "border-primary ring-1 ring-primary"
                : "border-border hover:border-muted-foreground/50",
            )}
          >
            <span className="mt-0.5 shrink-0 text-muted-foreground">{icon}</span>
            <div>
              <div className="text-sm font-medium">{t(labelKey)}</div>
              <div className="text-xs text-muted-foreground">{t(descKey)}</div>
            </div>
          </button>
        );
      })}
    </div>
  );
}

export function NewServicePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { instanceTypes } = useInstanceTypes();
  const { create, busy } = useCreateService();
  const { repos, loading: reposLoading } = useRepos();
  const { connection, loading: connectionLoading } = useGitConnection();

  const [serviceType, setServiceType] = useState<ServiceType>("web_service");
  const [tab, setTab] = useState<SourceTab>("github");
  const [selectedRepo, setSelectedRepo] = useState<RepoView | null>(null);
  const [repoSearch, setRepoSearch] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [imageVal, setImageVal] = useState("");
  const [name, setName] = useState("");
  const [nameEdited, setNameEdited] = useState(false);
  const [branch, setBranch] = useState("");
  const [rootDir, setRootDir] = useState("");
  const [planOverride, setPlanOverride] = useState<string | null>(null);
  const [autoDeploy, setAutoDeploy] = useState(true);
  const [schedule, setSchedule] = useState("");
  const [command, setCommand] = useState("");
  const [publishPath, setPublishPath] = useState("");

  const isCronType = serviceType === "cron_job";
  const isStaticType = serviceType === "static_site";
  const showNoUrlNote =
    serviceType === "private_service" || serviceType === "background_worker";
  const showPlan = !isStaticType;

  const plan = planOverride ?? instanceTypes[0]?.id ?? "";
  const isGitSource = tab === "github" || tab === "git";

  const scheduleError =
    isCronType && schedule.trim() !== "" && !isValidCron(schedule);

  // Auto-fill name + branch from source when user hasn't manually typed a name.
  // `nameEdited` resets to false when the name is cleared (onChange uses
  // `value !== ""`), so selecting a new source after clearing re-enables fill.
  useEffect(() => {
    if (nameEdited) return;
    if (tab === "github" && selectedRepo) {
      setName(repoNameSlug(selectedRepo.fullName));
      setBranch((b) => b || selectedRepo.defaultBranch);
    } else if (tab === "git" && gitUrl) {
      const slug = gitUrlSlug(gitUrl);
      if (slug) setName(slug);
    } else if (tab === "image" && imageVal) {
      const slug = imageSlug(imageVal);
      if (slug) setName(slug);
    }
  }, [tab, selectedRepo, gitUrl, imageVal, nameEdited]);

  const nameValid = isValidDnsLabel(name);
  const showNameError = name.length > 0 && !nameValid;

  const sourceValid =
    (tab === "github" && selectedRepo != null) ||
    (tab === "git" && isValidGitUrl(gitUrl)) ||
    (tab === "image" && imageVal.trim().length > 0);

  const canSubmit =
    nameValid &&
    sourceValid &&
    !busy &&
    (isStaticType || plan !== "") &&
    (!isCronType || (schedule.trim() !== "" && !scheduleError));

  const filteredRepos = useMemo(
    () =>
      repos.filter(
        (r) =>
          !repoSearch ||
          r.fullName.toLowerCase().includes(repoSearch.toLowerCase()),
      ),
    [repos, repoSearch],
  );

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

    const id = await create({
      name,
      type: serviceType,
      repo,
      image,
      branch: branchVal,
      rootDir: rootDir || undefined,
      plan: showPlan ? plan || undefined : undefined,
      autoDeploy: isGitSource ? autoDeploy : undefined,
      schedule: isCronType ? schedule.trim() || undefined : undefined,
      command: isCronType ? command.trim() || undefined : undefined,
      publishPath: isStaticType ? publishPath.trim() || undefined : undefined,
    });
    if (id) {
      void navigate({ to: "/services/$serviceId", params: { serviceId: id } });
    }
  }

  const gitHubDisconnected = !connectionLoading && connection?.connected !== true;

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("services.createTitle")}</CardTitle>
              <CardDescription>{t("services.createDescription")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* Service type picker */}
              <div className="space-y-3">
                <Label>{t("services.createTypePickerTitle")}</Label>
                <ServiceTypePicker value={serviceType} onChange={setServiceType} />
              </div>

              {/* Source picker */}
              <div className="space-y-3">
                <Label>{t("services.createSourceTitle")}</Label>
                <Tabs
                  value={tab}
                  onValueChange={(v) => {
                    setTab(v as SourceTab);
                    setSelectedRepo(null);
                    setBranch("");
                  }}
                >
                  <TabsList className="w-full">
                    <TabsTrigger value="github" className="flex-1 gap-1.5">
                      <Github className="size-4" />
                      {t("services.createTabGitHub")}
                    </TabsTrigger>
                    <TabsTrigger value="git" className="flex-1 gap-1.5">
                      <GitBranch className="size-4" />
                      {t("services.createTabPublicGit")}
                    </TabsTrigger>
                    <TabsTrigger value="image" className="flex-1 gap-1.5">
                      <Box className="size-4" />
                      {t("services.createTabImage")}
                    </TabsTrigger>
                  </TabsList>

                  {/* GitHub tab */}
                  <TabsContent value="github" className="mt-3">
                    {connectionLoading ? (
                      <Skeleton className="h-24 w-full" />
                    ) : gitHubDisconnected ? (
                      <div className="flex flex-col items-center gap-3 rounded-lg border p-6 text-center">
                        <Github className="size-8 text-muted-foreground" />
                        <div>
                          <p className="font-medium">
                            {t("services.createGitConnectPromptTitle")}
                          </p>
                          <p className="mt-1 text-sm text-muted-foreground">
                            {t("services.createGitConnectPromptBody")}
                          </p>
                        </div>
                        <Button asChild>
                          <a
                            href={connection?.installUrl ?? ""}
                            target="_blank"
                            rel="noreferrer"
                          >
                            {t("services.createGitConnectButton")}
                          </a>
                        </Button>
                      </div>
                    ) : (
                      <div className="space-y-2">
                        <Input
                          placeholder={t(
                            "services.createRepoSearchPlaceholder",
                          )}
                          value={repoSearch}
                          onChange={(e) => setRepoSearch(e.target.value)}
                          aria-label={t("services.createRepoSearchPlaceholder")}
                        />
                        <div className="max-h-64 overflow-y-auto rounded-md border divide-y">
                          {reposLoading ? (
                            Array.from({ length: 3 }).map((_, i) => (
                              <div key={i} className="flex items-center gap-3 p-3">
                                <Skeleton className="h-4 w-full" />
                              </div>
                            ))
                          ) : filteredRepos.length === 0 ? (
                            <div className="p-6 text-center text-sm text-muted-foreground">
                              {repoSearch
                                ? t("services.createRepoNoMatch")
                                : t("services.createRepoEmpty")}
                            </div>
                          ) : (
                            filteredRepos.map((r) => (
                              <button
                                key={r.id}
                                type="button"
                                onClick={() => setSelectedRepo(r)}
                                className={cn(
                                  "flex w-full items-center justify-between p-3 text-left transition-colors hover:bg-muted",
                                  selectedRepo?.id === r.id &&
                                    "bg-primary/5 hover:bg-primary/10",
                                )}
                              >
                                <div className="flex min-w-0 items-center gap-2">
                                  <Github className="size-4 shrink-0 text-muted-foreground" />
                                  <span className="truncate text-sm font-medium">
                                    {r.fullName}
                                  </span>
                                  {r.private && (
                                    <Badge
                                      variant="secondary"
                                      className="shrink-0 text-xs"
                                    >
                                      {t("services.createRepoPrivateBadge")}
                                    </Badge>
                                  )}
                                </div>
                                <span className="ml-3 shrink-0 text-xs text-muted-foreground">
                                  {r.defaultBranch}
                                </span>
                              </button>
                            ))
                          )}
                        </div>
                      </div>
                    )}
                  </TabsContent>

                  {/* Public Git URL tab */}
                  <TabsContent value="git" className="mt-3">
                    <div className="space-y-2">
                      <Label htmlFor="svc-git-url">
                        {t("services.createPublicUrlLabel")}
                      </Label>
                      <Input
                        id="svc-git-url"
                        value={gitUrl}
                        onChange={(e) => setGitUrl(e.target.value)}
                        placeholder={t("services.createPublicUrlPlaceholder")}
                        autoComplete="off"
                      />
                      {gitUrl && !isValidGitUrl(gitUrl) ? (
                        <p className="text-sm text-destructive">
                          {t("services.createPublicUrlError")}
                        </p>
                      ) : null}
                    </div>
                  </TabsContent>

                  {/* Existing Image tab */}
                  <TabsContent value="image" className="mt-3">
                    <div className="space-y-2">
                      <Label htmlFor="svc-image">
                        {t("services.createImageLabel")}
                      </Label>
                      <Input
                        id="svc-image"
                        value={imageVal}
                        onChange={(e) => setImageVal(e.target.value)}
                        placeholder={t("services.createImagePlaceholder")}
                        autoComplete="off"
                      />
                    </div>
                  </TabsContent>
                </Tabs>
              </div>

              {/* Settings */}
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
                      setName(e.target.value);
                      setNameEdited(e.target.value !== "");
                    }}
                    placeholder={t("services.createFieldNamePlaceholder")}
                    autoComplete="off"
                    aria-invalid={showNameError}
                  />
                  {showNameError ? (
                    <p className="text-sm text-destructive">
                      {t("services.createFieldNameError")}
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
                  </>
                ) : null}

                {/* Cron-specific fields */}
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
                        {t("services.createFieldCommand")}
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

                {/* Static site publish directory */}
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

                {/* No public URL note for private / worker types */}
                {showNoUrlNote ? (
                  <p className="text-sm text-muted-foreground">
                    {t("services.createNoPublicUrlNote")}
                  </p>
                ) : null}

                {showPlan ? (
                  <div className="space-y-2">
                    <Label>{t("services.createFieldPlan")}</Label>
                    <ServicePlanPicker
                      instanceTypes={instanceTypes}
                      value={plan}
                      onChange={setPlanOverride}
                    />
                  </div>
                ) : null}

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
              </div>

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
