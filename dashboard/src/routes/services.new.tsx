import { useEffect, useMemo, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  Github,
  GitBranch,
  Box,
  Loader2,
  Globe,
  Lock,
  Cpu,
  Clock,
  Layers,
  Plus,
  Trash2,
  ArrowUpRight,
  Sparkles,
} from "lucide-react";
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
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Textarea } from "@/common/components/ui/textarea";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
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
import { useServiceNameAvailability } from "@/features/services/hooks/use-service-name-availability";
import type {
  EnvVarEntry,
  SecretFileEntry,
} from "@/features/services/hooks/use-create-service";
import { useRepos } from "@/features/services/hooks/use-repos";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { isValidCron } from "@/features/services/lib/cron";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";
import { generateEnvValue } from "@/features/services/lib/generate-env-value";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useEnvironments } from "@/features/environments/hooks/use-environments";

// A C-locale env-var name — kept in sync with backend/internal/secrets validEnvKey.
const VALID_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/;
const VALID_SECRET_FILE = /^[-._A-Za-z0-9]+$/;

function isValidSecretFileName(name: string): boolean {
  return name !== "." && name !== ".." && VALID_SECRET_FILE.test(name);
}

type SourceTab = "github" | "git" | "image";
type NativeRuntime = "elixir" | "go" | "node" | "python" | "ruby" | "rust";
type GitRuntime = NativeRuntime | "docker";
type ServiceType =
  | "web_service"
  | "private_service"
  | "background_worker"
  | "cron_job"
  | "static_site";

const RUNTIME_COMMANDS: Record<
  NativeRuntime,
  { build: string; start: string }
> = {
  elixir: {
    build: "mix deps.get --only prod && mix compile",
    start: "mix phx.server",
  },
  go: { build: "go build -o app .", start: "./app" },
  node: { build: "npm install", start: "npm start" },
  python: {
    build: "pip install -r requirements.txt",
    start: "gunicorn app:app",
  },
  ruby: { build: "bundle install", start: "bundle exec puma" },
  rust: { build: "cargo build --release", start: "cargo run --release" },
};

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
              {formatInstanceMemory(it.memory)} RAM ·{" "}
              {formatInstanceCPU(it.cpu)}
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
            <span className="mt-0.5 shrink-0 text-muted-foreground">
              {icon}
            </span>
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

