import { useCallback, useId, useMemo, useRef, useState } from "react";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { useForm, useWatch } from "react-hook-form";
import { Link, useNavigate } from "@tanstack/react-router";
import { AtSign, Loader2 } from "lucide-react";
import { GithubIcon } from "@/common/components/github-icon";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { cn } from "@/common/lib/utils/utils";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
} from "@/common/components/ui/form";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useRepos } from "@/features/services/hooks/use-repos";
import { useConnectGitAccount } from "@/features/git/hooks/use-connect-git";
import { useOnGitConnectionsChange } from "@/features/git/hooks/use-on-git-connections-change";
import { useRefetchOnFocus } from "@/common/hooks/use-refetch-on-focus";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { useAgentSessionCapabilities } from "@/features/agent-sessions/hooks/use-agent-session-capabilities";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { useAttachmentUploads } from "@/features/agent-sessions/hooks/use-attachment-uploads";
import { useIdempotencyKey } from "@/features/agent-sessions/hooks/use-idempotency-key";
import {
  AttachButton,
  AttachmentChips,
  DropOverlay,
} from "@/features/agent-sessions/components/attachment-chips";
import { useAttachmentDropTarget } from "@/features/agent-sessions/hooks/use-attachment-drop-target";
import {
  AgentSessionError,
  agentSessionAvailabilityCopy,
  agentSessionErrorMessage,
  toAgentSessionError,
} from "@/features/agent-sessions/lib/errors";
import {
  InlineMentionEditor,
  type InlineMentionEditorHandle,
} from "@/features/agent-sessions/components/lazy-mention-editor";
import {
  deriveBranch,
  selectableAgents,
  type AgentOption,
  type ComposerValues,
} from "@/features/agent-sessions/lib/composer";
import type { ComposerDocument } from "@/features/agent-sessions/lib/composer-document";
import { parseSpendLimit } from "@/features/agent-sessions/lib/spend-limit";
import { ComposerSpendBanner } from "@/features/agent-sessions/components/composer-spend-banner";
import { useComposerSpendGate } from "@/features/agent-sessions/hooks/use-composer-spend-gate";
import { blocksCreate } from "@/features/agent-environment/lib/spend";
import { ComposerSecrets } from "@/features/agent-sessions/components/composer-secrets";
import { ModelPicker } from "@/features/agent-sessions/components/model-picker";
import { SandboxPicker } from "@/features/agent-sessions/components/sandbox-picker";
import {
  ComposerSkillChips,
  ComposerSkills,
} from "@/features/agent-sessions/components/composer-skills";
import {
  composerSkillsFingerprint,
  composerSkillsInput,
  composerSkillsValid,
  type ComposerSkill,
} from "@/features/agent-sessions/lib/composer-skills";
import {
  EMPTY_COMPOSER_SECRETS,
  type ComposerSecretsValue,
} from "@/features/agent-sessions/lib/composer-secrets";

const EXAMPLE_KEYS = [
  "agentSessions.exampleFixTests",
  "agentSessions.exampleAddReadme",
] as const;

/**
 * The prompt-box composer: the visual center of `/agents`. Agent, model,
 * sandbox and repo live on the toolbar (w10/m17, w10/m516, w10/m519): the
 * branch is always derived from the task, the sandbox network comes from the
 * workspace's agent environment, the model control shows the agent's model
 * and its catalog, and the sandbox control the workspace's templates with the
 * default preselected; each sends its choice only when the person picks
 * something other than the default. A repo is optional — a repo-less prompt
 * starts a chat-only session. The Connect GitHub callout appears only when the
 * composer targets a repo but the workspace has no GitHub App repos connected.
 */
