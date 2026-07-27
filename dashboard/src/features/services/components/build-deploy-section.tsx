import { useState } from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
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
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { DeployHookRows } from "@/features/services/components/deploy-hook-section";
import { useBranch } from "@/features/services/hooks/use-branch";
import { useRootDir } from "@/features/services/hooks/use-root-dir";
import { useStartCommand } from "@/features/services/hooks/use-start-command";
import { useBuildCommand } from "@/features/services/hooks/use-build-command";
import { useDockerfilePath } from "@/features/services/hooks/use-dockerfile-path";
import { usePreDeployCommand } from "@/features/services/hooks/use-pre-deploy-command";
import { useAutoDeploy } from "@/features/services/hooks/use-auto-deploy";
import { useBuildFilter } from "@/features/services/hooks/use-build-filter";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { commandPromptPrefix } from "@/features/services/lib/format";
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
  /**
   * Whether to render a separate "Deploy" card (Pre-Deploy, Start/Docker
   * Command, Auto-Deploy, Deploy Hook) after the "Build" card (Render's split,
   * w5/m52). False for a cron_job, whose deploy concerns live in its own Deploy
   * (Schedule/Command) section — there Auto-Deploy folds into the Build card and
   * the Deploy Hook stays a standalone card.
   */
  showDeployCard?: boolean;
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
  showDeployCard = true,
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

  // A repo hosted on the connected GitHub account auto-deploys hands-free via
  // the app's app-wide webhook; otherwise a push needs the manual HMAC webhook.
  // (The backend does the precise repo-grant match; this is the UI hint.)
  const viaGitHub = !!connection?.connected && /github\.com[/:]/i.test(repo);

  async function handleAutoDeployChange(next: boolean) {
    setAutoDeployOn(next);
    const ok = await setAutoDeploy(serviceId, next);
    if (!ok) setAutoDeployOn(!next); // revert
  }

  const dockerfileBuild =
    showDockerfilePath &&
    (runtime === "docker" || (!runtime && builder === "dockerfile"));
  const dockerCommand =
    runtime === "docker" || (!runtime && builder === "dockerfile");

  // Auto-Deploy toggle — lives in the Deploy card (web/static) or, for a cron_job
  // with no Deploy card, folds into the bottom of the Build card.
  const autoDeploySwitch = (
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
  );

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("services.buildTitle")}</CardTitle>
          <CardDescription>{t("services.buildDescription")}</CardDescription>
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
              An emptied input restores the backend default ("back to main"); the
              confirm dialog names it via the empty stand-in. */}
          <EditableFieldRow
            label={t("services.buildDeployBranchLabel")}
            hint={t("services.buildDeployBranchHint")}
            value={branch ?? ""}
            placeholder={t("services.buildDeployBranchPlaceholder")}
            editLabel={t("services.buildDeployBranchEdit")}
            mono
            busy={branchBusy}
            confirm={{
              title: (value) =>
                t("services.buildDeployBranchConfirmTitle", { value }),
              body: t("services.buildDeployBranchConfirmBody"),
              emptyValue: t("services.buildDeployBranchEmpty"),
            }}
            onSave={(value) => setBranch(serviceId, value)}
          />
          <EditableFieldRow
            label={t("services.buildDeployRootDirLabel")}
            hint={t("services.buildDeployRootDirHint")}
            value={rootDir ?? ""}
            placeholder={t("services.buildDeployRootDirPlaceholder")}
            editLabel={t("services.buildDeployEdit")}
            optional
            mono
            busy={busy}
            confirm={{
              title: (value) =>
                t("services.buildDeployConfirmTitle", { value }),
              body: t("services.buildDeployConfirmBody"),
              emptyValue: t("services.buildDeployConfirmRoot"),
            }}
            onSave={(value) => setRootDir(serviceId, value)}
          />

          {showBuildCommand && (
            <EditableFieldRow
              label={t("services.buildCommandLabel")}
              hint={t("services.buildCommandHint")}
              // Render's root-directory affordance (w5/m48/t004, w5/m51): the
              // command runs from rootDir, so the input carries an "<rootDir>/ $"
              // prompt (bare "$" when no root dir is set).
              valuePrefix={commandPromptPrefix(rootDir)}
              value={buildCommand ?? ""}
              placeholder={t("services.buildCommandPlaceholder")}
              editLabel={t("services.buildCommandEdit")}
              optional
              mono
              busy={buildCommandBusy}
              confirm={{
                title: (value) =>
                  t("services.buildCommandConfirmTitle", { value }),
                body: t("services.buildCommandConfirmBody"),
                emptyValue: t("services.buildCommandConfirmEmpty"),
              }}
              onSave={(value) => setBuildCommand(serviceId, value)}
            />
          )}

          {dockerfileBuild && (
            <EditableFieldRow
              label={t("services.dockerfilePathLabel")}
              hint={t("services.dockerfilePathHint")}
              value={dockerfilePath ?? ""}
              placeholder={t("services.dockerfilePathPlaceholder")}
              editLabel={t("services.dockerfilePathEdit")}
              optional
              mono
              busy={dockerfilePathBusy}
              confirm={{
                title: (value) =>
                  t("services.dockerfilePathConfirmTitle", { value }),
                body: t("services.dockerfilePathConfirmBody"),
                emptyValue: t("services.dockerfilePathConfirmEmpty"),
              }}
              onSave={(value) => setDockerfilePath(serviceId, value)}
            />
          )}

          <BuildFilterEditor
            serviceId={serviceId}
            buildFilter={buildFilter ?? null}
          />

          {/* A cron_job has no Deploy card (its Deploy section holds the
              schedule), so Auto-Deploy folds into the bottom of Build. */}
          {!showDeployCard && autoDeploySwitch}
        </CardContent>
      </Card>

      {showDeployCard && (
        <Card>
          <CardHeader>
            <CardTitle>{t("services.deploySectionTitle")}</CardTitle>
            <CardDescription>
              {t("services.deploySectionDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {showPreDeployCommand && (
              <EditableFieldRow
                label={t("services.preDeployLabel")}
                hint={t("services.preDeployHint")}
                value={preDeployCommand ?? ""}
                // Pre-deploy runs from rootDir too — same "<rootDir>/ $" prompt (w5/m51).
                valuePrefix={commandPromptPrefix(rootDir)}
                placeholder={t("services.preDeployPlaceholder")}
                editLabel={t("services.preDeployEdit")}
                optional
                mono
                busy={preDeployBusy}
                onSave={(value) => setPreDeployCommand(serviceId, value)}
              />
            )}

            {showStartCommand && (
              <EditableFieldRow
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
                value={startCommand ?? ""}
                // A native Start Command runs from rootDir (Render's "<rootDir>/ $"
                // prompt); a Docker Command overrides the container's CMD and isn't
                // a rootDir shell command, so it carries no prompt (w5/m51).
                valuePrefix={
                  dockerCommand ? undefined : commandPromptPrefix(rootDir)
                }
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
                optional={dockerCommand}
                mono
                busy={startCommandBusy}
                confirm={{
                  title: (value) =>
                    t(
                      dockerCommand
                        ? "services.dockerCommandConfirmTitle"
                        : "services.startCommandConfirmTitle",
                      { value },
                    ),
                  body: t("services.startCommandConfirmBody"),
                  emptyValue: t("services.startCommandConfirmEmpty"),
                }}
                onSave={(value) => setStartCommand(serviceId, value)}
              />
            )}

            {autoDeploySwitch}

            {/* Deploy Hook, moved into the Deploy section (Render parity, w5/m52). */}
            <div className="space-y-4">
              <div>
                <div className="text-sm font-medium">
                  {t("services.deployHookTitle")}
                </div>
                <p className="text-muted-foreground mt-1 text-sm">
                  {t("services.deployHookDescription")}
                </p>
              </div>
              <DeployHookRows serviceId={serviceId} />
            </div>
          </CardContent>
        </Card>
      )}
    </>
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
