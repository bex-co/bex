import { useState } from "react";
import { Pencil, Check, X, Loader2, Plus, Trash2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Switch } from "@/common/components/ui/switch";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useBranch } from "@/features/services/hooks/use-branch";
import { useRootDir } from "@/features/services/hooks/use-root-dir";
import { useStartCommand } from "@/features/services/hooks/use-start-command";
import { useBuildCommand } from "@/features/services/hooks/use-build-command";
import { useDockerfilePath } from "@/features/services/hooks/use-dockerfile-path";
import { usePreDeployCommand } from "@/features/services/hooks/use-pre-deploy-command";
import { useAutoDeploy } from "@/features/services/hooks/use-auto-deploy";
import { useBuildFilter } from "@/features/services/hooks/use-build-filter";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { rootDirPrefix } from "@/features/services/lib/format";
import type { BuildFilterView } from "@/features/services/types";

export interface BuildDeploySectionProps {
  serviceId: string;
  /** spec.repo — always set for a build-from-git App (this section only renders then). */
  repo: string;
  /** spec.branch, or null while loading. */
  branch: string | null;
  /** spec.rootDir; empty/null builds from the repo root. */
  rootDir: string | null;
  /** Render runtime; docker enables Dockerfile Path and uses Docker Command. */
  runtime?: string | null;
  /** Legacy build-strategy fallback for Apps created before runtime existed. */
  builder?: string | null;
  /** spec.startCommand; presented as Docker Command for the docker runtime. */
  startCommand?: string | null;
  /** spec.dockerfilePath, relative to rootDir; empty means Dockerfile. */
  dockerfilePath?: string | null;
  /**
   * spec.buildFilter — Render's Build Filters (glob paths/ignoredPaths gating
   * push auto-deploys); null/undefined means no filter (every matching push
   * deploys). Optional so callers that don't select it fall back to no filter.
   */
  buildFilter?: BuildFilterView | null;
  /** spec.autoDeploy — whether a signed git push redeploys this App. */
  autoDeploy: boolean;
  /** spec.preDeployCommand; empty/null means no pre-deploy step (w1/m33). */
  preDeployCommand: string | null;
  /**
   * Whether to show the Pre-Deploy Command field — true for web/private/worker,
   * false for cron_job/static_site (the pre-deploy step doesn't apply there, and
   * the backend rejects setPreDeployCommand for them).
   */
  showPreDeployCommand: boolean;
  /** False for cron/static services, which have no service start command here. */
  showStartCommand?: boolean;
  /** False for cron/static services, whose Docker build settings live elsewhere. */
  showDockerfilePath?: boolean;
  /** spec.buildCommand; the shell command that produces build artifacts. */
  buildCommand?: string | null;
  /** True for static_site — shows the Build Command editor (w7/m41). */
  showBuildCommand?: boolean;
}

/**
 * The Settings tab's "Build & Deploy" section (w5/m13, Render parity — layout
 * captured live from Render's own Settings → Build panel,
 * `.playwright-mcp/render-build-deploy-settings.png`): Source + Branch
 * read-only (bex has no write path for them yet), Root Directory editable
 * inline (pencil → input → confirm, following `w5/m7`'s plan-picker confirm
 * pattern since a change triggers a rebuild). Only rendered for a
 * build-from-git App — an image-backed App has nothing to build.
 */
