import { useState } from "react";
import { ArrowDown, ArrowUp, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import { ConnectedRepoCombobox } from "@/features/services/components/connected-repo-combobox";
import { normalizeSecretRepo } from "@/features/agent-environment/lib/secrets";
import {
  MAX_REPO_ENVIRONMENTS,
  REPO_STEP_LIMITS,
  repoEnvironmentErrorKey,
  repoEnvironmentProblem,
  repoStepProblems,
  trimRepoSteps,
  type RepoEnvironmentView,
  type RepoStep,
  type RepoStepKind,
  type RepoStepProblems,
  type RepoEnvironmentInput,
} from "@/features/agent-environment/lib/repo-environments";

/** What the dialog opens on: a new environment, or an existing one to edit or view. */
export type RepoEnvironmentDialogTarget =
  | { kind: "create" }
  | { kind: "edit" | "view"; environment: RepoEnvironmentView };

/** A row being edited; `id` keeps React's key stable while rows move. */
interface DraftStep extends RepoStep {
  id: number;
}

/** Both lists being edited, and the id the next added row takes. */
interface Draft {
  setup: DraftStep[];
  checks: DraftStep[];
  nextId: number;
}

const BLANK_STEP: RepoStep = { name: "", run: "" };

/** The lists to edit: a stored environment's, or one empty row in each. */
function initialDraft(existing: RepoEnvironmentView | null): Draft {
  let id = 0;
  const rows = (steps: readonly RepoStep[]) =>
    steps.map((step) => ({ name: step.name, run: step.run, id: id++ }));
  const setup = rows(existing ? existing.setup : [BLANK_STEP]);
  const checks = rows(existing ? existing.checks : [BLANK_STEP]);
  return { setup, checks, nextId: id };
}

/** Per list: its heading, hint, add button and row labels. */
const KIND_KEYS: Record<
  RepoStepKind,
  {
    title: string;
    hint: string;
    add: string;
    none: string;
    nameLabel: string;
    runLabel: string;
    moveUp: string;
    moveDown: string;
    remove: string;
    namePlaceholder: string;
    runPlaceholder: string;
  }
> = {
  setup: {
    title: "agentEnvironment.repoEnvs.setupTitle",
    hint: "agentEnvironment.repoEnvs.setupHint",
    add: "agentEnvironment.repoEnvs.addSetup",
    none: "agentEnvironment.repoEnvs.noSetup",
    nameLabel: "agentEnvironment.repoEnvs.setupNameLabel",
    runLabel: "agentEnvironment.repoEnvs.setupRunLabel",
    moveUp: "agentEnvironment.repoEnvs.setupMoveUp",
    moveDown: "agentEnvironment.repoEnvs.setupMoveDown",
    remove: "agentEnvironment.repoEnvs.setupRemove",
    namePlaceholder: "agentEnvironment.repoEnvs.setupNamePlaceholder",
    runPlaceholder: "agentEnvironment.repoEnvs.setupRunPlaceholder",
  },
  checks: {
    title: "agentEnvironment.repoEnvs.checksTitle",
    hint: "agentEnvironment.repoEnvs.checksHint",
    add: "agentEnvironment.repoEnvs.addCheck",
    none: "agentEnvironment.repoEnvs.noChecks",
    nameLabel: "agentEnvironment.repoEnvs.checkNameLabel",
    runLabel: "agentEnvironment.repoEnvs.checkRunLabel",
    moveUp: "agentEnvironment.repoEnvs.checkMoveUp",
    moveDown: "agentEnvironment.repoEnvs.checkMoveDown",
    remove: "agentEnvironment.repoEnvs.checkRemove",
    namePlaceholder: "agentEnvironment.repoEnvs.checkNamePlaceholder",
    runPlaceholder: "agentEnvironment.repoEnvs.checkRunPlaceholder",
  },
};

interface AgentRepoEnvironmentDialogProps {
  target: RepoEnvironmentDialogTarget;
  /** The repositories that already have an environment (owner/name, lowercased). */
  existingRepos: readonly string[];
  onOpenChange: (open: boolean) => void;
  onSave: (repo: string, environment: RepoEnvironmentInput) => Promise<void>;
  saving: boolean;
}

/**
 * Add, edit or view one repository's environment (ADR109 D1): its ordered
 * setup steps and its checks, each a name and a shell command. Saving sends
 * the whole environment, which replaces the stored one. A row left entirely
 * blank is dropped rather than refused, so the empty rows a new environment
 * opens with cost nothing.
 */
export function AgentRepoEnvironmentDialog({
  target,
  existingRepos,
  onOpenChange,
  onSave,
  saving,
}: AgentRepoEnvironmentDialogProps) {
  const { t } = useTranslations();
  const existing = target.kind === "create" ? null : target.environment;
  const readOnly = target.kind === "view";
  const [repo, setRepo] = useState(existing?.repo ?? "");
  const [draft, setDraft] = useState(() => initialDraft(existing));
  const { setup, checks } = draft;
  // Set by the first Save: a blank required field is flagged only after it.
  const [attempted, setAttempted] = useState(false);
  const [errorKey, setErrorKey] = useState<string | null>(null);

  const normalizedRepo = existing ? existing.repo : normalizeSecretRepo(repo);
  const repoProblem = existing
    ? null
    : normalizedRepo == null
      ? "agentEnvironment.repoEnvs.repoInvalid"
      : existingRepos.includes(normalizedRepo)
        ? "agentEnvironment.repoEnvs.repoExists"
        : existingRepos.length >= MAX_REPO_ENVIRONMENTS
          ? "agentEnvironment.repoEnvs.limitExceeded"
          : null;
  const setupProblems = repoStepProblems("setup", setup);
  const checkProblems = repoStepProblems("checks", checks);
  const sentSetup = trimRepoSteps(setup);
  const sentChecks = trimRepoSteps(checks);
  const environmentProblem = repoEnvironmentProblem(sentSetup, sentChecks);
  const rowsValid = [...setupProblems, ...checkProblems].every(
    (p) => p.name == null && p.run == null,
  );
  const valid = repoProblem == null && environmentProblem == null && rowsValid;
  const shownRepoProblem = attempted || repo.trim() !== "" ? repoProblem : null;
  const shownError =
    shownRepoProblem ?? (attempted ? environmentProblem : null) ?? errorKey;

  function edit(kind: RepoStepKind, rows: DraftStep[]) {
    setDraft((d) => ({ ...d, [kind]: rows }));
    setErrorKey(null);
  }

  function add(kind: RepoStepKind) {
    setDraft((d) => ({
      ...d,
      [kind]: [...d[kind], { ...BLANK_STEP, id: d.nextId }],
      nextId: d.nextId + 1,
    }));
    setErrorKey(null);
  }

  async function save() {
    setAttempted(true);
    setErrorKey(null);
    if (!valid || normalizedRepo == null) return;
    try {
      await onSave(normalizedRepo, { setup: sentSetup, checks: sentChecks });
    } catch (err) {
      setErrorKey(repoEnvironmentErrorKey(err));
    }
  }

  const title =
    target.kind === "create"
      ? t("agentEnvironment.repoEnvs.createTitle")
      : readOnly
        ? (existing?.repo ?? "")
        : t("agentEnvironment.repoEnvs.editTitle", {
            repo: existing?.repo ?? "",
          });

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[90vh] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-2xl"
        data-testid="agent-repo-environment-dialog"
      >
        <DialogHeader>
          <DialogTitle className={existing ? "font-mono" : undefined}>
            {title}
          </DialogTitle>
          <DialogDescription>
            {t("agentEnvironment.repoEnvs.dialogDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          id="agent-repo-environment-form"
          className="min-w-0 space-y-6"
          noValidate
          onSubmit={(e) => {
            e.preventDefault();
            if (!readOnly && !saving) void save();
          }}
        >
          {existing ? null : (
            <div className="space-y-1.5">
              <p className="text-sm leading-none font-medium">
                {t("agentEnvironment.repoEnvs.repoLabel")}
              </p>
              <RepoField
                value={repo}
                onChange={(next) => {
                  setRepo(next);
                  setErrorKey(null);
                }}
              />
            </div>
          )}
          {(["setup", "checks"] as const).map((kind) => (
            <StepList
              key={kind}
              kind={kind}
              rows={draft[kind]}
              problems={kind === "setup" ? setupProblems : checkProblems}
              showRequired={attempted}
              readOnly={readOnly}
              onChange={(next) => edit(kind, next)}
              onAdd={() => add(kind)}
            />
          ))}
          {shownError ? (
            <p className="text-destructive text-sm" role="alert">
              {t(shownError)}
            </p>
          ) : null}
        </form>
        <DialogFooter className="gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {readOnly
              ? t("agentEnvironment.repoEnvs.close")
              : t("agentEnvironment.repoEnvs.cancel")}
          </Button>
          {readOnly ? null : (
            <Button
              type="submit"
              form="agent-repo-environment-form"
              disabled={saving}
            >
              {saving ? <Loader2 className="animate-spin" /> : null}
              {t("agentEnvironment.repoEnvs.save")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The repository a new environment is for: the workspace's connected
 * repositories to pick from, or any owner/name typed in — the secrets card's
 * repo-scope picker.
 */
function RepoField({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="max-w-sm">
      <ConnectedRepoCombobox
        value={value}
        onChange={onChange}
        placeholder={t("agentEnvironment.repoEnvs.repoPlaceholder")}
        emptyText={t("agentEnvironment.repoEnvs.repoEmpty")}
        ariaLabel={t("agentEnvironment.repoEnvs.repoLabel")}
        className="font-mono"
      />
    </div>
  );
}

/** One ordered list — setup steps or checks — as editable rows or, read-only, as text. */
function StepList({
  kind,
  rows,
  problems,
  showRequired,
  readOnly,
  onChange,
  onAdd,
}: {
  kind: RepoStepKind;
  rows: DraftStep[];
  problems: RepoStepProblems[];
  showRequired: boolean;
  readOnly: boolean;
  onChange: (rows: DraftStep[]) => void;
  onAdd: () => void;
}) {
  const { t } = useTranslations();
  const keys = KIND_KEYS[kind];
  const headingId = `agent-repo-environment-${kind}`;

  function update(index: number, patch: Partial<RepoStep>) {
    onChange(rows.map((row, i) => (i === index ? { ...row, ...patch } : row)));
  }

  function move(index: number, by: -1 | 1) {
    const next = [...rows];
    [next[index], next[index + by]] = [next[index + by], next[index]];
    onChange(next);
  }

  // A field's problem shows once it has content, or after a Save attempt for
  // a field still blank (required).
  const shown = (problem: string | null, value: string) =>
    problem != null && (showRequired || value.trim() !== "") ? problem : null;

  return (
    <section className="space-y-2" aria-labelledby={headingId}>
      <div className="space-y-0.5">
        <h3 id={headingId} className="text-sm font-medium">
          {t(keys.title)}
        </h3>
        <p className="text-muted-foreground text-xs">{t(keys.hint)}</p>
      </div>
      {readOnly ? (
        rows.length === 0 ? (
          <p className="text-muted-foreground text-sm">{t(keys.none)}</p>
        ) : (
          <ol className="space-y-2" data-testid={`repo-environment-${kind}`}>
            {rows.map((row) => (
              <li key={row.id} className="space-y-1">
                <p className="font-mono text-sm font-medium">{row.name}</p>
                <pre className="bg-muted rounded-md p-2 font-mono text-xs break-all whitespace-pre-wrap">
                  {row.run}
                </pre>
              </li>
            ))}
          </ol>
        )
      ) : (
        <>
          {rows.length > 0 ? (
            <ol className="space-y-2" data-testid={`repo-environment-${kind}`}>
              {rows.map((row, index) => {
                const position = index + 1;
                const nameProblem = shown(problems[index]?.name, row.name);
                const runProblem = shown(problems[index]?.run, row.run);
                return (
                  <li key={row.id} className="space-y-1">
                    <div className="flex flex-col gap-2 sm:flex-row sm:items-start">
                      <Input
                        value={row.name}
                        onChange={(e) =>
                          update(index, { name: e.target.value })
                        }
                        aria-label={t(keys.nameLabel, { position })}
                        aria-invalid={nameProblem != null}
                        placeholder={t(keys.namePlaceholder)}
                        spellCheck={false}
                        autoCapitalize="off"
                        autoComplete="off"
                        className="font-mono sm:w-40 sm:shrink-0"
                      />
                      <Textarea
                        value={row.run}
                        onChange={(e) => update(index, { run: e.target.value })}
                        aria-label={t(keys.runLabel, { position })}
                        aria-invalid={runProblem != null}
                        placeholder={t(keys.runPlaceholder)}
                        rows={1}
                        spellCheck={false}
                        autoCapitalize="off"
                        autoComplete="off"
                        className="min-h-9 min-w-0 flex-1 resize-y py-1.5 font-mono text-xs md:text-xs"
                      />
                      <div className="flex shrink-0 items-center justify-end">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={t(keys.moveUp, { position })}
                          title={t(keys.moveUp, { position })}
                          disabled={index === 0}
                          onClick={() => move(index, -1)}
                        >
                          <ArrowUp className="size-4" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={t(keys.moveDown, { position })}
                          title={t(keys.moveDown, { position })}
                          disabled={index === rows.length - 1}
                          onClick={() => move(index, 1)}
                        >
                          <ArrowDown className="size-4" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={t(keys.remove, { position })}
                          title={t(keys.remove, { position })}
                          onClick={() =>
                            onChange(rows.filter((_, i) => i !== index))
                          }
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </div>
                    </div>
                    {nameProblem || runProblem ? (
                      <p className="text-destructive text-xs" role="alert">
                        {[nameProblem, runProblem]
                          .filter((p): p is string => p != null)
                          .map((p) => t(p))
                          .join(" ")}
                      </p>
                    ) : null}
                  </li>
                );
              })}
            </ol>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={rows.length >= REPO_STEP_LIMITS[kind].maxSteps}
            onClick={onAdd}
          >
            <Plus />
            {t(keys.add)}
          </Button>
        </>
      )}
    </section>
  );
}
