import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Combobox } from "@/common/components/ui/combobox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
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
import { isValidGitUrl } from "@/common/lib/utils/git-url";
import { isValidDockerImageRef } from "@/common/lib/utils/docker-image";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  ServiceSourcePicker,
  type SourceTab,
} from "@/features/services/components/service-source-picker";
import {
  defaultStartCommand,
  useBuildSource,
  useBuildSources,
  type BuildSourceOption,
} from "@/features/services/hooks/use-build-sources";
import { useLinkBuildSource } from "@/features/services/hooks/use-link-build-source";
import { useRepoBranches } from "@/features/services/hooks/use-repo-branches";
import type { RepoView } from "@/features/services/hooks/use-repos";
import { useSetImage } from "@/features/services/hooks/use-set-image";
import { useSetRepo } from "@/features/services/hooks/use-set-repo";
import { formatRepoLabel } from "@/features/services/lib/repo";

export interface ServiceSourceCardProps {
  serviceId: string;
  repo: string | null;
  branch: string | null;
  imagePath: string | null;
  registryCredentialId: string | null;
  /** The build source this service builds from (w2/m254), else null. */
  buildSourceId: string | null;
  /** The service's start command, kept when it links a native source. */
  startCommand: string | null;
}

/**
 * Render-style Settings → Source card. The ready state has one stable card for
 * both source kinds; Edit opens the shared account-grouped picker and lets the
 * user repoint repo+branch atomically, switch to a prebuilt image, or build
 * from a build source (w2/m254). Updating source intent never deploys it — the
 * next explicit/push deploy consumes it.
 */
export function ServiceSourceCard(props: ServiceSourceCardProps) {
  const { t } = useTranslations();
  const { source: linked, loading: linkedLoading } = useBuildSource(
    props.buildSourceId,
  );
  const capabilities = useCapabilities();
  const { canCreate } = capabilities;
  const reasonKey = capabilities.reasonKey("can_create");
  const reason = reasonKey ? t(reasonKey) : undefined;

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-4">
        <div className="space-y-1.5">
          <CardTitle>{t("services.sourceTitle")}</CardTitle>
          <CardDescription>
            {t(
              props.buildSourceId
                ? "services.sourceBuildSourceDescription"
                : props.repo
                  ? "services.sourceRepoDescription"
                  : "services.sourceImageDescription",
            )}
          </CardDescription>
        </div>
        <SourceEditDialog {...props} disabled={!canCreate} />
      </CardHeader>
      <CardContent>
        <dl className="grid gap-4 text-sm sm:grid-cols-2">
          <div className="space-y-1">
            <dt className="text-muted-foreground">
              {t(
                props.buildSourceId
                  ? "services.sourceBuildSourceLabel"
                  : props.repo
                    ? "services.buildDeploySourceLabel"
                    : "services.sourceImageLabel",
              )}
            </dt>
            <dd className="break-all font-mono font-medium">
              {props.buildSourceId
                ? (linked?.name ??
                  (linkedLoading ? (
                    <Skeleton className="inline-block h-4 w-32 align-middle" />
                  ) : (
                    props.buildSourceId
                  )))
                : props.repo
                  ? formatRepoLabel(props.repo)
                  : props.imagePath || t("services.sourceMissing")}
            </dd>
          </div>
          {props.buildSourceId && linked?.source && (
            <div className="space-y-1">
              <dt className="text-muted-foreground">
                {t("services.sourceBuildSourceBuildsFrom")}
              </dt>
              <dd className="break-all font-mono font-medium">
                {sourceLabel(linked)}
              </dd>
            </div>
          )}
          {props.repo && (
            <div className="space-y-1">
              <dt className="text-muted-foreground">
                {t("services.buildDeployBranchLabel")}
              </dt>
              <dd className="break-all font-mono font-medium">
                {props.branch || "main"}
              </dd>
            </div>
          )}
        </dl>
        {reason && (
          <p className="mt-4 text-sm text-muted-foreground">{reason}</p>
        )}
      </CardContent>
    </Card>
  );
}

