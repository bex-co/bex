import {
  useId,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type FormEvent,
} from "react";
import { Link } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Combobox } from "@/common/components/ui/combobox";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentAutomationInput } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useRepos } from "@/features/services/hooks/use-repos";
import { ComposerSecrets } from "@/features/agent-sessions/components/composer-secrets";
import {
  ComposerSkillChips,
  ComposerSkills,
} from "@/features/agent-sessions/components/composer-skills";
import { selectableAgents } from "@/features/agent-sessions/lib/composer";
import { ModelPicker } from "@/features/agent-sessions/components/model-picker";
import {
  modelUnoffered,
  shownModel,
} from "@/features/agent-sessions/lib/models";
import { SandboxPicker } from "@/features/agent-sessions/components/sandbox-picker";
import { GithubCliPicker } from "@/features/agent-sessions/components/github-cli-picker";
import {
  githubCliOnLock,
  inheritedGithubCli,
} from "@/features/agent-sessions/lib/github-cli";
import { useAgentSessionCapabilities } from "@/features/agent-sessions/hooks/use-agent-session-capabilities";
import { useAgentEnvironment } from "@/features/agent-environment/hooks/use-agent-environment";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  FormField,
  FormSection,
} from "@/features/agent-automations/components/form-field";
import { describedBy } from "@/features/agent-automations/lib/aria";
import {
  EMAIL_NOTIFICATIONS,
  emailNotificationLabel,
  isEmailNotification,
} from "@/features/agent-automations/lib/display";
import { SchedulePicker } from "@/features/agent-automations/components/schedule-picker";
import {
  hasProblems,
  mapAutomationError,
  toAutomationInput,
  validateForm,
  type AutomationFormValues,
  type FormField as FormFieldName,
  type MappedError,
} from "@/features/agent-automations/lib/form";
import {
  browserTimeZone,
  visibleScheduleFields,
  type ScheduleProblems,
} from "@/features/agent-automations/lib/schedule";

interface AutomationFormProps {
  mode: "create" | "edit";
  initial: AutomationFormValues;
  /** The stored version on an edit, sent as ifVersion. */
  version?: number;
  /** Saves; rejects with the mutation's error, which the form places. */
  onSave: (input: AgentAutomationInput) => Promise<void>;
  cancelTo:
    | { to: "/automations" }
    | { to: "/automations/$automationId"; id: string };
}

/**
 * The create and edit form (ADR101 D9): name, the schedule picker, then the
 * session each fire starts with the same task, repository, agent, model,
 * sandbox, GitHub CLI, spend-limit, secrets and skills controls the /agents
 * composer has (the sandbox may also follow the workspace default at each
 * run, w10/m519; the GitHub CLI binding inherits the workspace's automations
 * default, w10/m561), and which runs email the creator. Shape errors are
 * caught here and shown beside their field; the server's named refusals (a
 * schedule too frequent, a missing spend limit, a taken name, a bad email
 * setting, an unknown sandbox template) are shown beside the field they name,
 * with the server's own wording, and a gh binding refused as on with the
 * form's own.
 */
const subscribeNever = () => () => {};

