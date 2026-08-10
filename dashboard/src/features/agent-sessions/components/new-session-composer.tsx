import { useMemo, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "@tanstack/react-router";
import { ArrowUp, AtSign, Loader2, Settings2 } from "lucide-react";
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
import { Form } from "@/common/components/ui/form";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useRepos } from "@/features/services/hooks/use-repos";
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
} from "@/features/agent-sessions/components/inline-mention-editor";
import {
  MAX_EGRESS,
  deriveBranch,
  isBranchInNamespace,
  parseEgress,
} from "@/features/agent-sessions/lib/composer";
import type { ComposerValues } from "@/features/agent-sessions/lib/composer";
import type { ComposerDocument } from "@/features/agent-sessions/lib/composer-document";

/** Configuration fields a coded create failure anchors its message to. */
const ERROR_FIELDS: Record<string, "egress" | "modelEndpoint" | undefined> = {
  AGENT_SESSION_EGRESS_ALLOWLIST_INVALID: "egress",
  AGENT_SESSION_MODEL_ENDPOINT_INVALID: "modelEndpoint",
};

/**
 * The Devin-style prompt-box composer: one rich prompt editor with a slim
 * toolbar. Repositories and sessions stay where they were mentioned inline;
 * submitting without one nudges inline at the `@` button instead of
 * submitting. Every typed `AGENT_SESSION_*` code is surfaced inline — the
 * codes naming a Configuration field anchor there and auto-open the popover,
 * and the 503/unconfigured state renders a house callout above the box.
 */
export function NewSessionComposer() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const { create } = useAgentSessionMutations();
  const { repos, loading: reposLoading } = useRepos();
  // The sidebar renders alongside and owns the poll; this is a cache read.
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

  const form = useForm<ComposerValues>({
    defaultValues: {
      branch: "",
      agent: "claude",
      model: "",
      modelEndpoint: "",
      egress: "",
      openPr: false,
    },
  });
  const {
    formState: { isSubmitting },
  } = form;

  const source = useMemo(() => ({ repos, sessions }), [repos, sessions]);

  // ---- Submit --------------------------------------------------------------

  /** Anchor a message to a Configuration field and reveal it. */
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

    // Never silently submit without a repo — nudge at the `@` button.
    if (!repo) {
      setRepoNudge(true);
      return;
    }

    // Checked here rather than as a field rule because react-hook-form skips
    // unmounted fields and the Configuration popover is usually closed.
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

    // Session mentions ride along as context lines in the task prompt.
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
        openPr: values.openPr,
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
    if (err instanceof AgentSessionsUnavailableError) {
      setUnavailable(true);
      return;
    }
    if (!(err instanceof AgentSessionError)) {
      setSubmitError(err instanceof Error ? err.message : String(err));
      return;
    }
    // Resolve the coded message through i18n, falling back to the server's own
    // message for any code we don't have a locale key for yet.
    const message = t(err.messageKey, {
      ...err.params,
      defaultValue: err.message,
    });
    const field = ERROR_FIELDS[err.code];
    if (field) failInConfig(field, message);
    else setSubmitError(message);
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
            />

            <div className="flex items-center gap-1 px-1.5 pb-1.5">
              <div className="relative">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  aria-label={t("agentSessions.mentionButton")}
                  onClick={() => {
                    setRepoNudge(false);
                    editorRef.current?.openMention();
                  }}
                >
                  <AtSign className="size-4" />
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
                size="icon"
                className="ml-auto size-8 rounded-full"
                disabled={isSubmitting || task.trim().length === 0}
                aria-label={t("agentSessions.submit")}
              >
                {isSubmitting ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <ArrowUp className="size-4" />
                )}
              </Button>
            </div>
          </div>
        </form>
      </Form>
    </div>
  );
}