function SourceEditDialog({
  serviceId,
  repo,
  branch: currentBranch,
  imagePath,
  registryCredentialId,
  buildSourceId,
  startCommand: currentStartCommand,
  disabled,
}: ServiceSourceCardProps & { disabled: boolean }) {
  const { t } = useTranslations();
  const { setRepo, busy: repoBusy } = useSetRepo();
  const { linkBuildSource, busy: linkBusy } = useLinkBuildSource();
  const { setImage, busy: imageBusy, lastError: imageError } = useSetImage();
  // The value the server last refused; editing away from it clears the
  // under-field message (w4/406).
  const [refusedImage, setRefusedImage] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const initialTab: SourceTab = buildSourceId
    ? "build-source"
    : repo
      ? "git"
      : "image";
  const [tab, setTab] = useState<SourceTab>(initialTab);
  const [sourceId, setSourceId] = useState(buildSourceId ?? "");
  const [startCommand, setStartCommand] = useState(currentStartCommand ?? "");
  const {
    sources,
    loading: sourcesLoading,
    error: sourcesError,
  } = useBuildSources(open);
  const selectedSource = sources.find((source) => source.id === sourceId);
  const needsStart = selectedSource?.nativeRuntime != null;
  const [selectedRepo, setSelectedRepo] = useState<RepoView | null>(null);
  const [gitUrl, setGitUrl] = useState(repo ?? "");
  const [branch, setBranch] = useState(currentBranch ?? "main");
  const [image, setImageValue] = useState(imagePath ?? "");
  const [credentialId, setCredentialId] = useState(registryCredentialId ?? "");

  const sourceRepo = tab === "github" ? (selectedRepo?.htmlUrl ?? "") : gitUrl;
  const { branches } = useRepoBranches(
    open && tab !== "image" ? sourceRepo || null : null,
  );
  const branchOptions = useMemo(
    () => branches.map((value) => ({ value, label: value })),
    [branches],
  );
  const valid =
    tab === "build-source"
      ? selectedSource !== undefined &&
        (!needsStart || startCommand.trim() !== "")
      : tab === "image"
        ? isValidDockerImageRef(image)
        : tab === "github"
          ? selectedRepo !== null
          : isValidGitUrl(gitUrl);
  // An image-backed service's repo arrives as "" (bex-api), not null, so
  // source kind is repo presence — as the tab and the card already decide it
  // (w4/275).
  const dirty =
    tab === "build-source"
      ? sourceId !== (buildSourceId ?? "") ||
        (needsStart &&
          startCommand.trim() !== (currentStartCommand ?? "").trim())
      : tab === "image"
        ? Boolean(repo) ||
          image.trim() !== (imagePath ?? "").trim() ||
          (credentialId.trim() || null) !==
            (registryCredentialId?.trim() || null)
        : !repo ||
          sourceRepo.trim() !== repo.trim() ||
          (branch.trim() || "main") !== (currentBranch?.trim() || "main");
  const busy = repoBusy || imageBusy || linkBusy;

  function resetDraft() {
    setTab(initialTab);
    setSourceId(buildSourceId ?? "");
    setStartCommand(currentStartCommand ?? "");
    setSelectedRepo(null);
    setGitUrl(repo ?? "");
    setBranch(currentBranch || "main");
    setImageValue(imagePath ?? "");
    setCredentialId(registryCredentialId ?? "");
  }

  function changeOpen(next: boolean) {
    if (next) resetDraft();
    setOpen(next);
  }

  function selectRepo(next: RepoView) {
    setSelectedRepo(next);
    setBranch(next.defaultBranch || "main");
  }

  function selectSource(id: string) {
    setSourceId(id);
    const next = sources.find((source) => source.id === id);
    if (next && !startCommand.trim())
      setStartCommand(defaultStartCommand(next));
  }

  function changeTab(next: SourceTab) {
    setTab(next);
    if (next === "github" && selectedRepo && !branch) {
      setBranch(selectedRepo.defaultBranch || "main");
    }
  }

  async function save() {
    const ok =
      tab === "build-source"
        ? await linkBuildSource(
            serviceId,
            sourceId,
            needsStart ? startCommand.trim() : undefined,
          )
        : tab === "image"
          ? await setImage(serviceId, image.trim(), credentialId || undefined)
          : await setRepo(serviceId, {
              repo: sourceRepo.trim(),
              branch: branch.trim() || "main",
            });
    if (!ok) {
      if (tab === "image") setRefusedImage(image.trim());
      return;
    }
    setRefusedImage(null);
    setOpen(false);
  }

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <Button
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => changeOpen(true)}
      >
        {t("services.sourceEdit")}
      </Button>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("services.sourceDialogTitle")}</DialogTitle>
          <DialogDescription>
            {t("services.sourceNoAutoDeploy")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <ServiceSourcePicker
            tab={tab}
            onTabChange={changeTab}
            selectedRepo={selectedRepo}
            onSelectRepo={selectRepo}
            gitUrl={gitUrl}
            onGitUrlChange={setGitUrl}
            image={{
              value: image,
              onChange: setImageValue,
              registryCredentialId: credentialId,
              onRegistryCredentialChange: setCredentialId,
              showPortHint: false,
              serverError:
                refusedImage !== null && refusedImage === image.trim()
                  ? imageError
                  : null,
            }}
            buildSource={
              <BuildSourceTab
                sources={sources}
                loading={sourcesLoading}
                failed={sourcesError !== undefined}
                value={sourceId}
                onValueChange={selectSource}
                startCommand={needsStart ? startCommand : null}
                onStartCommandChange={setStartCommand}
              />
            }
            titleKey="services.sourcePickerLabel"
            idPrefix="update-source"
          />
          {(tab === "github" || tab === "git") && valid && (
            <div className="space-y-2">
              <Label>{t("services.buildDeployBranchLabel")}</Label>
              <Combobox
                value={branch}
                onValueChange={setBranch}
                options={branchOptions}
                allowCustom
                placeholder={t("services.buildDeployBranchPlaceholder")}
                emptyText={t("services.sourceBranchEmpty")}
                ariaLabel={t("services.buildDeployBranchLabel")}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">
                {t("services.sourceBranchHint")}
              </p>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => setOpen(false)}>
            {t("services.buildDeployCancel")}
          </Button>
          <Button
            disabled={!valid || !dirty || busy}
            onClick={() => void save()}
          >
            {busy && <Loader2 className="animate-spin" />}
            {t("services.sourceUpdateSave")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The Update Source dialog's build source tab (w2/m254): pick one of the
 * workspace's build sources, and give the service a start command when the
 * source builds with a native runtime (Render requires one; a Docker or image
 * source takes none).
 */
function BuildSourceTab({
  sources,
  loading,
  failed,
  value,
  onValueChange,
  startCommand,
  onStartCommandChange,
}: {
  sources: BuildSourceOption[];
  loading: boolean;
  /** The list read failed: say so rather than claim there are none. */
  failed: boolean;
  value: string;
  onValueChange: (id: string) => void;
  /** null when the selected source takes no start command. */
  startCommand: string | null;
  onStartCommandChange: (command: string) => void;
}) {
  const { t } = useTranslations();
  if (loading && sources.length === 0) {
    return (
      <div className="space-y-2" aria-busy="true">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-9 w-full rounded-md" />
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
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="update-source-build-source">
          {t("services.sourceBuildSourceLabel")}
        </Label>
        <Select value={value} onValueChange={onValueChange}>
          <SelectTrigger id="update-source-build-source">
            <SelectValue
              placeholder={t("services.sourceBuildSourcePlaceholder")}
            />
          </SelectTrigger>
          <SelectContent>
            {sources.map((source) => (
              <SelectItem key={source.id} value={source.id}>
                {source.name}
                {source.source && (
                  <span className="ml-2 font-mono text-xs text-muted-foreground">
                    {sourceLabel(source)}
                  </span>
                )}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {startCommand !== null && (
        <div className="space-y-2">
          <Label htmlFor="update-source-start-command">
            {t("services.startCommandLabel")}
          </Label>
          <Input
            id="update-source-start-command"
            value={startCommand}
            onChange={(e) => onStartCommandChange(e.target.value)}
            className="font-mono"
            autoComplete="off"
          />
          <p className="text-xs text-muted-foreground">
            {t("services.sourceBuildSourceStartHint")}
          </p>
        </div>
      )}
    </div>
  );
}

/** What a build source builds from: a repo label, or the image as is. */
function sourceLabel(source: BuildSourceOption): string {
  return source.kind === "git" ? formatRepoLabel(source.source) : source.source;
}
