import { useId, useMemo, useState } from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import type { en } from "@/i18n";
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
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { DeployCard } from "@/features/services/components/deploy-card";
import {
  buildFilterPatternError,
  relativePathError,
  rootDirError,
} from "@/features/services/lib/build-settings-validation";
import { useRootDir } from "@/features/services/hooks/use-root-dir";
import { useBuildCommand } from "@/features/services/hooks/use-build-command";
import { useDockerfilePath } from "@/features/services/hooks/use-dockerfile-path";
import { useDockerContext } from "@/features/services/hooks/use-docker-context";
import {
  useAutoDeploy,
  type AutoDeployTrigger,
} from "@/features/services/hooks/use-auto-deploy";
import { useBuildFilter } from "@/features/services/hooks/use-build-filter";
import { commandText } from "@/features/services/lib/command-text";
import { commandPromptPrefix } from "@/features/services/lib/format";
import { isDockerBuild } from "@/features/services/lib/service-type";
import type { BuildFilterView } from "@/features/services/types";

type TranslationKey = Parameters<Translate>[0];

export interface BuildDeploySectionProps {
  serviceId: string;
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
   * Render's Docker Build Context Directory, relative to rootDir (w8/m128);
   * the server reads an unset context back as ".".
   */
  dockerContext?: string | null;
  /**
   * spec.buildFilter — Render's Build Filters (glob paths/ignoredPaths gating
   * push auto-deploys); null/undefined means no filter (every matching push
   * deploys). Optional so callers that don't select it fall back to no filter.
   */
  buildFilter?: BuildFilterView | null;
  /** spec.autoDeploy — whether a signed git push redeploys this App. */
  autoDeploy: boolean;
  /** Render's Auto-Deploy enum — off | commit | checksPass (w1/m181). */
  autoDeployTrigger?: string | null;
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
  /** Free plan: the pre-deploy field is disabled (w8/m131). */
  freePlan?: boolean;
  /** False for cron/static services, which have no service start command here. */
  showStartCommand?: boolean;
  /** False for cron/static services, whose Docker build settings live elsewhere. */
  showDockerfilePath?: boolean;
  /** spec.buildCommand; the shell command that produces build artifacts. */
  buildCommand?: string | null;
  /** True for static_site — shows the Build Command editor (w7/m41). */
  showBuildCommand?: boolean;
  /**
   * True for static_site: a rebuild publishes a new revision prefix atomically
   * (ADR029) — no instances to drain or replace — so confirmations say so.
   */
  staticSite?: boolean;
  /**
   * Whether to render a separate "Deploy" card (Pre-Deploy, Start/Docker
   * Command, Auto-Deploy, Deploy Hook) after the "Build" card (Render's split,
   * w5/m52). False for a cron_job, whose deploy concerns live in its own Deploy
   * (Schedule/Command) section — there Auto-Deploy folds into the Build card and
   * the Deploy Hook stays a standalone card.
   */
  showDeployCard?: boolean;
  /**
   * A release has gone live (`hasServed`). False while the first deploy failed
   * or is still running: a confirmation then promises no version that keeps
   * serving (w4/548).
   */
  live?: boolean;
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
    case "unavailable":
      return "services.autoDeployUnavailable" as const;
    default:
      return "services.autoDeployDeliveryUnknown" as const;
  }
}

/**
 * The Settings tab's Build and Deploy cards (w5/m13). Ordinary services put
 * source in the unified ServiceSourceCard (every non-static type, cron
 * included, w4/498). Source changes are saved for the
 * next deploy rather than triggering one immediately.
 */