export function BuildDeploySection({
  serviceId,
  repo,
  branch,
  rootDir,
  runtime = null,
  builder = null,
  startCommand = null,
  dockerfilePath = null,
  buildFilter,
  autoDeploy,
  preDeployCommand,
  showPreDeployCommand,
  showStartCommand = false,
  showDockerfilePath = true,
  buildCommand = null,
  showBuildCommand = false,
}: BuildDeploySectionProps) {
  const { t } = useTranslations();
  const { setRootDir, busy } = useRootDir();
  const { setBranch, busy: branchBusy } = useBranch();
  const { setStartCommand, busy: startCommandBusy } = useStartCommand();
  const { setBuildCommand, busy: buildCommandBusy } = useBuildCommand();
  const { setDockerfilePath, busy: dockerfilePathBusy } = useDockerfilePath();
  const { setPreDeployCommand, busy: preDeployBusy } = usePreDeployCommand();
  const { setAutoDeploy, busy: autoDeployBusy } = useAutoDeploy();
  const { connection } = useGitConnection();
  // Optimistic switch state — reverted on a failed mutation.
  const [autoDeployOn, setAutoDeployOn] = useState(autoDeploy);
  // Pre-deploy command inline edit (w1/m33): a plain pencil→input→save flow, no
  // confirm dialog — Render edits this field inline with a Save button, and it
  // has its own state so it can't collide with the Root Directory edit above.
  const [preMode, setPreMode] = useState<"view" | "editing">("view");
  const [preDraft, setPreDraft] = useState("");

  // A repo hosted on the connected GitHub account auto-deploys hands-free via
  // the app's app-wide webhook; otherwise a push needs the manual HMAC webhook.
  // (The backend does the precise repo-grant match; this is the UI hint.)
  const viaGitHub = !!connection?.connected && /github\.com[/:]/i.test(repo);

  async function handleAutoDeployChange(next: boolean) {
    setAutoDeployOn(next);
    const ok = await setAutoDeploy(serviceId, next);
    if (!ok) setAutoDeployOn(!next); // revert
  }

  const preCurrent = preDeployCommand ?? "";
  const preCanSave = preDraft.trim() !== preCurrent;

  function startPreEdit() {
    setPreDraft(preCurrent);
    setPreMode("editing");
  }

  async function handlePreSave() {
    const ok = await setPreDeployCommand(serviceId, preDraft.trim());
    if (ok) setPreMode("view");
  }

  const dockerfileBuild =
    showDockerfilePath &&
    (runtime === "docker" || (!runtime && builder === "dockerfile"));
  const dockerCommand =
    runtime === "docker" || (!runtime && builder === "dockerfile");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.buildDeployTitle")}</CardTitle>
        <CardDescription>
          {t("services.buildDeployDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div>
          <div className="text-sm text-muted-foreground">
            {t("services.buildDeploySourceLabel")}
          </div>
          <div className="mt-1 font-mono text-sm break-all">{repo}</div>
        </div>
        {/* Branch is editable (w5/m48/t005, Render parity — Render offers a
            searchable branch picker; bex edits it inline like Root Directory).
            A change persists spec.branch: the next deploy builds the new
            branch and push-to-deploy matches pushes against it. */}
        <InlineEditSetting
          label={t("services.buildDeployBranchLabel")}
          hint={t("services.buildDeployBranchHint")}
          currentValue={branch ?? ""}
          emptyValue={t("services.buildDeployBranchEmpty")}
          confirmEmptyValue={t("services.buildDeployBranchEmpty")}
          placeholder={t("services.buildDeployBranchPlaceholder")}
          editLabel={t("services.buildDeployBranchEdit")}
          confirmTitle={(value) =>
            t("services.buildDeployBranchConfirmTitle", { value })
          }
          confirmBody={t("services.buildDeployBranchConfirmBody")}
          busy={branchBusy}
          // An emptied input restores the backend default (the shared verb
          // treats explicit empty as "back to main"); the confirm dialog
          // names it via confirmEmptyValue.
          onSave={(value) => setBranch(serviceId, value)}
        />
        <InlineEditSetting
          label={t("services.buildDeployRootDirLabel")}
          hint={t("services.buildDeployRootDirHint")}
          currentValue={rootDir ?? ""}
          emptyValue={t("services.buildDeployRootDirEmpty")}
          confirmEmptyValue={t("services.buildDeployConfirmRoot")}
          placeholder={t("services.buildDeployRootDirPlaceholder")}
          editLabel={t("services.buildDeployEdit")}
          confirmTitle={(value) =>
            t("services.buildDeployConfirmTitle", { value })
          }
          confirmBody={t("services.buildDeployConfirmBody")}
          optional
          busy={busy}
          onSave={(value) => setRootDir(serviceId, value)}
        />

        {showBuildCommand && (
          <InlineEditSetting
            label={t("services.buildCommandLabel")}
            hint={t("services.buildCommandHint")}
            // Render's root-directory affordance (w5/m48/t004): the command
            // runs from rootDir, so the input carries an "<rootDir>/ $" prompt.
            valuePrefix={
              rootDirPrefix(rootDir) ? rootDirPrefix(rootDir) + " $" : undefined
            }
            currentValue={buildCommand ?? ""}
            emptyValue={t("services.buildCommandEmpty")}
            confirmEmptyValue={t("services.buildCommandConfirmEmpty")}
            placeholder={t("services.buildCommandPlaceholder")}
            editLabel={t("services.buildCommandEdit")}
            confirmTitle={(value) =>
              t("services.buildCommandConfirmTitle", { value })
            }
            confirmBody={t("services.buildCommandConfirmBody")}
            optional
            busy={buildCommandBusy}
            onSave={(value) => setBuildCommand(serviceId, value)}
          />
        )}

        {showStartCommand && (
          <InlineEditSetting
            label={t(
              dockerCommand
                ? "services.dockerCommandLabel"
                : "services.startCommandLabel",
            )}
            hint={t(
              dockerCommand
                ? "services.dockerCommandHint"
                : "services.startCommandHint",
            )}
            currentValue={startCommand ?? ""}
            emptyValue={t("services.startCommandEmpty")}
            confirmEmptyValue={t("services.startCommandConfirmEmpty")}
            placeholder={t(
              dockerCommand
                ? "services.dockerCommandPlaceholder"
                : "services.startCommandPlaceholder",
            )}
            editLabel={t(
              dockerCommand
                ? "services.dockerCommandEdit"
                : "services.startCommandEdit",
            )}
            confirmTitle={(value) =>
              t(
                dockerCommand
                  ? "services.dockerCommandConfirmTitle"
                  : "services.startCommandConfirmTitle",
                { value },
              )
            }
            confirmBody={t("services.startCommandConfirmBody")}
            optional={dockerCommand}
            busy={startCommandBusy}
            onSave={(value) => setStartCommand(serviceId, value)}
          />
        )}

        {dockerfileBuild && (
          <InlineEditSetting
            label={t("services.dockerfilePathLabel")}
            hint={t("services.dockerfilePathHint")}
            currentValue={dockerfilePath ?? ""}
            emptyValue={t("services.dockerfilePathEmpty")}
            confirmEmptyValue={t("services.dockerfilePathConfirmEmpty")}
            placeholder={t("services.dockerfilePathPlaceholder")}
            editLabel={t("services.dockerfilePathEdit")}
            confirmTitle={(value) =>
              t("services.dockerfilePathConfirmTitle", { value })
            }
            confirmBody={t("services.dockerfilePathConfirmBody")}
            optional
            busy={dockerfilePathBusy}
            onSave={(value) => setDockerfilePath(serviceId, value)}
          />
        )}

        <BuildFilterEditor
          serviceId={serviceId}
          buildFilter={buildFilter ?? null}
        />

        {showPreDeployCommand && (
          <div>
            <div className="text-sm text-muted-foreground">
              {t("services.preDeployLabel")}
            </div>
            <div className="mt-1 text-sm text-muted-foreground">
              {t("services.preDeployHint")}
            </div>
            {preMode === "editing" ? (
              <div className="mt-2 flex items-center gap-2">
                <Input
                  value={preDraft}
                  onChange={(e) => setPreDraft(e.target.value)}
                  placeholder={t("services.preDeployPlaceholder")}
                  autoFocus
                  className="font-mono text-sm"
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && preCanSave) void handlePreSave();
                    if (e.key === "Escape") setPreMode("view");
                  }}
                />
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("services.buildDeploySave")}
                  disabled={preDeployBusy || !preCanSave}
                  onClick={() => void handlePreSave()}
                >
                  {preDeployBusy ? (
                    <Loader2 className="animate-spin" />
                  ) : (
                    <Check className="text-emerald-600" />
                  )}
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("services.buildDeployCancel")}
                  disabled={preDeployBusy}
                  onClick={() => setPreMode("view")}
                >
                  <X />
                </Button>
              </div>
            ) : (
              <div className="mt-2 flex items-center gap-2">
                {preCurrent ? (
                  <span className="font-mono text-sm break-all">
                    {preCurrent}
                  </span>
                ) : (
                  <span className="text-sm text-muted-foreground italic">
                    {t("services.preDeployEmpty")}
                  </span>
                )}
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("services.preDeployEdit")}
                  onClick={startPreEdit}
                >
                  <Pencil />
                </Button>
              </div>
            )}
          </div>
        )}

        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="text-sm text-muted-foreground">
              {t("services.autoDeployLabel")}
            </div>
            <div className="text-sm text-muted-foreground">
              {viaGitHub
                ? t("services.autoDeployViaGitHub")
                : t("services.autoDeployViaWebhook")}
            </div>
          </div>
          <Switch
            checked={autoDeployOn}
            disabled={autoDeployBusy}
            onCheckedChange={handleAutoDeployChange}
            aria-label={t("services.autoDeployLabel")}
          />
        </div>
      </CardContent>
    </Card>
  );
}