export function NewSessionComposer() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const { create } = useAgentSessionMutations();
  const { repos, loading: reposLoading, refetch: refetchRepos } = useRepos();
  const { sessions } = useAgentSessions({ poll: false });
  const {
    capabilities,
    loading: capabilitiesLoading,
    error: capabilitiesError,
    refetch: refetchCapabilities,
  } = useAgentSessionCapabilities();
  // GitHub connected elsewhere (the callout's new tab): re-read readiness and
  // repos so the callout goes away and the repo becomes usable.
  const refreshGitHub = useCallback(() => {
    void refetchCapabilities();
    void refetchRepos();
  }, [refetchCapabilities, refetchRepos]);

  const attachmentLimits = capabilities?.attachments ?? null;
  const uploads = useAttachmentUploads(attachmentLimits);
  const attachmentsBlock = uploads.uploading || uploads.hasErrors;
  const [composerDocument, setComposerDocument] = useState<ComposerDocument>({
    task: "",
    repo: null,
    sessionIds: [],
  });
  const [availabilityError, setAvailabilityError] = useState<unknown>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  // The optional per-session model spend ceiling (ADR094 D8), kept as typed.
  const [spendLimitText, setSpendLimitText] = useState("");
  // The composer's message line: the ceiling's error is announced and
  // described on the field and the disabled send button (w4/524).
  const messageId = useId();
  const spendLimit = parseSpendLimit(spendLimitText);
  // Session secrets (ADR098): workspace refs and inline values, sent once.
  const [secrets, setSecrets] = useState<ComposerSecretsValue>(
    EMPTY_COMPOSER_SECRETS,
  );
  // Workspace skills this first turn must follow (ADR099 D4), sent once.
  const [skills, setSkills] = useState<ComposerSkill[]>([]);
  // The model the session runs (w10/m516); "" runs the agent's default.
  const [model, setModel] = useState("");
  // The sandbox template it runs on (w10/m519); "" runs the workspace default.
  const [template, setTemplate] = useState("");
  const skillsInvalid = !composerSkillsValid(skills);
  const editorRef = useRef<InlineMentionEditorHandle | null>(null);
  const { task, repo, sessionIds } = composerDocument;
  const noRepos = !reposLoading && repos.length === 0;
  // Repo-backed create needs GitHub; chat-only sessions do not.
  const githubMissing = capabilities ? !capabilities.github.connected : noRepos;
  // Sticky once the person found the repository list empty: clicking the
  // callout moves focus out of the editor, which closes that list.
  const [reachedForRepo, setReachedForRepo] = useState(false);
  const onReposEmpty = useCallback(() => setReachedForRepo(true), []);
  const showGitHubCallout = githubMissing && (Boolean(repo) || reachedForRepo);
  const showExamples = !noRepos && sessions.length === 0;
  const platformUnavailable = capabilities?.enabled === false;
  const form = useForm<ComposerValues>({
    defaultValues: { agent: "claude" },
  });
  const {
    formState: { isSubmitting },
  } = form;
  // Readiness is per agent: the platform route serves claude in every
  // workspace, while codex and gemini still need the workspace's own key.
  const agent = useWatch({ control: form.control, name: "agent" });
  const modelOffer = capabilities?.models[agent];
  const modelKeyMissing =
    capabilities?.enabled && !capabilities.modelKeyReadyAgents.includes(agent);
  // ADR091 D6: a platform-route workspace at its spend cap (or with an unpaid
  // failed invoice) cannot start a session the model mint would refuse.
  const spendGate = useComposerSpendGate();
  const spendBlocked = blocksCreate(spendGate);
  const preflightError = useMemo(
    () =>
      platformUnavailable
        ? new AgentSessionError(
            "AGENT_SESSION_NOT_CONFIGURED",
            "agent sessions are not configured",
            {},
          )
        : capabilitiesError
          ? toAgentSessionError(capabilitiesError)
          : null,
    [capabilitiesError, platformUnavailable],
  );
  const availabilityCopy = useMemo(
    () => agentSessionAvailabilityCopy(availabilityError ?? preflightError),
    [availabilityError, preflightError],
  );

  // No new files while a create is in flight: they would miss the send.
  const drop = useAttachmentDropTarget(
    uploads.add,
    Boolean(attachmentLimits?.enabled) && !isSubmitting,
  );

  const source = useMemo(() => ({ repos, sessions }), [repos, sessions]);

  const submitKey = useIdempotencyKey();

  const submit = async (values: ComposerValues) => {
    setAvailabilityError(null);
    setSubmitError(null);

    // The button carries the same gate, but Enter can submit from the editor.
    // Re-check here so no interaction can race a known capability gap.
    if (
      capabilitiesLoading ||
      platformUnavailable ||
      modelKeyMissing ||
      spendBlocked ||
      (repo && githubMissing) ||
      attachmentsBlock ||
      skillsInvalid ||
      !spendLimit.ok
    ) {
      return;
    }

    // Repo-less (chat-only) sessions are allowed: with no repo the agent just
    // runs the prompt in an empty sandbox and delivers no PR. A branch is only
    // meaningful when a repo is chosen, and then it is always derived.
    const branch = repo ? deriveBranch(task) : "";

    const prompt = [
      task.trim(),
      ...sessionIds.map((id) => `Context: agent session ${id}`),
    ].join("\n\n");

    const sentAttachmentIds = uploads.readyIds;
    // One key per distinct submit: a retry of the same content after a failure
    // reuses it (the server hands back the session it already created, `created:
    // false`, which is success here), edited content gets a fresh one.
    const idempotencyKey = submitKey.keyFor(
      JSON.stringify([
        currentWorkspaceId,
        repo ?? "",
        branch,
        values.agent,
        model,
        template,
        prompt,
        sentAttachmentIds,
        spendLimit.value,
        // Values too, though the server's digest covers names only (ADR098 D2):
        // a corrected value is a new request. The fingerprint never leaves memory.
        [...secrets.secretNames].sort(),
        [...secrets.secrets].sort((a, b) => a.name.localeCompare(b.name)),
        composerSkillsFingerprint(skills),
      ]),
    );
    try {
      const ticket = await create({
        idempotencyKey,
        ownerId: currentWorkspaceId,
        repo: repo ?? "",
        branch,
        agent: values.agent,
        ...(model ? { model } : {}),
        ...(template ? { template } : {}),
        task: prompt,
        attachmentIds: sentAttachmentIds,
        maxSpendUSD: spendLimit.value,
        secretNames: secrets.secretNames,
        secrets: secrets.secrets,
        skills: composerSkillsInput(skills),
      });
      uploads.clear(sentAttachmentIds);
      setSecrets(EMPTY_COMPOSER_SECRETS);
      setSkills([]);
      submitKey.release();
      await navigate({
        to: "/agents/$agentSessionId",
        params: { agentSessionId: ticket.session.id },
      });
    } catch (err) {
      handleCreateError(err);
    }
  };
  // handleSubmit runs per submit event, not during render, because the
  // callback mints the submit key.
  const onSubmit = (event?: React.BaseSyntheticEvent) =>
    form.handleSubmit(submit)(event);

  function handleCreateError(err: unknown) {
    // The user closed the ADR075 D7 payment dialog — their own choice, not an
    // error to report.
    if (isPaymentOnboardingCancelled(err)) return;
    if (!(err instanceof AgentSessionError)) {
      setSubmitError(agentSessionErrorMessage(err, t));
      return;
    }
    if (agentSessionAvailabilityCopy(err)) {
      setAvailabilityError(err);
      return;
    }
    setSubmitError(
      t(err.messageKey, { ...err.params, defaultValue: err.message }),
    );
  }

  function openMention() {
    editorRef.current?.openMention();
  }

  function insertExample(text: string) {
    editorRef.current?.insertPrompt(text);
    openMention();
  }

  return (
    <div className="space-y-3">
      {availabilityCopy ? (
        <Alert
          variant={availabilityCopy.destructive ? "destructive" : "default"}
        >
          <AlertTitle>{t(availabilityCopy.titleKey as never)}</AlertTitle>
          <AlertDescription>
            {t(availabilityCopy.bodyKey as never)}
          </AlertDescription>
        </Alert>
      ) : null}
      {modelKeyMissing ? (
        <Alert>
          <AlertTitle>{t("agentSessions.modelKeyMissingTitle")}</AlertTitle>
          <AlertDescription>
            <p>{t("agentSessions.modelKeyMissingBody")}</p>
            <Link
              to="/workspace/settings"
              hash="agents"
              className="font-medium underline underline-offset-4"
            >
              {t("agentSessions.modelKeyMissingAction")}
            </Link>
          </AlertDescription>
        </Alert>
      ) : null}
      <ComposerSpendBanner gate={spendGate} />
      {submitError ? (
        <Alert variant="destructive">
          <AlertTitle>{t("agentSessions.createErrorTitle")}</AlertTitle>
          <AlertDescription>{submitError}</AlertDescription>
        </Alert>
      ) : null}

      {showGitHubCallout ? (
        <GitHubEmptyCallout onConnected={refreshGitHub} />
      ) : null}

      <Form {...form}>
        <form onSubmit={onSubmit}>
          <div
            {...drop.handlers}
            className={cn(
              "bg-background focus-within:border-ring relative rounded-xl border shadow-sm",
              drop.dragging && "border-ring ring-ring/40 ring-[3px]",
            )}
          >
            <DropOverlay visible={drop.dragging} />
            <AttachmentChips uploads={uploads} limits={attachmentLimits} />
            <ComposerSkillChips
              value={skills}
              onChange={setSkills}
              disabled={isSubmitting}
            />
            <InlineMentionEditor
              ref={editorRef}
              source={source}
              reposLoading={reposLoading}
              ariaLabel={t("agentSessions.taskLabel")}
              placeholder={t("agentSessions.taskPlaceholder")}
              onReposEmpty={onReposEmpty}
              onChange={(nextDocument) => {
                setComposerDocument(nextDocument);
              }}
              onSubmit={() => {
                if (
                  isSubmitting ||
                  attachmentsBlock ||
                  task.trim().length === 0
                )
                  return;
                void onSubmit();
              }}
            />

            <div className="flex flex-wrap items-center gap-1 px-1.5 pb-1.5">
              <FormField
                name="agent"
                render={({ field }) => (
                  <FormItem className="space-y-0">
                    <FormControl>
                      <Select
                        value={field.value}
                        onValueChange={(v) => {
                          field.onChange(v as AgentOption);
                          setModel("");
                        }}
                      >
                        <SelectTrigger
                          size="sm"
                          className="h-8 w-[7.5rem] border-0 bg-transparent shadow-none"
                          aria-label={t("agentSessions.agentLabel")}
                        >
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {selectableAgents(
                            capabilities?.agents,
                            field.value,
                          ).map((agent) => (
                            <SelectItem key={agent} value={agent}>
                              {t(`agentSessions.agent.${agent}`)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormControl>
                  </FormItem>
                )}
              />

              <ModelPicker
                offer={modelOffer}
                value={model}
                onChange={setModel}
                disabled={isSubmitting}
              />

              <SandboxPicker
                offer={capabilities?.sandbox}
                value={template}
                onChange={setTemplate}
                disabled={isSubmitting}
              />

              {attachmentLimits?.enabled ? (
                <AttachButton onFiles={uploads.add} disabled={isSubmitting} />
              ) : null}

              <ComposerSecrets
                ownerId={currentWorkspaceId}
                value={secrets}
                onChange={setSecrets}
                repo={repo}
                disabled={isSubmitting}
              />

              <ComposerSkills
                ownerId={currentWorkspaceId}
                value={skills}
                onChange={setSkills}
                disabled={isSubmitting}
              />

              <Input
                type="text"
                inputMode="decimal"
                value={spendLimitText}
                onChange={(e) => setSpendLimitText(e.target.value)}
                disabled={isSubmitting}
                placeholder={t("agentSessions.spendLimitPlaceholder")}
                aria-label={t("agentSessions.spendLimitLabel")}
                title={t("agentSessions.spendLimitHint")}
                aria-invalid={!spendLimit.ok}
                aria-describedby={messageId}
                data-testid="agent-composer-spend-limit"
                className="h-8 w-28 border-0 bg-transparent px-2 text-xs shadow-none"
              />

              <Button
                type="button"
                variant={repo ? "secondary" : "ghost"}
                size="sm"
                className="h-8 max-w-56 gap-1.5 px-2 text-xs font-normal"
                aria-label={
                  repo
                    ? t("agentSessions.repoChip", { repo })
                    : t("agentSessions.mentionButton")
                }
                onClick={openMention}
              >
                <AtSign className="size-3.5" />
                <span className="truncate">
                  {repo ?? t("agentSessions.mentionButton")}
                </span>
              </Button>

              <Button
                type="submit"
                size="sm"
                className="ml-auto h-8"
                aria-describedby={spendLimit.ok ? undefined : messageId}
                disabled={
                  isSubmitting ||
                  capabilitiesLoading ||
                  platformUnavailable ||
                  modelKeyMissing ||
                  spendBlocked ||
                  (Boolean(repo) && githubMissing) ||
                  attachmentsBlock ||
                  skillsInvalid ||
                  !spendLimit.ok ||
                  task.trim().length === 0
                }
              >
                {isSubmitting ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : null}
                {isSubmitting
                  ? t("agentSessions.submitting")
                  : t("agentSessions.submit")}
              </Button>
            </div>
          </div>
        </form>
      </Form>

      <p
        id={messageId}
        role="status"
        aria-live="polite"
        className={cn(
          "px-1 text-xs",
          spendLimit.ok ? "text-muted-foreground" : "text-destructive",
        )}
      >
        {!spendLimit.ok
          ? t("agentSessions.spendLimitInvalid")
          : uploads.hasErrors
            ? t("agentSessions.attachmentsBlockSend")
            : uploads.uploading
              ? t("agentSessions.attachmentsUploading")
              : t("agentSessions.keyboardHint")}
      </p>

      {showExamples ? (
        <div className="flex flex-wrap gap-2 px-1">
          {EXAMPLE_KEYS.map((key) => (
            <Button
              key={key}
              type="button"
              variant="outline"
              size="sm"
              className="h-auto max-w-full text-left text-xs font-normal whitespace-normal"
              onClick={() => insertExample(t(key))}
            >
              {t(key)}
            </Button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

/**
 * Connect GitHub from the composer (w1/m526): the single connect entry
 * (ADR078 §3b) in a NEW TAB, so the task the person typed survives. Once the
 * account is connected there — the connections query polls, and focus counts
 * too — `onConnected` re-reads the composer's readiness and repos.
 */
function GitHubEmptyCallout({ onConnected }: { onConnected: () => void }) {
  const { t } = useTranslations();
  const { connect, busy } = useConnectGitAccount({ newTab: true });
  useOnGitConnectionsChange(onConnected);
  useRefetchOnFocus(onConnected);
  return (
    <div
      className="flex flex-col items-center gap-3 px-6 py-8 text-center"
      data-testid="agent-composer-github-empty"
    >
      <GithubIcon className="text-muted-foreground size-8" />
      <div className="space-y-1">
        <p className="text-sm font-medium">
          {t("agentSessions.connectGitHubTitle")}
        </p>
        <p className="text-muted-foreground text-sm">
          {t("agentSessions.connectGitHubBody")}
        </p>
      </div>
      <div className="flex flex-wrap items-center justify-center gap-2">
        <Button type="button" disabled={busy} onClick={() => void connect()}>
          {busy ? <Loader2 className="size-4 animate-spin" /> : null}
          {t("git.connectButton")}
        </Button>
        <Button type="button" variant="ghost" asChild>
          <Link to="/settings" hash="integrations">
            {t("agentSessions.connectGitHubSettings")}
          </Link>
        </Button>
      </div>
    </div>
  );
}
