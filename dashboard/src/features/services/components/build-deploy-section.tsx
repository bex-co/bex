import { useMemo, useState } from "react";
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
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { DeployHookRows } from "@/features/services/components/deploy-hook-section";
import { useBranch } from "@/features/services/hooks/use-branch";
import { useRootDir } from "@/features/services/hooks/use-root-dir";
import { useStartCommand } from "@/features/services/hooks/use-start-command";
import { useBuildCommand } from "@/features/services/hooks/use-build-command";
import { useDockerfilePath } from "@/features/services/hooks/use-dockerfile-path";
import { usePreDeployCommand } from "@/features/services/hooks/use-pre-deploy-command";
import { useAutoDeploy } from "@/features/services/hooks/use-auto-deploy";
import { useBuildFilter } from "@/features/services/hooks/use-build-filter";
import { useRepoBranches } from "@/features/services/hooks/use-repo-branches";
import { useSetRepo } from "@/features/services/hooks/use-set-repo";
import { useRepos } from "@/features/services/hooks/use-repos";
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
  /** Server-computed push deliverability for THIS repo — see autoDeployHintKey. */
  pushDeliveryMethod?: string | null;
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
  /**
   * Keep the legacy inline Source/Branch rows for source-specialized variants
   * (cron/static). Ordinary services render the unified ServiceSourceCard.
   */
  showSourceFields?: boolean;
}

/**
 * The translation key for the Auto-Deploy row's source line, chosen by the
 * server's per-repo push-deliverability answer — the same predicate a deploy
 * trigger applies (lego/backend/internal/apps/pushdelivery.go). It replaces a
 * `connection.connected && /github\.com/` heuristic that promised "via the
 * GitHub app" for any github.com URL whenever the workspace had a connection at
 * all, including repos the installation does not grant, where GitHub never
 * sends a push event and no deploy ever fires (w6/m99).
 *
 * Everything else — `unknown` (GitHub unreachable), `none`, and the not-yet-
 * loaded first render — states the uncertainty rather than asserting a
 * mechanism in whichever direction happens to be wrong.
 */
function autoDeployHintKey(pushDeliveryMethod: string | null) {
  switch (pushDeliveryMethod) {
    case "github_app":
      return "services.autoDeployViaGitHub" as const;
    case "manual_webhook":
      return "services.autoDeployViaWebhook" as const;
    default:
      return "services.autoDeployDeliveryUnknown" as const;
  }
}