export function BuildDeploySection({
  serviceId,
  rootDir,
  runtime = null,
  builder = null,
  startCommand = null,
  dockerfilePath = null,
  dockerContext = null,
  buildFilter,
  autoDeploy,
  autoDeployTrigger,
  pushDeliveryMethod = null,
  preDeployCommand,
  showPreDeployCommand,
  freePlan,
  showStartCommand = false,
  showDockerfilePath = true,
  buildCommand = null,
  showBuildCommand = false,
  staticSite = false,
  showDeployCard = true,
  live = true,
}: BuildDeploySectionProps) {
  const { t } = useTranslations();
  // Choosing what a service builds and runs (source, branch, root dir, commands,
  // Dockerfile path, pre-deploy) is can_create — a contributor is refused on save
  // (docs/ADR024-members.md). Disable those rows with a reason instead (w9/m84);
  // Auto-Deploy stays editable (it is can_operate, contributor-and-up).
  const capabilities = useCapabilities();
  const { canCreate, canOperate } = capabilities;
  const createDisabled = !canCreate;
  const createReasonKey = capabilities.reasonKey("can_create");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const { setRootDir, busy } = useRootDir();
  const { setBuildCommand, busy: buildCommandBusy } = useBuildCommand();
  const { setDockerfilePath, busy: dockerfilePathBusy } = useDockerfilePath();
  const { setDockerContext, busy: dockerContextBusy } = useDockerContext();
  const { setAutoDeployTrigger, busy: autoDeployBusy } = useAutoDeploy();
  const autoDeployHint = t(autoDeployHintKey(pushDeliveryMethod));
  // A rebuild's confirmation. A static site publishes atomically, with no
  // instances to replace; with nothing live yet, no version keeps serving
  // (w4/548).
  const rebuildBody = (
    keys: Record<"live" | "liveStatic" | "neverLive", TranslationKey>,
  ) =>
    t(
      staticSite
        ? live
          ? keys.liveStatic
          : "services.staticBuildConfirmBodyNeverLive"
        : live
          ? keys.live
          : keys.neverLive,
    );

  // Render presents Auto-Deploy as a select, not a switch (w5/m53), with its
  // three autoDeployTrigger choices: On Commit, After CI Checks Pass (w1/m181)
  // and Off.
  const autoDeployOptions = useMemo(
    () => [
      { value: "commit", label: t("services.autoDeployOnCommit") },
      { value: "checksPass", label: t("services.autoDeployAfterChecks") },
      { value: "off", label: t("services.autoDeployOff") },
    ],
    [t],
  );

  // Mark a draft the server would refuse in its row, before Save and before the
  // rebuild confirmation (w21/045).
  const pathValidate =
    (check: (draft: string) => keyof typeof en | null) => (draft: string) => {
      const reason = check(draft);
      return reason ? t(reason) : null;
    };
  const dockerCommand = isDockerBuild({ runtime, builder });
  const dockerfileBuild = showDockerfilePath && dockerCommand;

  // Auto-Deploy select (Render's disabled-select-with-pencil, w5/m53) — lives in
  // the Deploy card (web/static) or, for a cron_job with no Deploy card, folds
  // into the bottom of the Build card.
  const autoDeployRow = (
    <EditableFieldRow
      label={t("services.autoDeployLabel")}
      hint={autoDeployHint}
      value={autoDeployTrigger ?? (autoDeploy ? "commit" : "off")}
      editLabel={t("services.autoDeployEdit")}
      busy={autoDeployBusy}
      options={autoDeployOptions}
      // Bound to the saved prop, never a local copy: a poll or another tab's
      // save must reach an untouched row, its Edit baseline and Cancel (w4/281).
      // The accepted mutation normalizes Service.autoDeploy into the cache.
      onSave={(value) =>
        setAutoDeployTrigger(serviceId, value as AutoDeployTrigger)
      }
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
          <EditableFieldRow
            label={t("services.buildDeployRootDirLabel")}
            hint={t("services.buildDeployRootDirHint")}
            value={rootDir ?? ""}
            placeholder={t("services.buildDeployRootDirPlaceholder")}
            editLabel={t("services.buildDeployEdit")}
            optional
            mono
            busy={busy}
            validate={pathValidate(rootDirError)}
            confirm={{
              title: (value) =>
                t("services.buildDeployConfirmTitle", { value }),
              body: rebuildBody({
                live: "services.buildDeployConfirmBody",
                liveStatic: "services.buildDeployConfirmBodyStatic",
                neverLive: "services.buildDeployConfirmBodyNeverLive",
              }),
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
              normalize={commandText}
              busy={buildCommandBusy}
              confirm={{
                title: (value) =>
                  t("services.buildCommandConfirmTitle", { value }),
                body: rebuildBody({
                  live: "services.buildCommandConfirmBody",
                  liveStatic: "services.buildCommandConfirmBodyStatic",
                  neverLive: "services.buildCommandConfirmBodyNeverLive",
                }),
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
              validate={pathValidate(relativePathError)}
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

          {/* Render's Docker Build Context Directory sits beside Dockerfile
              Path, relative to the Root Directory (w8/m128). */}
          {dockerfileBuild && (
            <EditableFieldRow
              label={t("services.dockerContextLabel")}
              hint={t("services.dockerContextHint")}
              value={dockerContext ?? ""}
              placeholder={t("services.dockerContextPlaceholder")}
              editLabel={t("services.dockerContextEdit")}
              optional
              mono
              busy={dockerContextBusy}
              validate={pathValidate(relativePathError)}
              confirm={{
                title: (value) =>
                  t("services.dockerContextConfirmTitle", { value }),
                body: t("services.dockerContextConfirmBody"),
                emptyValue: t("services.dockerContextConfirmEmpty"),
              }}
              disabled={createDisabled}
              disabledReason={createReason}
              onSave={(value) => setDockerContext(serviceId, value)}
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
        <DeployCard
          serviceId={serviceId}
          commandKind={dockerCommand ? "docker" : "start"}
          command={startCommand}
          rootDir={rootDir}
          preDeployCommand={preDeployCommand}
          showPreDeployCommand={showPreDeployCommand}
          freePlan={freePlan}
          showCommand={showStartCommand}
          autoDeploy={autoDeployRow}
          live={live}
        />
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
  canOperate,
}: {
  serviceId: string;
  buildFilter: BuildFilterView | null;
  canOperate: boolean;
}) {
  const { t } = useTranslations();
  const { setBuildFilter, busy } = useBuildFilter();
  // SetBuildFilter is RelCanOperate, same class as Auto-Deploy.
  // Caller passes fail-closed canOperate; reason stays role copy when denied.
  const operateReason = canOperate
    ? undefined
    : t("capabilities.reasonCanOperate");
  const current = {
    paths: buildFilter?.paths ?? [],
    ignoredPaths: buildFilter?.ignoredPaths ?? [],
  };
  const [draft, setDraft] = useState(current);
  // The saved lists the draft last synced to. An untouched draft follows a
  // newer saved filter (another tab, the API) instead of offering the old
  // lists as an unsaved change whose Save would roll it back (w4/274); a
  // draft that differs from it is a real edit or a save in flight, and stays.
  const [baseline, setBaseline] = useState(current);
  const currentJson = JSON.stringify(current);
  const baselineJson = JSON.stringify(baseline);
  const draftJson = JSON.stringify(draft);
  if (currentJson !== baselineJson) {
    setBaseline(current);
    if (draftJson === baselineJson) setDraft(current);
  }
  const dirty = draftJson !== currentJson;
  // A pattern the server would refuse holds Save, and is marked in its row.
  const patternError = (value: string) => {
    const reason = buildFilterPatternError(value);
    return reason ? t(reason) : null;
  };
  const invalid = [...draft.paths, ...draft.ignoredPaths].some(
    (value) => buildFilterPatternError(value) !== null,
  );

  async function handleSave() {
    const clean = (xs: string[]) => xs.map((x) => x.trim()).filter(Boolean);
    const submitted = draft;
    const paths = clean(submitted.paths);
    const ignoredPaths = clean(submitted.ignoredPaths);
    const ok = await setBuildFilter(serviceId, paths, ignoredPaths);
    // Adopt the normalized lists, but never over typing done meanwhile.
    if (ok) setDraft((d) => (d === submitted ? { paths, ignoredPaths } : d));
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
        validate={patternError}
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
        validate={patternError}
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
          disabled={busy || !dirty || !canOperate || invalid}
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
  validate,
  disabled = false,
}: {
  title: string;
  hint: string;
  placeholder: string;
  addLabel: string;
  removeLabel: string;
  values: string[];
  onChange: (values: string[]) => void;
  /** An error to mark a row with (null ⇒ valid); the parent holds its own Save. */
  validate?: (value: string) => string | null;
  disabled?: boolean;
}) {
  const idPrefix = useId();
  return (
    <div className="space-y-2">
      <div className="text-sm font-medium">{title}</div>
      <div className="text-xs text-muted-foreground">{hint}</div>
      {values.map((value, i) => {
        const error = validate?.(value) ?? null;
        const errorId = `${idPrefix}-${i}-error`;
        return (
          <div key={i} className="space-y-1">
            <div className="flex items-center gap-2">
              <Input
                value={value}
                onChange={(e) =>
                  onChange(values.map((v, j) => (j === i ? e.target.value : v)))
                }
                placeholder={placeholder}
                className="font-mono text-sm"
                aria-label={title}
                aria-invalid={error !== null || undefined}
                aria-describedby={error !== null ? errorId : undefined}
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
            {error !== null && (
              <p id={errorId} role="alert" className="text-destructive text-xs">
                {error}
              </p>
            )}
          </div>
        );
      })}
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