interface InlineEditSettingProps {
  label: string;
  hint: string;
  currentValue: string;
  emptyValue: string;
  confirmEmptyValue: string;
  placeholder: string;
  editLabel: string;
  confirmTitle: (value: string) => string;
  confirmBody: string;
  optional?: boolean;
  busy: boolean;
  /** Display-only prefix shown before the value (Render's root-directory
   *  affordance, e.g. "app/ $" — w5/m48/t004). Never part of the saved value. */
  valuePrefix?: string;
  onSave: (value: string) => Promise<boolean>;
}

/** Shared pencil → input → confirmation flow for rebuild-affecting settings. */
function InlineEditSetting({
  label,
  hint,
  currentValue,
  emptyValue,
  confirmEmptyValue,
  placeholder,
  editLabel,
  confirmTitle,
  confirmBody,
  optional = false,
  busy,
  valuePrefix,
  onSave,
}: InlineEditSettingProps) {
  const { t } = useTranslations();
  const [mode, setMode] = useState<"view" | "editing" | "confirming">("view");
  const [draft, setDraft] = useState("");
  const canSave = draft.trim() !== currentValue;

  function startEdit() {
    setDraft(currentValue);
    setMode("editing");
  }

  async function handleConfirm() {
    setMode("editing");
    if (await onSave(draft.trim())) setMode("view");
  }

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        {label}
        {optional && (
          <Badge variant="outline" className="text-xs font-normal">
            {t("services.buildDeployRootDirOptional")}
          </Badge>
        )}
      </div>
      <div className="mt-1 text-sm text-muted-foreground">{hint}</div>
      {mode !== "view" ? (
        <div className="mt-2 flex items-center gap-2">
          {valuePrefix ? (
            <code className="text-muted-foreground shrink-0 font-mono text-sm">
              {valuePrefix}
            </code>
          ) : null}
          <Input
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder={placeholder}
            autoFocus
            className="font-mono text-sm"
            onKeyDown={(event) => {
              if (event.key === "Enter" && canSave) setMode("confirming");
              if (event.key === "Escape") setMode("view");
            }}
          />
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.buildDeploySave")}
            disabled={busy || !canSave}
            onClick={() => setMode("confirming")}
          >
            {busy ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Check className="text-emerald-600" />
            )}
          </Button>
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.buildDeployCancel")}
            disabled={busy}
            onClick={() => setMode("view")}
          >
            <X />
          </Button>
        </div>
      ) : (
        <div className="mt-2 flex items-center gap-2">
          {currentValue ? (
            <span className="font-mono text-sm break-all">
              {valuePrefix ? (
                <span className="text-muted-foreground">{valuePrefix} </span>
              ) : null}
              {currentValue}
            </span>
          ) : (
            <span className="text-sm text-muted-foreground italic">
              {emptyValue}
            </span>
          )}
          <Button
            size="icon"
            variant="ghost"
            aria-label={editLabel}
            onClick={startEdit}
          >
            <Pencil />
          </Button>
        </div>
      )}

      <AlertDialog
        open={mode === "confirming"}
        onOpenChange={(open) => setMode(open ? "confirming" : "editing")}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmTitle(draft.trim() || confirmEmptyValue)}
            </AlertDialogTitle>
            <AlertDialogDescription>{confirmBody}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.buildDeployCancel")}
            </AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleConfirm()}>
              {t("services.buildDeploySave")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

/**
 * The Build Filters editor (w1/m34 — Render's Build & Deploy "Build Filters"
 * panel): two glob lists, Included Paths and Ignored Paths, deciding whether a
 * git push triggers an auto-deploy. Edits both lists into a single draft and
 * saves them together via `setBuildFilter` (one round-trip, like Render's bulk
 * save). Empty rows are dropped on save; two empty lists clear the filter.
 */
function BuildFilterEditor({
  serviceId,
  buildFilter,
}: {
  serviceId: string;
  buildFilter: BuildFilterView | null;
}) {
  const { t } = useTranslations();
  const { setBuildFilter, busy } = useBuildFilter();
  const current = {
    paths: buildFilter?.paths ?? [],
    ignoredPaths: buildFilter?.ignoredPaths ?? [],
  };
  const [draft, setDraft] = useState(current);
  const dirty = JSON.stringify(draft) !== JSON.stringify(current);

  async function handleSave() {
    const clean = (xs: string[]) => xs.map((x) => x.trim()).filter(Boolean);
    const paths = clean(draft.paths);
    const ignoredPaths = clean(draft.ignoredPaths);
    const ok = await setBuildFilter(serviceId, paths, ignoredPaths);
    if (ok) setDraft({ paths, ignoredPaths });
  }

  return (
    <div className="space-y-4">
      <div>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          {t("services.buildFilterLabel")}
          <Badge variant="outline" className="text-xs font-normal">
            {t("services.buildDeployRootDirOptional")}
          </Badge>
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.buildFilterHint")}
        </div>
      </div>
      <PathList
        title={t("services.buildFilterIncludedTitle")}
        hint={t("services.buildFilterIncludedHint")}
        placeholder={t("services.buildFilterIncludedPlaceholder")}
        addLabel={t("services.buildFilterAddIncluded")}
        removeLabel={t("services.buildFilterRemoveIncluded")}
        values={draft.paths}
        onChange={(paths) => setDraft((d) => ({ ...d, paths }))}
      />
      <PathList
        title={t("services.buildFilterIgnoredTitle")}
        hint={t("services.buildFilterIgnoredHint")}
        placeholder={t("services.buildFilterIgnoredPlaceholder")}
        addLabel={t("services.buildFilterAddIgnored")}
        removeLabel={t("services.buildFilterRemoveIgnored")}
        values={draft.ignoredPaths}
        onChange={(ignoredPaths) => setDraft((d) => ({ ...d, ignoredPaths }))}
      />
      <div className="flex justify-end gap-2">
        {dirty && (
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => setDraft(current)}
          >
            {t("services.buildDeployCancel")}
          </Button>
        )}
        <Button disabled={busy || !dirty} onClick={() => void handleSave()}>
          {busy && <Loader2 className="animate-spin" />}
          {t("services.buildFilterSave")}
        </Button>
      </div>
    </div>
  );
}