/**
 * The Settings tab's Build and Deploy cards (w5/m13). Ordinary services put
 * source in the unified ServiceSourceCard; cron/static variants retain the
 * legacy inline Source/Branch fields here. Source changes are saved for the
 * next deploy rather than triggering one immediately.
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
  pushDeliveryMethod = null,
  preDeployCommand,
  showPreDeployCommand,
  showStartCommand = false,
  showDockerfilePath = true,
  buildCommand = null,
  showBuildCommand = false,
  showDeployCard = true,
  showSourceFields = true,
}: BuildDeploySectionProps) {
  const { t } = useTranslations();
  // Choosing what a service builds and runs (source, branch, root dir, commands,
  // Dockerfile path, pre-deploy) is can_create — a contributor is refused on save
  // (docs/ADR024-members.md). Disable those rows with a reason instead (w9/m84);
  // Auto-Deploy stays editable (it is can_operate, contributor-and-up).
  const { canCreate, canOperate } = useCapabilities();
  const createDisabled = !canCreate;
  const createReason = createDisabled
    ? t("capabilities.reasonCanCreate")
    : undefined;
  const { setRootDir, busy } = useRootDir();
  const { setStartCommand, busy: startCommandBusy } = useStartCommand();
  const { setBuildCommand, busy: buildCommandBusy } = useBuildCommand();
  const { setDockerfilePath, busy: dockerfilePathBusy } = useDockerfilePath();
  const { setPreDeployCommand, busy: preDeployBusy } = usePreDeployCommand();
  const { setAutoDeploy, busy: autoDeployBusy } = useAutoDeploy();
  // Optimistic switch state — reverted on a failed mutation.
  const [autoDeployOn, setAutoDeployOn] = useState(autoDeploy);

  const autoDeployHint = t(autoDeployHintKey(pushDeliveryMethod));

  // Render presents Auto-Deploy as a select ("On Commit" | "Off"), not a switch
  // (w5/m53). bex has only two states — its boolean spec.autoDeploy maps to
  // Render's autoDeployTrigger "commit"/"off"; "checksPass" is unsupported.
  const autoDeployOptions = useMemo(
    () => [
      { value: "commit", label: t("services.autoDeployOnCommit") },
      { value: "off", label: t("services.autoDeployOff") },
    ],
    [t],
  );

  const dockerfileBuild =
    showDockerfilePath &&
    (runtime === "docker" || (!runtime && builder === "dockerfile"));
  const dockerCommand =
    runtime === "docker" || (!runtime && builder === "dockerfile");

  // Auto-Deploy select (Render's disabled-select-with-pencil, w5/m53) — lives in
  // the Deploy card (web/static) or, for a cron_job with no Deploy card, folds
  // into the bottom of the Build card.
  const autoDeployRow = (
    <EditableFieldRow
      label={t("services.autoDeployLabel")}
      hint={autoDeployHint}
      value={autoDeployOn ? "commit" : "off"}
      editLabel={t("services.autoDeployEdit")}
      busy={autoDeployBusy}
      options={autoDeployOptions}
      onSave={async (value) => {
        const next = value === "commit";
        setAutoDeployOn(next); // optimistic
        const ok = await setAutoDeploy(serviceId, next);
        if (!ok) setAutoDeployOn(!next); // revert
        return ok;
      }}
    />
  );

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("services.buildTitle")}</CardTitle>
          <CardDescription>{t("services.buildDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {showSourceFields && (
            <LegacySourceFields
              serviceId={serviceId}
              repo={repo}
              branch={branch}
              disabled={createDisabled}
              disabledReason={createReason}
            />
          )}
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
            disabled={createDisabled}
            disabledReason={createReason}
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
              disabled={createDisabled}
              disabledReason={createReason}
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
              disabled={createDisabled}
              disabledReason={createReason}
              onSave={(value) => setDockerfilePath(serviceId, value)}
            />
          )}

          <BuildFilterEditor
            serviceId={serviceId}
            buildFilter={buildFilter ?? null}
            canOperate={canOperate}
          />

          {/* A cron_job has no Deploy card (its Deploy section holds the
              schedule), so Auto-Deploy folds into the bottom of Build. */}
          {!showDeployCard && autoDeployRow}
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
                disabled={createDisabled}
                disabledReason={createReason}
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
                disabled={createDisabled}
                disabledReason={createReason}
                onSave={(value) => setStartCommand(serviceId, value)}
              />
            )}

            {autoDeployRow}

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

