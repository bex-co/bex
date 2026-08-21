import { useMemo, useRef, useState } from "react";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { useForm } from "react-hook-form";
import { Link, useNavigate } from "@tanstack/react-router";
import { AtSign, Github, Loader2, Settings2 } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
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
import { cn } from "@/common/lib/utils/utils";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useRepos } from "@/features/services/hooks/use-repos";
import { useConnectGit } from "@/features/git/hooks/use-connect-git";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
} from "@/features/agent-sessions/lib/errors";
import { ConfigurationFields } from "@/features/agent-sessions/components/configuration-fields";
import {
  InlineMentionEditor,
  type InlineMentionEditorHandle,
} from "@/features/agent-sessions/components/lazy-mention-editor";
import {
  AGENT_OPTIONS,
  MAX_EGRESS,
  deriveBranch,
  isBranchInNamespace,
  parseEgress,
  type AgentOption,
  type ComposerValues,
} from "@/features/agent-sessions/lib/composer";
import type { ComposerDocument } from "@/features/agent-sessions/lib/composer-document";

/** Configuration fields a coded create failure anchors its message to. */
const ERROR_FIELDS: Record<string, "egress" | "modelEndpoint" | undefined> = {
  AGENT_SESSION_EGRESS_ALLOWLIST_INVALID: "egress",
  AGENT_SESSION_MODEL_ENDPOINT_INVALID: "modelEndpoint",
};

const EXAMPLE_KEYS = [
  "agentSessions.exampleFixTests",
  "agentSessions.exampleAddReadme",
] as const;

/**
 * The prompt-box composer: the visual center of `/agents`. Agent and repo live
 * on the toolbar; Advanced keeps branch / model / endpoint / egress. Submitting
 * without a repo highlights the chip rather than firing create. A workspace
 * with zero GitHub App repos is a Connect GitHub wall, not a writable prompt.
 */