/** Inline key-value editor for create-time env vars (Render parity, w5/m19). */
function EnvVarEditor({
  rows,
  onChange,
}: {
  rows: EnvVarEntry[];
  onChange: (rows: EnvVarEntry[]) => void;
}) {
  const { t } = useTranslations();

  function addRow() {
    onChange([...rows, { key: "", value: "" }]);
  }

  function removeRow(i: number) {
    onChange(rows.filter((_, idx) => idx !== i));
  }

  function updateRow(i: number, field: keyof EnvVarEntry, val: string) {
    onChange(rows.map((r, idx) => (idx === i ? { ...r, [field]: val } : r)));
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{t("services.createFieldEnvVarsTitle")}</Label>
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="size-3.5" />
          {t("services.createFieldEnvVarsAdd")}
        </Button>
      </div>
      {rows.length > 0 && (
        <div className="space-y-1.5">
          <div className="grid grid-cols-[1fr_1fr_auto_auto] gap-2 px-0.5">
            <span className="text-xs text-muted-foreground">
              {t("services.createFieldEnvVarsKey")}
            </span>
            <span className="text-xs text-muted-foreground">
              {t("services.createFieldEnvVarsValue")}
            </span>
            <span />
            <span />
          </div>
          {rows.map((row, i) => {
            const keyInvalid = row.key !== "" && !VALID_KEY.test(row.key);
            return (
              <div key={i} className="space-y-1">
                <div className="grid grid-cols-[1fr_1fr_auto_auto] gap-2">
                  <Input
                    value={row.key}
                    onChange={(e) => updateRow(i, "key", e.target.value)}
                    placeholder={t("services.createFieldEnvVarsKeyPlaceholder")}
                    aria-label={t("services.createFieldEnvVarsKey")}
                    aria-invalid={keyInvalid}
                    className="font-mono text-sm"
                  />
                  <Input
                    value={row.value}
                    onChange={(e) => updateRow(i, "value", e.target.value)}
                    placeholder={t(
                      "services.createFieldEnvVarsValuePlaceholder",
                    )}
                    aria-label={t("services.createFieldEnvVarsValue")}
                    className="font-mono text-sm"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => updateRow(i, "value", generateEnvValue())}
                  >
                    <Sparkles className="size-3.5" />
                    {t("services.envGenerate")}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => removeRow(i)}
                    aria-label={t("services.createFieldEnvVarsRemove")}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
                {keyInvalid && (
                  <p className="text-xs text-destructive">
                    {t("services.createFieldEnvVarsKeyError")}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

/** Create-time files mounted read-only at /etc/secrets from first boot. */
function SecretFileEditor({
  rows,
  onChange,
}: {
  rows: SecretFileEntry[];
  onChange: (rows: SecretFileEntry[]) => void;
}) {
  const { t } = useTranslations();

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div>
          <Label>{t("services.createFieldSecretFilesTitle")}</Label>
          <p className="text-sm text-muted-foreground">
            {t("services.createFieldSecretFilesHint")}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onChange([...rows, { name: "", content: "" }])}
        >
          <Plus className="size-3.5" />
          {t("services.createFieldSecretFilesAdd")}
        </Button>
      </div>
      {rows.map((row, i) => {
        const invalid = row.name !== "" && !isValidSecretFileName(row.name);
        return (
          <div key={i} className="space-y-1 rounded-md border p-3">
            <div className="flex items-start gap-2">
              <div className="flex-1 space-y-2">
                <Input
                  value={row.name}
                  onChange={(event) =>
                    onChange(
                      rows.map((item, index) =>
                        index === i
                          ? { ...item, name: event.target.value }
                          : item,
                      ),
                    )
                  }
                  placeholder={t(
                    "services.createFieldSecretFilesNamePlaceholder",
                  )}
                  aria-label={t("services.createFieldSecretFilesName")}
                  aria-invalid={invalid}
                  className="font-mono text-sm"
                />
                <Textarea
                  value={row.content}
                  onChange={(event) =>
                    onChange(
                      rows.map((item, index) =>
                        index === i
                          ? { ...item, content: event.target.value }
                          : item,
                      ),
                    )
                  }
                  placeholder={t(
                    "services.createFieldSecretFilesContentPlaceholder",
                  )}
                  aria-label={t("services.createFieldSecretFilesContent")}
                  className="min-h-20 font-mono text-sm"
                />
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={() => onChange(rows.filter((_, index) => index !== i))}
                aria-label={t("services.createFieldSecretFilesRemove")}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
            {invalid ? (
              <p className="text-xs text-destructive">
                {t("services.createFieldSecretFilesNameError")}
              </p>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

export function NewServicePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { instanceTypes } = useInstanceTypes();
  const { create, busy, capLimit, nameConflict, clearNameConflict } =
    useCreateService();
  const { repos, loading: reposLoading } = useRepos();
  const { connection, loading: connectionLoading } = useGitConnection();
  const { projects } = useProjects();

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
  const [runtime, setRuntime] = useState<GitRuntime>("node");
  const [buildCommand, setBuildCommand] = useState(RUNTIME_COMMANDS.node.build);
  const [startCommand, setStartCommand] = useState(RUNTIME_COMMANDS.node.start);
  const [dockerfilePath, setDockerfilePath] = useState("");
  const [planOverride, setPlanOverride] = useState<string | null>(null);
  const [autoDeploy, setAutoDeploy] = useState(true);
  const [schedule, setSchedule] = useState("");
  const [command, setCommand] = useState("");
  const [publishPath, setPublishPath] = useState("");
  const [envVars, setEnvVars] = useState<EnvVarEntry[]>([]);
  const [secretFiles, setSecretFiles] = useState<SecretFileEntry[]>([]);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [environmentId, setEnvironmentId] = useState<string | null>(null);
  const { environments } = useEnvironments(projectId);

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
      // Intentional sync of the name field to the selected source while the
      // user hasn't hand-edited it (guarded by nameEdited); the effect reacts
      // to selection changes, so a synchronous set here is the desired behavior.
      // eslint-disable-next-line react-hooks/set-state-in-effect
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
  const {
    taken: nameTaken,
    suggestion: nameSuggestion,
    checking: checkingName,
  } = useServiceNameAvailability(nameValid ? name : "");

  // Same repo connected twice (or any other auto-filled slug that's already
  // taken): prefill the suggested free name instead of leaving the user to
  // notice and fix it by hand — the auto-fill effect above just set `name` to
  // a taken slug, this swaps it for the free alternative once the check
  // settles. Never fires once the user has actually typed (nameEdited).
  // Adjusted during render — React's own "store information from previous
  // renders" pattern (state, not a ref: refs can't be read/written during
  // render either) — rather than a useEffect, so it swaps at most once per
  // taken name instead of looping.
  const [lastAutoSwappedName, setLastAutoSwappedName] = useState<string | null>(
    null,
  );
  if (
    !nameEdited &&
    nameTaken &&
    nameSuggestion &&
    name !== nameSuggestion &&
    lastAutoSwappedName !== name
  ) {
    setLastAutoSwappedName(name);
    setName(nameSuggestion);
  }

  const showNameError = name.length > 0 && !nameValid;
  const showNameTaken = nameValid && (nameTaken || nameConflict);

  const sourceValid =
    (tab === "github" && selectedRepo != null) ||
    (tab === "git" && isValidGitUrl(gitUrl)) ||
    (tab === "image" && imageVal.trim().length > 0);

  const envVarsValid = envVars.every(
    (r) => r.key === "" || VALID_KEY.test(r.key),
  );
  const secretFilesValid = secretFiles.every(
    (file) => file.name === "" || isValidSecretFileName(file.name),
  );
  const nativeCommandsValid =
    !isGitSource ||
    isStaticType ||
    runtime === "docker" ||
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

    const validEnvVars = envVars.filter(
      (r) => r.key.trim() !== "" && VALID_KEY.test(r.key.trim()),
    );
    const validSecretFiles = secretFiles.filter((file) =>
      isValidSecretFileName(file.name.trim()),
    );
    const id = await create({
      name,
      type: serviceType,
      environmentId: environmentId || undefined,
      repo,
      image,
      branch: branchVal,
      rootDir: rootDir || undefined,
      runtime:
        tab === "image"
          ? "image"
          : isGitSource && !isStaticType
            ? runtime
            : undefined,
      buildCommand:
        isGitSource && !isStaticType && runtime !== "docker"
          ? buildCommand.trim()
          : undefined,
      startCommand:
        isGitSource && !isStaticType
          ? isCronType
            ? command.trim()
            : startCommand.trim() || undefined
          : undefined,
      dockerfilePath:
        isGitSource && !isStaticType && runtime === "docker"
          ? dockerfilePath.trim() || undefined
          : undefined,
      plan: showPlan ? plan || undefined : undefined,
      autoDeploy: isGitSource ? autoDeploy : undefined,
      schedule: isCronType ? schedule.trim() || undefined : undefined,
      command: isCronType ? command.trim() || undefined : undefined,
      publishPath: isStaticType ? publishPath.trim() || undefined : undefined,
      envVars: validEnvVars.length ? validEnvVars : undefined,
      secretFiles: validSecretFiles.length ? validSecretFiles : undefined,
    });
    if (id) {
      void navigate({ to: "/services/$serviceId", params: { serviceId: id } });
    }
  }

  const gitHubDisconnected =
    !connectionLoading && connection?.connected !== true;

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("services.createTitle")}</CardTitle>
              <CardDescription>
                {t("services.createDescription")}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* Service type picker */}
              <div className="space-y-3">
                <Label>{t("services.createTypePickerTitle")}</Label>
                <ServiceTypePicker
                  value={serviceType}
                  onChange={setServiceType}
                />
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
                              <div
                                key={i}
                                className="flex items-center gap-3 p-3"
                              >
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
                          onClick={() => {
                            setName(nameSuggestion);
                            setNameEdited(true);
                          }}
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
                    {!isStaticType ? (
                      <>
                        <div className="space-y-2">
                          <Label htmlFor="svc-runtime">
                            {t("services.createFieldRuntime")}
                          </Label>
                          <Select
                            value={runtime}
                            onValueChange={(value) => {
                              const next = value as GitRuntime;
                              setRuntime(next);
                              if (next === "docker") {
                                setBuildCommand("");
                                setStartCommand("");
                              } else {
                                setDockerfilePath("");
                                setBuildCommand(RUNTIME_COMMANDS[next].build);
                                setStartCommand(RUNTIME_COMMANDS[next].start);
                              }
                            }}
                          >
                            <SelectTrigger id="svc-runtime" className="w-full">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="node">
                                {t("services.createRuntimeNode")}
                              </SelectItem>
                              <SelectItem value="python">
                                {t("services.createRuntimePython")}
                              </SelectItem>
                              <SelectItem value="go">
                                {t("services.createRuntimeGo")}
                              </SelectItem>
                              <SelectItem value="ruby">
                                {t("services.createRuntimeRuby")}
                              </SelectItem>
                              <SelectItem value="rust">
                                {t("services.createRuntimeRust")}
                              </SelectItem>
                              <SelectItem value="elixir">
                                {t("services.createRuntimeElixir")}
                              </SelectItem>
                              <SelectItem value="docker">
                                {t("services.createRuntimeDocker")}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        {runtime !== "docker" ? (
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

                <div className="space-y-2">
                  <Label>{t("services.createFieldEnvironmentTitle")}</Label>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    <Select
                      value={projectId ?? "unassigned"}
                      onValueChange={(value) => {
                        setProjectId(value === "unassigned" ? null : value);
                        setEnvironmentId(null);
                      }}
                    >
                      <SelectTrigger
                        aria-label={t("services.createFieldProject")}
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="unassigned">
                          {t("services.createFieldProjectNone")}
                        </SelectItem>
                        {projects.map((project) => (
                          <SelectItem key={project.id} value={project.id}>
                            {project.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Select
                      disabled={projectId == null}
                      value={environmentId ?? "unassigned"}
                      onValueChange={(value) =>
                        setEnvironmentId(value === "unassigned" ? null : value)
                      }
                    >
                      <SelectTrigger
                        aria-label={t("services.createFieldEnvironment")}
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="unassigned">
                          {t("services.createFieldEnvironmentNone")}
                        </SelectItem>
                        {environments.map((environment) => (
                          <SelectItem
                            key={environment.id}
                            value={environment.id}
                          >
                            {environment.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {t("services.createFieldEnvironmentHint")}
                  </p>
                </div>

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

                <EnvVarEditor rows={envVars} onChange={setEnvVars} />
                <SecretFileEditor
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