/**
 * One editable glob list (Included or Ignored Paths): a row of monospace inputs
 * with a per-row remove and an add button, mirroring the static-site RoutesEditor
 * shape. The parent owns the value/onChange so both lists save as one draft.
 * Exported so the create wizard can reuse it without a mutation (w7/m41).
 */
export function PathList({
  title,
  hint,
  placeholder,
  addLabel,
  removeLabel,
  values,
  onChange,
}: {
  title: string;
  hint: string;
  placeholder: string;
  addLabel: string;
  removeLabel: string;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="text-sm font-medium">{title}</div>
      <div className="text-xs text-muted-foreground">{hint}</div>
      {values.map((value, i) => (
        <div key={i} className="flex items-center gap-2">
          <Input
            value={value}
            onChange={(e) =>
              onChange(values.map((v, j) => (j === i ? e.target.value : v)))
            }
            placeholder={placeholder}
            className="font-mono text-sm"
          />
          <Button
            size="icon"
            variant="ghost"
            aria-label={removeLabel}
            onClick={() => onChange(values.filter((_, j) => j !== i))}
          >
            <Trash2 />
          </Button>
        </div>
      ))}
      <Button
        variant="outline"
        size="sm"
        onClick={() => onChange([...values, ""])}
      >
        <Plus /> {addLabel}
      </Button>
    </div>
  );
}