export function NewSessionComposer() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const { create } = useAgentSessionMutations();
  const { repos, loading: reposLoading } = useRepos();
  const { sessions } = useAgentSessions({ poll: false });

  const [composerDocument, setComposerDocument] = useState<ComposerDocument>({
    task: "",
    repo: null,
    sessionIds: [],
  });
  const [configOpen, setConfigOpen] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [repoNudge, setRepoNudge] = useState(false);
  const editorRef = useRef<InlineMentionEditorHandle | null>(null);
  const { task, repo, sessionIds } = composerDocument;
  const noRepos = !reposLoading && repos.length === 0;
  const showExamples = !noRepos && sessions.length === 0;

  const form = useForm<ComposerValues>({
    defaultValues: {
      branch: "",
      agent: "claude",
      model: "",
      modelEndpoint: "",
      egress: "",
    },
  });
  const {
    formState: { isSubmitting },
  } = form;

  const source = useMemo(() => ({ repos, sessions }), [repos, sessions]);

  function failInConfig(
    field: "branch" | "egress" | "modelEndpoint",
    message: string,
  ) {
    form.setError(field, { message });
    setConfigOpen(true);
  }

  const onSubmit = form.handleSubmit(async (values) => {
    setUnavailable(false);
    setSubmitError(null);

    if (noRepos) return;
    if (!repo) {
      setRepoNudge(true);
      return;
    }

    const branch = values.branch.trim() || deriveBranch(task);
    if (!isBranchInNamespace(branch)) {
      failInConfig("branch", t("agentSessions.branchInvalid"));
      return;
    }

    const egressAllowlist = parseEgress(values.egress);
    if (egressAllowlist.length > MAX_EGRESS) {
      failInConfig("egress", t("agentSessions.egressTooMany"));
      return;
    }

    const prompt = [
      task.trim(),
      ...sessionIds.map((id) => `Context: agent session ${id}`),
    ].join("\n\n");

    try {
      const ticket = await create({
        ownerId: currentWorkspaceId,
        repo,
        branch,
        agent: values.agent,
        model: values.model.trim() || undefined,
        modelEndpoint: values.modelEndpoint.trim() || undefined,
        task: prompt,
        egressAllowlist,
      });
      await navigate({
        to: "/agents/$agentSessionId",
        params: { agentSessionId: ticket.session.id },
      });
    } catch (err) {
      handleCreateError(err);
    }
  });

  function handleCreateError(err: unknown) {
    // The user closed the ADR075 D7 payment dialog — their own choice, not an
    // error to report.
    if (isPaymentOnboardingCancelled(err)) return;
    if (err instanceof AgentSessionsUnavailableError) {
      setUnavailable(true);
      return;
    }
    if (!(err instanceof AgentSessionError)) {
      setSubmitError(err instanceof Error ? err.message : String(err));
      return;
    }
    const message = t(err.messageKey, {
      ...err.params,
      defaultValue: err.message,
    });
    const field = ERROR_FIELDS[err.code];
    if (field) failInConfig(field, message);
    else setSubmitError(message);
  }

  function openMention() {
    setRepoNudge(false);
    editorRef.current?.openMention();
  }

  function insertExample(text: string) {
    editorRef.current?.insertPrompt(text);
    openMention();
  }

  return (
    <div className="space-y-3">
      {unavailable ? (
        <Alert>
          <AlertTitle>{t("agentSessions.unavailableTitle")}</AlertTitle>
          <AlertDescription>
            {t("agentSessions.unavailableBody")}
          </AlertDescription>
        </Alert>
      ) : null}
      {submitError ? (
        <Alert variant="destructive">
          <AlertTitle>{t("agentSessions.createErrorTitle")}</AlertTitle>
          <AlertDescription>{submitError}</AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <form onSubmit={onSubmit}>
          <div className="bg-background focus-within:border-ring relative rounded-xl border shadow-sm">
            {noRepos ? (
              <GitHubEmptyCallout />
            ) : (
              <InlineMentionEditor
                ref={editorRef}
                source={source}
                reposLoading={reposLoading}
                ariaLabel={t("agentSessions.taskLabel")}
                placeholder={t("agentSessions.taskPlaceholder")}
                onChange={(nextDocument) => {
                  setComposerDocument(nextDocument);
                  if (nextDocument.repo) setRepoNudge(false);
                }}
                onSubmit={() => {
                  if (isSubmitting || task.trim().length === 0) return;
                  void onSubmit();
                }}
              />
            )}

            <div className="flex flex-wrap items-center gap-1 px-1.5 pb-1.5">
              <FormField
                name="agent"
                render={({ field }) => (
                  <FormItem className="space-y-0">
                    <FormControl>
                      <Select
                        value={field.value}
                        onValueChange={(v) => field.onChange(v as AgentOption)}
                        disabled={noRepos}
                      >
                        <SelectTrigger
                          size="sm"
                          className="h-8 w-[7.5rem] border-0 bg-transparent shadow-none"
                          aria-label={t("agentSessions.agentLabel")}
                        >
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {AGENT_OPTIONS.map((agent) => (
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

              <div className="relative">
                <Button
                  type="button"
                  variant={repo ? "secondary" : "ghost"}
                  size="sm"
                  disabled={noRepos}
                  data-testid="agent-composer-repo-chip"
                  className={cn(
                    "h-8 max-w-48 gap-1.5 px-2 text-xs font-normal",
                    repoNudge &&
                      "ring-destructive text-destructive ring-2 ring-offset-1",
                  )}
                  aria-label={
                    repo
                      ? t("agentSessions.repoChip", { repo })
                      : t("agentSessions.addRepository")
                  }
                  onClick={openMention}
                >
                  <AtSign className="size-3.5" />
                  <span className="truncate">
                    {repo ?? t("agentSessions.addRepository")}
                  </span>
                </Button>
                {repoNudge ? (
                  <p
                    role="alert"
                    className="bg-popover text-popover-foreground absolute top-full left-0 z-20 mt-1 w-max max-w-64 rounded-md border px-2.5 py-1.5 text-xs shadow-md"
                  >
                    {t("agentSessions.repoNudge")}
                  </p>
                ) : null}
              </div>

              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8"
                disabled={noRepos}
                aria-label={t("agentSessions.mentionButton")}
                onClick={openMention}
              >
                <AtSign className="size-4" />
              </Button>

              <Popover open={configOpen} onOpenChange={setConfigOpen}>
                <PopoverTrigger asChild>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="text-muted-foreground h-8 gap-1.5 px-2 text-xs font-normal"
                  >
                    <Settings2 className="size-4" />
                    {t("agentSessions.configButton")}
                  </Button>
                </PopoverTrigger>
                <PopoverContent
                  align="start"
                  className="max-h-[70vh] w-96 max-w-[calc(100vw-2rem)] space-y-4 overflow-y-auto"
                >
                  <ConfigurationFields branchPlaceholder={deriveBranch(task)} />
                </PopoverContent>
              </Popover>

              <Button
                type="submit"
                size="sm"
                className="ml-auto h-8"
                disabled={noRepos || isSubmitting || task.trim().length === 0}
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

      {noRepos ? null : (
        <p className="text-muted-foreground px-1 text-xs">
          {t("agentSessions.keyboardHint")}
        </p>
      )}

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

function GitHubEmptyCallout() {
  const { t } = useTranslations();
  const { connect, busy } = useConnectGit();
  return (
    <div
      className="flex flex-col items-center gap-3 px-6 py-8 text-center"
      data-testid="agent-composer-github-empty"
    >
      <Github className="text-muted-foreground size-8" />
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
          {t("agentSessions.connectGitHub")}
        </Button>
        <Button type="button" variant="ghost" asChild>
          <Link to="/workspace/settings">
            {t("agentSessions.connectGitHubSettings")}
          </Link>
        </Button>
      </div>
    </div>
  );
}
