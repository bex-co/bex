import { useEffect, useId, useMemo, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "@tanstack/react-router";
import {
  ArrowUp,
  AtSign,
  BookMarked,
  Bot,
  Loader2,
  Settings2,
} from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { RemovableChip } from "@/common/components/removable-chip";
import { Textarea } from "@/common/components/ui/textarea";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { Form } from "@/common/components/ui/form";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useRepos } from "@/features/services/hooks/use-repos";
import type { RepoView } from "@/features/services/hooks/use-repos";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
} from "@/features/agent-sessions/lib/errors";
import { AGENT_COMPOSER_FOCUS_EVENT } from "@/features/agent-sessions/lib/composer-focus";
import { ConfigurationFields } from "@/features/agent-sessions/components/configuration-fields";
import { MentionPicker } from "@/features/agent-sessions/components/mention-picker";
import {
  MAX_EGRESS,
  deriveBranch,
  isBranchInNamespace,
  parseEgress,
} from "@/features/agent-sessions/lib/composer";
import type { ComposerValues } from "@/features/agent-sessions/lib/composer";
import {
  mentionEmptyText,
  mentionEnd,
  mentionOptionId,
  mentionOptions,
  mentionToken,
  mentionTokenEnd,
} from "@/features/agent-sessions/lib/mention";
import type {
  MentionOption,
  MentionState,
} from "@/features/agent-sessions/lib/mention";
import { sessionTitle } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** Configuration fields a coded create failure anchors its message to. */
const ERROR_FIELDS: Record<string, "egress" | "modelEndpoint" | undefined> = {
  AGENT_SESSION_EGRESS_ALLOWLIST_INVALID: "egress",
  AGENT_SESSION_MODEL_ENDPOINT_INVALID: "modelEndpoint",
};

