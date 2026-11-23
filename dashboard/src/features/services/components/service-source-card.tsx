import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
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
import { publishDirectoryError } from "@/features/services/lib/publish-directory";
import {
  formatRepoLabel,
  repoSlug,
  sameRepo,
} from "@/features/services/lib/repo";
import {
  isNativeRuntime,
  RUNTIME_COMMANDS,
  RUNTIME_DEFS,
  type GitRuntime,
} from "@/features/services/lib/runtime";
import { useServiceBase } from "@/features/services/lib/service-base";
import {
  isCronType,
  isDockerBuild,
  isStaticSiteType,
} from "@/features/services/lib/service-type";

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

const DEFAULT_BUILD_COMMANDS = new Set(
  Object.values(RUNTIME_COMMANDS).map((c) => c.build),
);
const DEFAULT_START_COMMANDS = new Set(
  Object.values(RUNTIME_COMMANDS).map((c) => c.start),
);

function resolveSavedRuntime(
  runtime: string | null | undefined,
  builder: string | null | undefined,
  repo: string | null,
): GitRuntime | null {
  const known = RUNTIME_DEFS.find((d) => d.id === runtime)?.id;
  if (known) return known;
  if (repo && isDockerBuild({ runtime, builder })) return "docker";
  return null;
}

export interface ServiceSourceCardProps {
  serviceId: string;
  /** The service type (`web_service`, `static_site`, `cron_job`, etc.). */
  serviceType?: string | null;
  repo: string | null;
  branch: string | null;
  imagePath: string | null;
  registryCredentialId: string | null;
  /** The build source this service builds from (w2/m254), else null. */
  buildSourceId: string | null;
  /** The service's start command (or cron command), prefilled for native builds. */
  startCommand: string | null;
  /** The service's configured runtime (docker or a native runtime), else null. */
  runtime?: string | null;
  /** Legacy build strategy fallback when runtime is unset. */
  builder?: string | null;
  buildCommand?: string | null;
  /** Static site publish directory (spec.publishPath). */
  publishPath?: string | null;
  /** Repository subdirectory used to validate static-site Publish Directory. */
  rootDir?: string | null;
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
 * Render-style Settings → Source card (w5/m76, w4/m593). Shown for every
 * service type — including static sites — with Edit opening the Update Source
 * dialog to reconfirm build settings, deploy immediately, and navigate to the
 * new deploy.
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
  serviceType,
  repo,
  branch: currentBranch,
  imagePath,
  registryCredentialId,
  buildSourceId,
  startCommand: currentStartCommand,
  runtime: currentRuntime,
  builder,
  buildCommand: currentBuildCommand,
  publishPath: currentPublishPath,
  rootDir,
  pushDeliveryMethod,
  disabled,
}: ServiceSourceCardProps & { disabled: boolean }) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const base = useServiceBase();
  const { setRepo, busy: repoBusy, lastError: repoError } = useSetRepo();
  const {
    linkBuildSource,
    busy: linkBusy,
    lastError: linkError,
  } = useLinkBuildSource();
  const { setImage, busy: imageBusy, lastError: imageError } = useSetImage();

  const staticSite = isStaticSiteType(serviceType ?? "");
  const cron = isCronType(serviceType ?? "");
  const commandLabelKey = cron
    ? "services.deployCommandLabel"
    : "services.startCommandLabel";

  // The value the server last refused; editing away from it clears the
  // under-field message (w4/406).
  const [refusedImage, setRefusedImage] = useState<string | null>(null);
  const [showError, setShowError] = useState(false);
  const [open, setOpen] = useState(false);

  // The service's repository when the workspace's GitHub connection grants it:
  // a repository to pick on the GitHub tab, not a URL to paste, a private one
  // least of all.
  const connectedRepo =
    !buildSourceId && repo && pushDeliveryMethod === "github_app" ? repo : null;
  const isOwnRepo = (picked: RepoView) =>
    Boolean(repo && sameRepo(picked.htmlUrl, repo));
  const initialTab: SourceTab = staticSite
    ? connectedRepo
      ? "github"
      : "git"
    : buildSourceId
      ? "build-source"
      : connectedRepo
        ? "github"
        : repo
          ? "git"
          : "image";
  const initialGitUrl = connectedRepo ? "" : (repo ?? "");
  const savedRuntime = staticSite
    ? null
    : resolveSavedRuntime(currentRuntime, builder, repo);
  const defaultRuntime: GitRuntime =
    savedRuntime ?? (builder === "native" ? "node" : "docker");

  const [tab, setTab] = useState<SourceTab>(initialTab);
  const [sourceId, setSourceId] = useState(buildSourceId ?? "");
  const [selectedRuntime, setSelectedRuntime] =
    useState<GitRuntime>(defaultRuntime);
  const [startCommand, setStartCommand] = useState(currentStartCommand ?? "");
  const [buildCommand, setBuildCommand] = useState(currentBuildCommand ?? "");
  const [publishPath, setPublishPath] = useState(currentPublishPath ?? "");
  const [publishTouched, setPublishTouched] = useState(false);

  const {
    sources,
    loading: sourcesLoading,
    error: sourcesError,
  } = useBuildSources(open && !staticSite);
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
  const nativeGit = gitTab && !staticSite && isNativeRuntime(selectedRuntime);

  const { branches } = useRepoBranches(
    open && gitTab ? sourceRepo || null : null,
  );
  const branchOptions = useMemo(
    () => branches.map((value) => ({ value, label: value })),
    [branches],
  );
  const repoValid =
    tab === "github"
      ? githubRepo !== ""
      : isValidGitUrl(gitUrl) && !parseGitHubPage(gitUrl)?.file;

  const publishPathErrorKey = staticSite
    ? publishPath.trim() === ""
      ? publishTouched
        ? "services.sourcePublishDirectoryRequired"
        : null
      : publishDirectoryError(publishPath, { rootDir, repoBacked: true })
    : null;
  const publishPathValid =
    !staticSite ||
    (publishPath.trim() !== "" &&
      publishDirectoryError(publishPath, { rootDir, repoBacked: true }) ===
        null);
  const nativeCommandsValid =
    !nativeGit || (buildCommand.trim() !== "" && startCommand.trim() !== "");

  const valid =
    tab === "build-source"
      ? selectedSource !== undefined &&
        (!needsStart || startCommand.trim() !== "")
      : tab === "image"
        ? isValidDockerImageRef(image)
        : repoValid && publishPathValid && nativeCommandsValid;

  // An image-backed service's repo arrives as "" (bex-api), not null, so
  // source kind is repo presence — as the tab and the card already decide it
  // (w4/275). Build-setting confirmation changes also mark the form dirty so
  // reconfirming with a new runtime, command, or publish directory deploys.
  const dirty =
    tab === "build-source"
      ? sourceId !== (buildSourceId ?? "") ||
        (needsStart &&
          startCommand.trim() !== (currentStartCommand ?? "").trim())
      : tab === "image"
        ? Boolean(repo) ||
          Boolean(buildSourceId) ||
          image.trim() !== (imagePath ?? "").trim() ||
          (credentialId.trim() || null) !==
            (registryCredentialId?.trim() || null)
        : !repo ||
          sourceRepo.trim() !== repo.trim() ||
          (branch.trim() || "main") !== (currentBranch?.trim() || "main") ||
          (staticSite
            ? commandText(buildCommand) !==
                commandText(currentBuildCommand ?? "") ||
              publishPath.trim() !== (currentPublishPath ?? "").trim()
            : selectedRuntime !== savedRuntime ||
              (isNativeRuntime(selectedRuntime) &&
                (commandText(buildCommand) !==
                  commandText(currentBuildCommand ?? "") ||
                  commandText(startCommand) !==
                    commandText(currentStartCommand ?? ""))));

  const busy = repoBusy || imageBusy || linkBusy;
  const inlineError =
    !showError || tab === "image"
      ? null
      : tab === "build-source"
        ? linkError
        : repoError;

  // Why the primary action is disabled. A malformed URL, publish directory, or
  // image also has its own message under the field; the footer still says the
  // form is not ready.
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

  // What submitting will deploy. The choice is otherwise a check mark on one
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
    setSelectedRuntime(defaultRuntime);
    setStartCommand(currentStartCommand ?? "");
    setBuildCommand(currentBuildCommand ?? "");
    setPublishPath(currentPublishPath ?? "");
    setPublishTouched(false);
    setSelectedRepo(null);
    setGitUrl(initialGitUrl);
    setBranch(currentBranch || "main");
    setImageValue(imagePath ?? "");
    setCredentialId(registryCredentialId ?? "");
    setShowError(false);
  }

  function changeOpen(next: boolean) {
    if (next) resetDraft();
    setOpen(next);
  }

  function selectRepo(next: RepoView) {
    setSelectedRepo(next);
    setShowError(false);
    // Another repository starts on its default branch; the service's own
    // keeps the branch the service deploys.
    setBranch((isOwnRepo(next) ? currentBranch : next.defaultBranch) || "main");
  }

  function selectSource(id: string) {
    setSourceId(id);
    setShowError(false);
    const next = sources.find((source) => source.id === id);
    if (next && !startCommand.trim())
      setStartCommand(defaultStartCommand(next));
  }

  function changeRuntime(next: GitRuntime) {
    setSelectedRuntime(next);
    setShowError(false);
    if (!isNativeRuntime(next)) return;
    const defaults = RUNTIME_COMMANDS[next];
    const targetBuild =
      next === savedRuntime && currentBuildCommand
        ? currentBuildCommand
        : defaults.build;
    const targetStart =
      next === savedRuntime && currentStartCommand
        ? currentStartCommand
        : defaults.start;
    if (
      !buildCommand.trim() ||
      DEFAULT_BUILD_COMMANDS.has(buildCommand.trim())
    ) {
      setBuildCommand(targetBuild);
    }
    if (
      !startCommand.trim() ||
      DEFAULT_START_COMMANDS.has(startCommand.trim())
    ) {
      setStartCommand(targetStart);
    }
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
    setShowError(false);
    if (next === "github" && selectedRepo && !branch) {
      setBranch(selectedRepo.defaultBranch || "main");
    }
  }

  async function save() {
    setShowError(false);
    const result =
      tab === "build-source"
        ? await linkBuildSource(
            serviceId,
            sourceId,
            needsStart ? startCommand.trim() : undefined,
            true,
          )
        : tab === "image"
          ? await setImage(
              serviceId,
              image.trim(),
              credentialId || undefined,
              true,
            )
          : await setRepo(serviceId, {
              repo: sourceRepo.trim(),
              branch: branch.trim() || "main",
              deploy: true,
              ...(staticSite
                ? {
                    buildCommand: commandText(buildCommand),
                    publishPath: publishPath.trim(),
                  }
                : {
                    runtime: selectedRuntime,
                    ...(isNativeRuntime(selectedRuntime) && {
                      buildCommand: commandText(buildCommand),
                      ...(cron
                        ? { command: commandText(startCommand) }
                        : { startCommand: commandText(startCommand) }),
                    }),
                  }),
            });
    if (!result.ok) {
      if (tab === "image") setRefusedImage(image.trim());
      else setShowError(true);
      return;
    }
    setRefusedImage(null);
    setOpen(false);
    if (result.deployId) {
      void navigate({
        to: `${base}/$serviceId/deploys/$deployId`,
        params: { serviceId, deployId: result.deployId },
      });
    }
  }

  const primaryLabel =
    tab === "image"
      ? t(busy ? "services.sourceConnecting" : "services.sourceConnect")
      : t(busy ? "services.sourceDeploying" : "services.sourceDeploy");

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
            {t(
              staticSite
                ? "services.sourceDialogDescriptionStatic"
                : "services.sourceDialogDescription",
            )}
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
            image={
              staticSite
                ? undefined
                : {
                    value: image,
                    onChange: setImageValue,
                    registryCredentialId: credentialId,
                    onRegistryCredentialChange: setCredentialId,
                    showPortHint: false,
                    serverError:
                      refusedImage !== null && refusedImage === image.trim()
                        ? imageError
                        : null,
                  }
            }
            buildSource={
              staticSite ? undefined : (
                <BuildSourceTab
                  sources={sources}
                  loading={sourcesLoading}
                  failed={sourcesError !== undefined}
                  value={sourceId}
                  onValueChange={selectSource}
                  startCommand={needsStart ? startCommand : null}
                  onStartCommandChange={setStartCommand}
                  commandLabelKey={commandLabelKey}
                />
              )
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
          {gitTab && staticSite && (
            <div className="space-y-4" data-testid="source-static-settings">
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
                <p className="text-xs text-muted-foreground">
                  {t("services.sourceBuildCommandOptionalHint")}
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="update-source-publish-directory">
                  {t("services.publishPathLabel")}
                </Label>
                <Input
                  id="update-source-publish-directory"
                  value={publishPath}
                  onChange={(e) => {
                    setPublishPath(e.target.value);
                    setPublishTouched(true);
                  }}
                  onBlur={() => setPublishTouched(true)}
                  placeholder={t("services.publishPathPlaceholder")}
                  aria-invalid={publishPathErrorKey !== null}
                  className={cn(CREATE_INPUT_CLASS, "font-mono")}
                  autoComplete="off"
                />
                {publishPathErrorKey && (
                  <p role="alert" className="text-xs text-destructive">
                    {t(publishPathErrorKey)}
                  </p>
                )}
              </div>
            </div>
          )}
          {gitTab && !staticSite && (
            <div className="space-y-4" data-testid="source-git-build-settings">
              <div className="space-y-2">
                <Label htmlFor="update-source-runtime">
                  {t("services.runtimeLabel")}
                </Label>
                <Select
                  value={selectedRuntime}
                  onValueChange={(value) => changeRuntime(value as GitRuntime)}
                >
                  <SelectTrigger
                    id="update-source-runtime"
                    aria-label={t("services.runtimeLabel")}
                    className={CREATE_SELECT_CLASS}
                  >
                    <SelectValue
                      placeholder={t("services.runtimePlaceholder")}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    {RUNTIME_DEFS.map((def) => (
                      <SelectItem key={def.id} value={def.id}>
                        {t(def.labelKey)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {nativeGit && (
                <div
                  className="space-y-4"
                  data-testid="source-native-commands"
                >
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
                      {t(commandLabelKey)}
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
          )}
          {inlineError && (
            <Alert variant="destructive" data-testid="source-submit-error">
              <TriangleAlert />
              <AlertDescription>{inlineError}</AlertDescription>
            </Alert>
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
            {primaryLabel}
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
  commandLabelKey,
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
  commandLabelKey: "services.startCommandLabel" | "services.deployCommandLabel";
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
            {t(commandLabelKey)}
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
