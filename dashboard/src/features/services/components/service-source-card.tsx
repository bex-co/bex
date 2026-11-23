import { useEffect, useMemo, useRef, useState } from "react";
import { Loader2, TriangleAlert } from "lucide-react";
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
  CardAction,
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
import {
  CREATE_INPUT_CLASS,
  CREATE_SELECT_CLASS,
} from "@/common/components/create-form";
import { useTranslations } from "@/common/hooks/use-translations";
import { isValidGitUrl, parseGitHubPage } from "@/common/lib/utils/git-url";
import { cn } from "@/common/lib/utils/utils";
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
import { commandText } from "@/features/services/lib/command-text";
import {
  formatRepoLabel,
  repoSlug,
  sameRepo,
} from "@/features/services/lib/repo";

// 44 px on a phone, where the two share the row; a ring that reads on the
// dialog's surface (the default is the ring colour at half strength).
const FOOTER_BUTTON_CLASS = cn(
  CREATE_INPUT_CLASS,
  "focus-visible:ring-ring max-sm:flex-1",
);
// Disabled, the primary action turns neutral: the default half-strength green
// still read as a button to press, most of all on the dark surface.
const FOOTER_PRIMARY_CLASS = cn(
  FOOTER_BUTTON_CLASS,
  "disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100",
);

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
  /**
   * The service's builder and build command. An image service keeps a native
   * builder for a switch back to Git (w4/m202) but not the commands its build
   * baked in (w8/m461), so that switch asks for both (w32/052).
   */
  builder?: string | null;
  buildCommand?: string | null;
  /**
   * bex's GitHub app connected the repository and can no longer read it, so
   * no push reaches bex (w10/m490): the card says so and what to do.
   */
  repoAccessLost?: boolean;
  /**
   * How a push to the repository reaches bex. `github_app` means the
   * workspace's GitHub connection grants the repository, so Update Source
   * opens on the GitHub tab with it selected; any other repository is a URL,
   * on the Public Git URL tab.
   */
  pushDeliveryMethod?: string | null;
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
      <CardHeader>
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
        {/* The card's action, top right like every other card's: as a bare
            grid item the button stretched across the card above the
            values it edits. */}
        <CardAction>
          <SourceEditDialog {...props} disabled={!canCreate} />
        </CardAction>
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
        {props.repo && props.repoAccessLost && (
          <Alert className="mt-4" data-testid="source-repo-access-lost">
            <TriangleAlert />
            <AlertTitle>{t("services.sourceRepoAccessLostTitle")}</AlertTitle>
            <AlertDescription>
              <p>{t("services.sourceRepoAccessLostBody")}</p>
            </AlertDescription>
          </Alert>
        )}
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
  builder,
  buildCommand: currentBuildCommand,
  pushDeliveryMethod,
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
  // The service's repository when the workspace's GitHub connection grants it:
  // a repository to pick on the GitHub tab, not a URL to paste, a private one
  // least of all.
  const connectedRepo =
    !buildSourceId && repo && pushDeliveryMethod === "github_app" ? repo : null;
  const isOwnRepo = (picked: RepoView) =>
    Boolean(repo && sameRepo(picked.htmlUrl, repo));
  const initialTab: SourceTab = buildSourceId
    ? "build-source"
    : connectedRepo
      ? "github"
      : repo
        ? "git"
        : "image";
  const initialGitUrl = connectedRepo ? "" : (repo ?? "");
  const [tab, setTab] = useState<SourceTab>(initialTab);
  const [sourceId, setSourceId] = useState(buildSourceId ?? "");
  const [startCommand, setStartCommand] = useState(currentStartCommand ?? "");
  const [buildCommand, setBuildCommand] = useState(currentBuildCommand ?? "");
  const {
    sources,
    loading: sourcesLoading,
    error: sourcesError,
  } = useBuildSources(open);
  const selectedSource = sources.find((source) => source.id === sourceId);
  const needsStart = selectedSource?.nativeRuntime != null;
  const [selectedRepo, setSelectedRepo] = useState<RepoView | null>(null);
  const [gitUrl, setGitUrl] = useState(initialGitUrl);
  const [branch, setBranch] = useState(currentBranch ?? "main");
  const [image, setImageValue] = useState(imagePath ?? "");
  const [credentialId, setCredentialId] = useState(registryCredentialId ?? "");

  // The GitHub tab's repository: the one picked, else the service's own. The
  // service's own keeps the URL it already stores, which may end in ".git"
  // where GitHub's page URL does not, so picking it again changes nothing.
  const githubRepo = selectedRepo
    ? isOwnRepo(selectedRepo)
      ? (repo ?? "")
      : selectedRepo.htmlUrl
    : (connectedRepo ?? "");
  const sourceRepo = tab === "github" ? githubRepo : gitUrl;
  const gitTab = tab === "github" || tab === "git";
  // A switch from an image back to Git builds natively again, which bex-api
  // refuses without both commands, as create does (w32/052).
  const needsCommands =
    gitTab && !repo && !buildSourceId && builder === "native";
  const commandsGiven =
    buildCommand.trim() !== "" && startCommand.trim() !== "";
  const { branches } = useRepoBranches(
    open && tab !== "image" ? sourceRepo || null : null,
  );
  const branchOptions = useMemo(
    () => branches.map((value) => ({ value, label: value })),
    [branches],
  );
  const repoValid =
    tab === "github"
      ? githubRepo !== ""
      : isValidGitUrl(gitUrl) && !parseGitHubPage(gitUrl)?.file;
  const valid =
    tab === "build-source"
      ? selectedSource !== undefined &&
        (!needsStart || startCommand.trim() !== "")
      : tab === "image"
        ? isValidDockerImageRef(image)
        : repoValid && (!needsCommands || commandsGiven);
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
  // Why Update source is disabled. A malformed URL or image also has its own
  // message under the field; the footer still says the form is not ready.
  const idleReason =
    tab === "github" && !repoValid
      ? t("services.sourceSelectRepo")
      : tab === "git" && gitUrl.trim() === ""
        ? t("services.sourceEnterRepoUrl")
        : tab === "image" && image.trim() === ""
          ? t("services.sourceEnterImage")
          : tab === "build-source" && selectedSource === undefined
            ? sourcesLoading
              ? null
              : t(
                  sources.length > 0
                    ? "services.sourceSelectBuildSource"
                    : "services.sourceNoBuildSources",
                )
            : !valid
              ? t("services.sourceFixField")
              : !dirty
                ? t("services.sourceNoChanges")
                : null;
  // What Update source will save. The choice is otherwise a check mark on one
  // row of a list that scrolls and filters: out of sight, nothing named it.
  // A GitHub tab's repository reads "owner/name": the tab names the forge. A
  // typed URL keeps its host in view (formatRepoLabel).
  const pendingRepo =
    tab === "github"
      ? (repoSlug(sourceRepo) ?? sourceRepo)
      : formatRepoLabel(sourceRepo.trim());
  const pendingSource =
    tab === "build-source"
      ? (selectedSource?.name ?? "")
      : tab === "image"
        ? image.trim()
        : `${pendingRepo} · ${branch.trim() || "main"}`;
  const footerStatus =
    idleReason ??
    (valid && dirty
      ? t("services.sourcePending", { source: pendingSource })
      : "");

  function resetDraft() {
    setTab(initialTab);
    setSourceId(buildSourceId ?? "");
    setStartCommand(currentStartCommand ?? "");
    setBuildCommand(currentBuildCommand ?? "");
    setSelectedRepo(null);
    setGitUrl(initialGitUrl);
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
    // Another repository starts on its default branch; the service's own
    // keeps the branch the service deploys.
    setBranch((isOwnRepo(next) ? currentBranch : next.defaultBranch) || "main");
  }

  function selectSource(id: string) {
    setSourceId(id);
    const next = sources.find((source) => source.id === id);
    if (next && !startCommand.trim())
      setStartCommand(defaultStartCommand(next));
  }

  // Each tab is a different height, and CSS cannot ease between two `auto`
  // heights: after a switch the dialog's height is animated from what it was.
  // Read from a ResizeObserver, not an effect on `tab`: Radix mounts the new
  // panel one render after the tab changes, so an effect still measures the
  // old height. The body does not scroll meanwhile, or a scrollbar flashes
  // while a taller tab is still growing into its height.
  const [dialog, setDialog] = useState<HTMLDivElement | null>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const tabSwitchedRef = useRef(false);
  useEffect(() => {
    if (!dialog) return;
    let height = dialog.offsetHeight;
    const observer = new ResizeObserver(() => {
      const from = height;
      height = dialog.offsetHeight;
      if (!tabSwitchedRef.current || from === height) return;
      tabSwitchedRef.current = false;
      if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
      const timing = { duration: 200, easing: "ease-out" };
      dialog.animate(
        [{ height: `${from}px` }, { height: `${height}px` }],
        timing,
      );
      bodyRef.current?.animate(
        [{ overflowY: "hidden" }, { overflowY: "hidden" }],
        timing,
      );
    });
    observer.observe(dialog);
    return () => observer.disconnect();
  }, [dialog]);

  function changeTab(next: SourceTab) {
    tabSwitchedRef.current = true;
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
              ...(needsCommands && {
                buildCommand: commandText(buildCommand),
                startCommand: commandText(startCommand),
              }),
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
      {/* Header and footer stay put and the body between them scrolls, so the
          primary action is on screen at any height. Hung from where its
          tallest tab starts, not centred: each tab is a different height, and
          a centred dialog moved the tab row out from under the pointer on
          every switch (its height eases to the new tab's, above).
          A column, not the default grid: a grid track grows to its widest
          unwrappable line (a long repository name), which pushed the whole
          dialog sideways on a phone. */}
      <DialogContent
        ref={setDialog}
        className="top-4 flex max-h-[calc(100dvh-2rem)] translate-y-0 flex-col gap-0 overflow-hidden p-0 max-sm:has-[[data-repo-list]]:h-[calc(100dvh-2rem)] sm:top-[max(1rem,calc((100dvh-44rem)/2))] sm:max-h-[min(calc(100dvh-2rem),44rem)] sm:max-w-2xl"
      >
        <DialogHeader className="shrink-0 border-b px-6 pt-6 pb-4 text-left">
          <DialogTitle className="pr-8">
            {t("services.sourceDialogTitle")}
          </DialogTitle>
          <DialogDescription className="text-pretty">
            {t("services.sourceNoAutoDeploy")}
          </DialogDescription>
        </DialogHeader>
        {/* On a phone the repository list fills what the screen has left,
            in any language: with it showing the dialog is the screen's
            height (`has-[[data-repo-list]]` above), and the picker grows
            into this column (`fillHeight`). A height worked out from the
            viewport fitted English only: German's longer title and source
            names pushed the Branch hint under the footer. The list never
            goes under three rows; past that this body scrolls. */}
        <div
          ref={bodyRef}
          className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4"
        >
          <ServiceSourcePicker
            tab={tab}
            onTabChange={changeTab}
            selectedRepo={selectedRepo}
            onSelectRepo={selectRepo}
            currentRepo={connectedRepo}
            gitUrl={gitUrl}
            onGitUrlChange={setGitUrl}
            // The page names its branch; a branch the user changed is kept.
            onGitHubPage={(page) => {
              if (branch === (currentBranch || "main")) setBranch(page.branch);
            }}
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
            fillHeight
          />
          {/* Always there on a git tab, waiting for a repository: appearing
              and disappearing with each keystroke of the URL, it moved the
              footer under the pointer. */}
          {gitTab && (
            <div className="space-y-2">
              <Label>{t("services.buildDeployBranchLabel")}</Label>
              <Combobox
                value={branch}
                onValueChange={setBranch}
                options={branchOptions}
                disabled={!repoValid}
                allowCustom
                placeholder={t("services.buildDeployBranchPlaceholder")}
                emptyText={t("services.sourceBranchEmpty")}
                ariaLabel={t("services.buildDeployBranchLabel")}
                className={cn(
                  CREATE_INPUT_CLASS,
                  "font-mono focus-visible:ring-ring",
                )}
              />
              <p className="text-xs text-muted-foreground">
                {t(
                  repoValid
                    ? "services.sourceBranchHint"
                    : tab === "git"
                      ? "services.sourceBranchLockedUrl"
                      : "services.sourceBranchLocked",
                )}
              </p>
            </div>
          )}
          {needsCommands && (
            <div className="space-y-4" data-testid="source-native-commands">
              <p className="text-xs text-muted-foreground">
                {t("services.sourceNativeCommandsHint")}
              </p>
              <div className="space-y-2">
                <Label htmlFor="update-source-git-build-command">
                  {t("services.buildCommandLabel")}
                </Label>
                <Input
                  id="update-source-git-build-command"
                  value={buildCommand}
                  onChange={(e) => setBuildCommand(e.target.value)}
                  className={cn(CREATE_INPUT_CLASS, "font-mono")}
                  autoComplete="off"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="update-source-git-start-command">
                  {t("services.startCommandLabel")}
                </Label>
                <Input
                  id="update-source-git-start-command"
                  value={startCommand}
                  onChange={(e) => setStartCommand(e.target.value)}
                  className={cn(CREATE_INPUT_CLASS, "font-mono")}
                  autoComplete="off"
                />
              </div>
            </div>
          )}
        </div>
        {/* Side by side on a phone too: stacked, the two buttons cost the
            list a row. */}
        <DialogFooter className="shrink-0 flex-row flex-wrap items-center justify-end gap-x-2 gap-y-3 border-t px-6 py-4">
          {/* Always there, one line tall: why nothing can be saved, or what
              will be. Coming and going, it moved the buttons at the moment
              one of them became pressable. */}
          <p
            id="update-source-status"
            aria-live="polite"
            title={footerStatus}
            className={cn(
              "min-h-5 min-w-0 truncate text-sm max-sm:basis-full sm:flex-1",
              idleReason ? "text-muted-foreground" : "text-foreground",
            )}
          >
            {footerStatus}
          </p>
          <Button
            variant="ghost"
            className={FOOTER_BUTTON_CLASS}
            onClick={() => setOpen(false)}
          >
            {t("services.buildDeployCancel")}
          </Button>
          <Button
            className={FOOTER_PRIMARY_CLASS}
            disabled={!valid || !dirty || busy}
            aria-describedby={footerStatus ? "update-source-status" : undefined}
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
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="update-source-build-source">
          {t("services.sourceBuildSourceLabel")}
        </Label>
        <Select value={value} onValueChange={onValueChange}>
          <SelectTrigger
            id="update-source-build-source"
            className={CREATE_SELECT_CLASS}
          >
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
            className={cn(CREATE_INPUT_CLASS, "font-mono")}
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