export function AutomationForm({
  mode,
  initial,
  version,
  onSave,
  cancelTo,
}: AutomationFormProps) {
  const { t } = useTranslations();
  const base = useId();
  const id = (name: string) => `${base}-${name}`;
  const formRef = useRef<HTMLFormElement>(null);
  const { currentWorkspaceId } = useWorkspace();
  const { capabilities } = useAgentSessionCapabilities();
  // The workspace's gh chain (w10/m561): what inherit resolves to, and
  // whether "on" is locked by a mandatory off or the creator's role. An
  // automation that already stores on keeps it, as the server does.
  const { environment } = useAgentEnvironment(currentWorkspaceId ?? null);
  const { canManage } = useCapabilities();
  const githubCliLock = githubCliOnLock({
    env: environment,
    canManage,
    kept: initial.githubCli === "on",
  });
  const { repos } = useRepos();
  const [chosen, setValues] = useState<AutomationFormValues>(initial);
  // A create form leaves the zone "" until the person picks one; until then it
  // reads "UTC" on the server and during hydration, and the browser's zone
  // after, so the first client render equals the server render.
  const browserZone = useSyncExternalStore(
    subscribeNever,
    browserTimeZone,
    () => "UTC",
  );
  const values = useMemo(
    () =>
      chosen.schedule.timezone === ""
        ? { ...chosen, schedule: { ...chosen.schedule, timezone: browserZone } }
        : chosen,
    [chosen, browserZone],
  );
  const [attempted, setAttempted] = useState(false);

  const [saving, setSaving] = useState(false);
  const [refusal, setRefusal] = useState<MappedError | null>(null);

  const repoOptions = useMemo(
    () => repos.map((r) => ({ value: r.fullName, label: r.fullName })),
    [repos],
  );
  const problems = useMemo(() => validateForm(values), [values]);
  const shown = attempted ? problems : { fields: {}, schedule: {} };

  function change(patch: Partial<AutomationFormValues>) {
    setValues((prev) => ({ ...prev, ...patch }));
    setRefusal(null);
  }

  const fieldError = (field: FormFieldName): string | null => {
    if (refusal?.target === field) {
      return refusal.messageKey ? t(refusal.messageKey) : refusal.message;
    }
    const key = shown.fields[field];
    return key ? t(key) : null;
  };
  const serverSchedule: Partial<Record<keyof ScheduleProblems, string>> = {};
  const namedField =
    refusal?.target === "schedule" &&
    refusal.scheduleField &&
    visibleScheduleFields(values.schedule.frequency).includes(
      refusal.scheduleField,
    )
      ? refusal.scheduleField
      : null;
  if (refusal && namedField) {
    serverSchedule[namedField] = refusal.message;
  }
  const scheduleWideError =
    refusal?.target === "schedule" && !namedField ? refusal.message : null;

  async function submit(event: FormEvent) {
    event.preventDefault();
    setRefusal(null);
    setAttempted(true);
    if (hasProblems(problems)) {
      // Move focus to the first invalid control so a keyboard or screen-reader
      // user lands on what to fix.
      queueMicrotask(() =>
        formRef.current
          ?.querySelector<HTMLElement>('[aria-invalid="true"]')
          ?.focus(),
      );
      return;
    }
    setSaving(true);
    try {
      await onSave(toAutomationInput(values, { ifVersion: version }));
    } catch (err) {
      const mapped = mapAutomationError(err, t("agentAutomations.saveFailed"));
      setRefusal(mapped);
      if (mapped.target === "general") {
        queueMicrotask(() =>
          formRef.current
            ?.querySelector<HTMLElement>("[data-form-error]")
            ?.focus(),
        );
      }
    } finally {
      setSaving(false);
    }
  }

  const nameError = fieldError("name");
  const taskError = fieldError("task");
  const repoError = fieldError("repo");
  const spendError = fieldError("spendLimit");
  const modelError = fieldError("model");
  // The agent's model offer (w10/m516): the platform catalog to pick from, a
  // fixed model, or none (the workspace's own key: any model id).
  const modelOffer = capabilities?.models[values.agent];
  // A saved model an admin turned off or the catalog dropped (w10/m527)
  // stays the value, with a note that runs use the workspace default.
  const unofferedModel =
    modelOffer && modelUnoffered(modelOffer, values.model.trim())
      ? { model: values.model.trim(), fallback: shownModel(modelOffer, "") }
      : null;
  const modelDescribedBy = describedBy(id("model"), {
    hasError: modelError != null,
    hasHelp: true,
  });
  const templateError = fieldError("template");
  // The workspace's sandbox templates (w10/m519).
  const sandboxOffer = capabilities?.sandbox;
  const templateDescribedBy = describedBy(id("template"), {
    hasError: templateError != null,
    hasHelp: true,
  });
  const githubCliError = fieldError("githubCli");
  const emailError = fieldError("emailNotifications");
  const skillsError = fieldError("skills");

  return (
    <form
      ref={formRef}
      onSubmit={(e) => void submit(e)}
      noValidate
      className="space-y-8"
      aria-label={t(
        mode === "create"
          ? "agentAutomations.createTitle"
          : "agentAutomations.editTitle",
      )}
    >
      {refusal?.target === "general" ? (
        <Alert variant="destructive" data-form-error tabIndex={-1} role="alert">
          <AlertDescription>{refusal.message}</AlertDescription>
        </Alert>
      ) : null}

      <FormSection title={t("agentAutomations.section.details")}>
        <FormField
          id={id("name")}
          label={t("agentAutomations.field.name")}
          help={t("agentAutomations.help.name")}
          error={nameError}
        >
          <Input
            id={id("name")}
            value={values.name}
            onChange={(e) => change({ name: e.target.value })}
            disabled={saving}
            spellCheck={false}
            autoCapitalize="off"
            autoComplete="off"
            placeholder={t("agentAutomations.placeholder.name")}
            aria-invalid={nameError != null}
            aria-describedby={describedBy(id("name"), {
              hasError: nameError != null,
              hasHelp: true,
            })}
          />
        </FormField>
      </FormSection>

      <FormSection
        title={t("agentAutomations.section.schedule")}
        description={t("agentAutomations.section.scheduleDescription")}
      >
        <SchedulePicker
          value={values.schedule}
          onChange={(schedule) => change({ schedule })}
          problems={shown.schedule}
          serverErrors={serverSchedule}
          serverScheduleError={scheduleWideError}
          disabled={saving}
        />
      </FormSection>

      <FormSection
        title={t("agentAutomations.section.session")}
        description={t("agentAutomations.section.sessionDescription")}
      >
        <FormField
          id={id("task")}
          label={t("agentAutomations.field.task")}
          help={t("agentAutomations.help.task")}
          error={taskError}
        >
          <Textarea
            id={id("task")}
            value={values.task}
            onChange={(e) => change({ task: e.target.value })}
            disabled={saving}
            className="min-h-40"
            placeholder={t("agentSessions.taskPlaceholder")}
            aria-invalid={taskError != null}
            aria-describedby={describedBy(id("task"), {
              hasError: taskError != null,
              hasHelp: true,
            })}
          />
        </FormField>

        <FormField
          id={id("repo")}
          label={t("agentAutomations.field.repo")}
          help={t("agentAutomations.help.repo")}
          error={repoError}
        >
          <Combobox
            options={repoOptions}
            value={values.repo}
            onValueChange={(repo) => change({ repo })}
            disabled={saving}
            ariaLabel={t("agentAutomations.field.repo")}
            placeholder={t("agentAutomations.placeholder.repo")}
          />
        </FormField>

        <div className="grid gap-4 sm:grid-cols-2">
          <FormField id={id("agent")} label={t("agentAutomations.field.agent")}>
            <Select
              value={values.agent}
              onValueChange={(agent) =>
                change({
                  agent: agent as AutomationFormValues["agent"],
                  model: "",
                })
              }
              disabled={saving}
            >
              <SelectTrigger id={id("agent")} className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {selectableAgents(capabilities?.agents, values.agent).map(
                  (agent) => (
                    <SelectItem key={agent} value={agent}>
                      {t(`agentSessions.agent.${agent}`)}
                    </SelectItem>
                  ),
                )}
              </SelectContent>
            </Select>
          </FormField>

          <FormField
            id={id("model")}
            label={t("agentAutomations.field.model")}
            help={
              unofferedModel ? (
                <span className="text-amber-700 dark:text-amber-400">
                  {t("agentAutomations.help.modelUnoffered", unofferedModel)}
                </span>
              ) : (
                t("agentAutomations.help.model")
              )
            }
            error={modelError}
          >
            {modelOffer && modelOffer.models.length > 0 ? (
              <ModelPicker
                id={id("model")}
                variant="field"
                offer={modelOffer}
                value={values.model}
                onChange={(model) => change({ model })}
                disabled={saving}
                invalid={modelError != null}
                describedBy={modelDescribedBy}
              />
            ) : (
              <Input
                id={id("model")}
                value={values.model}
                onChange={(e) => change({ model: e.target.value })}
                disabled={saving}
                placeholder={t("agentAutomations.placeholder.model")}
                spellCheck={false}
                autoCapitalize="off"
                autoComplete="off"
                className="font-mono"
                aria-invalid={modelError != null}
                aria-describedby={modelDescribedBy}
              />
            )}
          </FormField>
        </div>

        <FormField
          id={id("template")}
          label={t("agentAutomations.field.template")}
          help={t("agentAutomations.help.template")}
          error={templateError}
        >
          {sandboxOffer && sandboxOffer.templates.length > 0 ? (
            <SandboxPicker
              id={id("template")}
              variant="field"
              defaultChoice="follow"
              offer={sandboxOffer}
              value={values.template}
              onChange={(template) => change({ template })}
              disabled={saving}
              invalid={templateError != null}
              describedBy={templateDescribedBy}
            />
          ) : (
            // No list to pick from yet (or none served): the stored choice,
            // read-only, holds the field's place.
            <Input
              id={id("template")}
              value={values.template}
              readOnly
              disabled
              placeholder={t("agentAutomations.placeholder.template")}
              className="font-mono"
              aria-invalid={templateError != null}
              aria-describedby={templateDescribedBy}
            />
          )}
        </FormField>

        <FormField
          id={id("github-cli")}
          label={t("agentEnvironment.githubCliLabel")}
          help={t("agentAutomations.help.githubCli")}
          error={githubCliError}
        >
          <GithubCliPicker
            id={id("github-cli")}
            variant="field"
            value={values.githubCli}
            onChange={(githubCli) => change({ githubCli })}
            inherited={inheritedGithubCli(environment, "automation")}
            onLock={githubCliLock}
            disabled={saving}
            invalid={githubCliError != null}
            describedBy={describedBy(id("github-cli"), {
              hasError: githubCliError != null,
              hasHelp: true,
            })}
          />
        </FormField>

        <div className="grid gap-4 sm:grid-cols-2">
          <FormField
            id={id("spend")}
            label={t("agentAutomations.field.spendLimit")}
            help={t("agentAutomations.help.spendLimit")}
            error={spendError}
          >
            <Input
              id={id("spend")}
              type="text"
              inputMode="decimal"
              value={values.spendLimit}
              onChange={(e) => change({ spendLimit: e.target.value })}
              disabled={saving}
              placeholder={t("agentSessions.spendLimitPlaceholder")}
              aria-invalid={spendError != null}
              aria-describedby={describedBy(id("spend"), {
                hasError: spendError != null,
                hasHelp: true,
              })}
            />
          </FormField>

          <FormField
            id={id("email")}
            label={t("agentAutomations.field.emailNotifications")}
            help={t("agentAutomations.help.emailNotifications")}
            error={emailError}
          >
            <Select
              value={values.emailNotifications}
              onValueChange={(email) => {
                if (isEmailNotification(email)) {
                  change({ emailNotifications: email });
                }
              }}
              disabled={saving}
            >
              <SelectTrigger
                id={id("email")}
                className="w-full"
                aria-invalid={emailError != null}
                aria-describedby={describedBy(id("email"), {
                  hasError: emailError != null,
                  hasHelp: true,
                })}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {EMAIL_NOTIFICATIONS.map((email) => (
                  <SelectItem key={email} value={email}>
                    {emailNotificationLabel(email, t)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormField>
        </div>

        <div className="space-y-1.5">
          <Label id={id("secrets-label")}>
            {t("agentAutomations.field.secrets")}
          </Label>
          <div
            className="flex flex-wrap items-center gap-2"
            role="group"
            aria-labelledby={id("secrets-label")}
          >
            <ComposerSecrets
              ownerId={currentWorkspaceId ?? null}
              value={{ secretNames: values.secretNames, secrets: [] }}
              onChange={(next) => change({ secretNames: next.secretNames })}
              repo={values.repo || null}
              disabled={saving}
              workspaceOnly
            />
            {values.secretNames.length === 0 ? (
              <span className="text-muted-foreground text-sm">
                {t("agentAutomations.noSecrets")}
              </span>
            ) : (
              values.secretNames.map((name) => (
                <Badge key={name} variant="secondary" className="font-mono">
                  {name}
                </Badge>
              ))
            )}
          </div>
          <p className="text-muted-foreground text-xs">
            {t("agentAutomations.help.secrets")}
          </p>
        </div>

        <div className="space-y-1.5">
          <Label id={id("skills-label")}>
            {t("agentAutomations.field.skills")}
          </Label>
          <div
            className="flex flex-wrap items-center gap-2"
            role="group"
            aria-labelledby={id("skills-label")}
          >
            <ComposerSkills
              ownerId={currentWorkspaceId ?? null}
              value={values.skills}
              onChange={(skills) => change({ skills })}
              disabled={saving}
            />
            {values.skills.length === 0 ? (
              <span className="text-muted-foreground text-sm">
                {t("agentAutomations.noSkills")}
              </span>
            ) : null}
          </div>
          {values.skills.length > 0 ? (
            <div className="-mx-2.5 -mt-2">
              <ComposerSkillChips
                value={values.skills}
                onChange={(skills) => change({ skills })}
                disabled={saving}
              />
            </div>
          ) : null}
          {skillsError ? (
            <p role="alert" className="text-destructive text-sm">
              {skillsError}
            </p>
          ) : null}
        </div>
      </FormSection>

      <div className="flex flex-wrap justify-end gap-2 border-t pt-4">
        <Button type="button" variant="outline" asChild disabled={saving}>
          {cancelTo.to === "/automations" ? (
            <Link to="/automations">{t("common.cancel")}</Link>
          ) : (
            <Link
              to="/automations/$automationId"
              params={{ automationId: cancelTo.id }}
            >
              {t("common.cancel")}
            </Link>
          )}
        </Button>
        <Button type="submit" disabled={saving}>
          {saving ? <Loader2 className="animate-spin" /> : null}
          {t(
            mode === "create"
              ? "agentAutomations.createSubmit"
              : "agentAutomations.editSubmit",
          )}
        </Button>
      </div>
    </form>
  );
}