/**
 * The Devin-style prompt-box composer: one task textarea with a slim in-input
 * toolbar. The repo arrives exclusively through the `@` mention picker's chip;
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

  const [task, setTask] = useState("");
  const [configOpen, setConfigOpen] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [repoChip, setRepoChip] = useState<RepoView | null>(null);
  const [sessionChips, setSessionChips] = useState<AgentSessionView[]>([]);
  const [repoNudge, setRepoNudge] = useState(false);
  const [mention, setMention] = useState<MentionState | null>(null);

  const taskRef = useRef<HTMLTextAreaElement | null>(null);
  const boxRef = useRef<HTMLDivElement | null>(null);
  const listboxId = useId();

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

  // ---- Mention state machine -----------------------------------------------

  const source = useMemo(() => ({ repos, sessions }), [repos, sessions]);
  const options = useMemo(
    () => (mention ? mentionOptions(mention, source, t) : []),
    [mention, source, t],
  );

  /** Write the task text and restore the caret afterward. */
  function writeTask(value: string, caret: number) {
    setTask(value);
    const el = taskRef.current;
    if (!el) return;
    el.focus();
    // After React flushes the controlled value.
    setTimeout(() => el.setSelectionRange(caret, caret), 0);
  }

  /** Re-derive the open mention's query from the new value/caret; close if
   *  the caret or an edit has escaped the token it was anchored to. */
  function syncMention(value: string, caret: number) {
    setMention((current) => {
      if (!current) return current;
      if (value[current.start] !== "@" || caret <= current.start) return null;
      const tokenEnd = mentionTokenEnd(current);
      if (
        current.category !== null &&
        (!value.startsWith(mentionToken(current.category), current.start) ||
          caret < tokenEnd)
      ) {
        return null;
      }
      const query = value.slice(tokenEnd, caret);
      return query === current.query
        ? current
        : { ...current, query, highlight: 0 };
    });
  }

  function handleTaskChange(value: string, caret: number) {
    setTask(value);
    if (mention) {
      syncMention(value, caret);
      return;
    }
    const prev = caret >= 2 ? value[caret - 2] : "";
    if (value[caret - 1] === "@" && (caret === 1 || /\s/.test(prev))) {
      setRepoNudge(false);
      setMention({ category: null, start: caret - 1, query: "", highlight: 0 });
    }
  }

  function openMentionFromButton() {
    setRepoNudge(false);
    if (mention) {
      setMention(null);
      return;
    }
    const caret = taskRef.current?.selectionStart ?? task.length;
    writeTask(task.slice(0, caret) + "@" + task.slice(caret), caret + 1);
    setMention({ category: null, start: caret, query: "", highlight: 0 });
  }

  function selectOption(option: MentionOption) {
    if (!mention) return;
    const end = mentionEnd(mention);
    if (option.kind === "category") {
      const token = mentionToken(option.category);
      writeTask(
        task.slice(0, mention.start) + token + task.slice(end),
        mention.start + token.length,
      );
      setMention({
        category: option.category,
        start: mention.start,
        query: "",
        highlight: 0,
      });
      return;
    }
    // Picking an item takes the typed token back out and leaves a chip.
    writeTask(task.slice(0, mention.start) + task.slice(end), mention.start);
    if (option.kind === "repo") {
      setRepoChip(option.repo);
      setRepoNudge(false);
    } else {
      setSessionChips((chips) =>
        chips.some((c) => c.id === option.session.id)
          ? chips
          : [...chips, option.session],
      );
    }
    setMention(null);
  }

  function handleTaskKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (!mention) return;
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        setMention(
          (m) =>
            m && {
              ...m,
              highlight: Math.min(m.highlight + 1, options.length - 1),
            },
        );
        return;
      case "ArrowUp":
        event.preventDefault();
        setMention(
          (m) => m && { ...m, highlight: Math.max(m.highlight - 1, 0) },
        );
        return;
      case "Enter": {
        event.preventDefault();
        const option = options[mention.highlight];
        if (option) selectOption(option);
        return;
      }
      case "Escape":
        event.preventDefault();
        setMention(null);
        return;
    }
  }

  const mentionOpen = mention !== null;
  useEffect(() => {
    if (!mentionOpen) return;
    function onPointerDown(event: PointerEvent) {
      if (!boxRef.current?.contains(event.target as Node)) setMention(null);
    }
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [mentionOpen]);

  useEffect(() => {
    function onFocusRequest() {
      taskRef.current?.focus();
    }
    window.addEventListener(AGENT_COMPOSER_FOCUS_EVENT, onFocusRequest);
    return () =>
      window.removeEventListener(AGENT_COMPOSER_FOCUS_EVENT, onFocusRequest);
  }, []);

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
    if (!repoChip) {
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
      ...sessionChips.map((s) => `Context: agent session ${s.id}`),
    ].join("\n\n");

    try {
      const ticket = await create({
        ownerId: currentWorkspaceId,
        repo: repoChip.fullName,
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
          <div
            ref={boxRef}
            className="bg-background focus-within:border-ring relative rounded-xl border shadow-sm"
          >
            {repoChip || sessionChips.length > 0 ? (
              <div className="flex flex-wrap gap-1.5 px-2.5 pt-2.5">
                {repoChip ? (
                  <MentionChip
                    icon={<BookMarked className="size-3" aria-hidden />}
                    label={repoChip.fullName}
                    onRemove={() => setRepoChip(null)}
                  />
                ) : null}
                {sessionChips.map((session) => (
                  <MentionChip
                    key={session.id}
                    icon={<Bot className="size-3" aria-hidden />}
                    label={sessionTitle(session)}
                    onRemove={() =>
                      setSessionChips((chips) =>
                        chips.filter((c) => c.id !== session.id),
                      )
                    }
                  />
                ))}
              </div>
            ) : null}

            <Textarea
              ref={taskRef}
              value={task}
              onChange={(event) =>
                handleTaskChange(
                  event.target.value,
                  event.target.selectionStart ?? event.target.value.length,
                )
              }
              onSelect={(event) =>
                syncMention(
                  event.currentTarget.value,
                  event.currentTarget.selectionStart ?? 0,
                )
              }
              onKeyDown={handleTaskKeyDown}
              rows={4}
              autoFocus
              aria-label={t("agentSessions.taskLabel")}
              role="combobox"
              aria-expanded={mentionOpen}
              aria-controls={listboxId}
              aria-activedescendant={
                mention && options.length > 0
                  ? mentionOptionId(listboxId, mention.highlight)
                  : undefined
              }
              placeholder={t("agentSessions.taskPlaceholder")}
              className="min-h-16 resize-none rounded-none border-0 bg-transparent shadow-none focus-visible:ring-0 dark:bg-transparent"
            />

            <div className="flex items-center gap-1 px-1.5 pb-1.5">
              <div className="relative">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  aria-label={t("agentSessions.mentionButton")}
                  onClick={openMentionFromButton}
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

            {mention ? (
              <MentionPicker
                idBase={listboxId}
                options={options}
                highlight={mention.highlight}
                emptyText={t(mentionEmptyText(mention, source, reposLoading))}
                onHighlight={(highlight) =>
                  setMention((m) => m && { ...m, highlight })
                }
                onSelect={selectOption}
              />
            ) : null}
          </div>
        </form>
      </Form>
    </div>
  );
}

function MentionChip({
  icon,
  label,
  onRemove,
}: {
  icon: React.ReactNode;
  label: string;
  onRemove: () => void;
}) {
  const { t } = useTranslations();
  return (
    <RemovableChip
      icon={icon}
      removeLabel={t("agentSessions.chipRemove", { name: label })}
      onRemove={onRemove}
      className="max-w-64"
    >
      <span className="truncate">{label}</span>
    </RemovableChip>
  );
}