/** Legacy source rows retained only by cron/static settings variants. */
function LegacySourceFields({
  serviceId,
  repo,
  branch,
  disabled,
  disabledReason,
}: {
  serviceId: string;
  repo: string;
  branch: string | null;
  disabled: boolean;
  disabledReason?: string;
}) {
  const { t } = useTranslations();
  const { setBranch, busy: branchBusy } = useBranch();
  const { branches } = useRepoBranches(repo);
  const branchOptions = useMemo(
    () => branches.map((value) => ({ value, label: value })),
    [branches],
  );
  const { setRepo, busy: repoBusy } = useSetRepo();
  const { repos } = useRepos();
  const repoOptions = useMemo(
    () =>
      repos.map((candidate) => ({
        value: candidate.htmlUrl,
        label: candidate.fullName,
      })),
    [repos],
  );

  return (
    <>
      <EditableFieldRow
        label={t("services.buildDeploySourceLabel")}
        hint={t("services.buildDeploySourceHint")}
        value={repo}
        placeholder={t("services.buildDeploySourcePlaceholder")}
        editLabel={t("services.buildDeploySourceEdit")}
        mono
        busy={repoBusy}
        comboboxOptions={repoOptions}
        confirm={{
          title: (value) =>
            t("services.buildDeploySourceConfirmTitle", { value }),
          body: t("services.sourceNoAutoDeploy"),
        }}
        disabled={disabled}
        disabledReason={disabledReason}
        onSave={(value) => setRepo(serviceId, { repo: value })}
      />
      <EditableFieldRow
        label={t("services.buildDeployBranchLabel")}
        hint={t("services.buildDeployBranchHint")}
        value={branch ?? ""}
        placeholder={t("services.buildDeployBranchPlaceholder")}
        editLabel={t("services.buildDeployBranchEdit")}
        mono
        busy={branchBusy}
        comboboxOptions={branchOptions}
        confirm={{
          title: (value) =>
            t("services.buildDeployBranchConfirmTitle", { value }),
          body: t("services.sourceNoAutoDeploy"),
          emptyValue: t("services.buildDeployBranchEmpty"),
        }}
        disabled={disabled}
        disabledReason={disabledReason}
        onSave={(value) => setBranch(serviceId, value)}
      />
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
  canOperate,
}: {
  serviceId: string;
  buildFilter: BuildFilterView | null;
  canOperate: boolean;
}) {
  const { t } = useTranslations();
  const { setBuildFilter, busy } = useBuildFilter();
  // SetBuildFilter is RelCanOperate, same class as Auto-Deploy.
  const operateReason = canOperate
    ? undefined
    : t("capabilities.reasonCanOperate");
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
        {operateReason && (
          <p className="text-muted-foreground mt-1 text-sm">{operateReason}</p>
        )}
      </div>
      <PathList
        title={t("services.buildFilterIncludedTitle")}
        hint={t("services.buildFilterIncludedHint")}
        placeholder={t("services.buildFilterIncludedPlaceholder")}
        addLabel={t("services.buildFilterAddIncluded")}
        removeLabel={t("services.buildFilterRemoveIncluded")}
        values={draft.paths}
        onChange={(paths) => setDraft((d) => ({ ...d, paths }))}
        disabled={!canOperate}
      />
      <PathList
        title={t("services.buildFilterIgnoredTitle")}
        hint={t("services.buildFilterIgnoredHint")}
        placeholder={t("services.buildFilterIgnoredPlaceholder")}
        addLabel={t("services.buildFilterAddIgnored")}
        removeLabel={t("services.buildFilterRemoveIgnored")}
        values={draft.ignoredPaths}
        onChange={(ignoredPaths) => setDraft((d) => ({ ...d, ignoredPaths }))}
        disabled={!canOperate}
      />
      <div className="flex justify-end gap-2">
        {dirty && (
          <Button
            variant="ghost"
            disabled={busy || !canOperate}
            onClick={() => setDraft(current)}
          >
            {t("services.buildDeployCancel")}
          </Button>
        )}
        <Button
          disabled={busy || !dirty || !canOperate}
          onClick={() => void handleSave()}
        >
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
  disabled = false,
}: {
  title: string;
  hint: string;
  placeholder: string;
  addLabel: string;
  removeLabel: string;
  values: string[];
  onChange: (values: string[]) => void;
  disabled?: boolean;
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
            disabled={disabled}
          />
          <Button
            size="icon"
            variant="ghost"
            aria-label={removeLabel}
            disabled={disabled}
            onClick={() => onChange(values.filter((_, j) => j !== i))}
          >
            <Trash2 />
          </Button>
        </div>
      ))}
      <Button
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => onChange([...values, ""])}
      >
        <Plus /> {addLabel}
      </Button>
    </div>
  );
}
