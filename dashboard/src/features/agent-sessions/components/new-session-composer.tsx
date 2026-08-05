import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { useForm } from "react-hook-form";
import type { UseFormReturn } from "react-hook-form";
import { useNavigate } from "@tanstack/react-router";
import {
  ArrowUp,
  AtSign,
  BookMarked,
  Bot,
  Loader2,
  Settings2,
  X,
} from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Textarea } from "@/common/components/ui/textarea";
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
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/common/components/ui/form";
import { useTranslations } from "@/common/hooks/use-translations";
import { toSlug } from "@/common/lib/utils/slug";
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
import { MentionPicker } from "@/features/agent-sessions/components/mention-picker";
import {
  fuzzyMatch,
  mentionOptionId,
  mentionToken,
} from "@/features/agent-sessions/lib/mention";
import type {
  MentionCategory,
  MentionOption,
} from "@/features/agent-sessions/lib/mention";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** Max egress hostnames the backend accepts (ADR047 D9 — kept in sync as a
 *  friendly client-side pre-check; the backend re-validates per entry). */
const MAX_EGRESS = 32;

/** The mandated working-branch namespace (ADR047 — `bex-agent/*` confinement). */
const BRANCH_PREFIX = "bex-agent/";

/** Max slug length appended to the branch prefix when auto-deriving. */
const BRANCH_SLUG_MAX = 40;

type AgentOption = "claude" | "gemini" | "codex";

interface ComposerValues {
  task: string;
  branch: string;
  agent: AgentOption;
  model: string;
  modelEndpoint: string;
  /** Raw textarea — one hostname per line (also accepts commas). */
  egress: string;
}

/** The open mention popup's state: level + the `@`'s index + the live caret. */
interface MentionState {
  level: "category" | "items";
  category?: MentionCategory;
  /** Index of the trigger `@` in the task text. */
  start: number;
  /** Current caret position (filter text = between token end and caret). */
  caret: number;
}

/** Split the egress textarea into trimmed, non-empty hostnames. */
function parseEgress(raw: string): string[] {
  return raw
    .split(/[\n,]/)
    .map((h) => h.trim())
    .filter((h) => h.length > 0);
}

/** Auto-derive `bex-agent/<slug>` from the task text, truncated sensibly. */
function deriveBranch(task: string): string {
  const slug = toSlug(task).slice(0, BRANCH_SLUG_MAX).replace(/-+$/, "");
  return BRANCH_PREFIX + slug;
}

const MENTION_CATEGORIES: MentionCategory[] = ["repos", "sessions"];

/**
 * The Devin-style prompt-box composer (w3/m45): ONE large task textarea with a
 * slim in-input toolbar — the `@` mention button, the Configuration popover
 * (agent/model/endpoint/egress + the editable auto-derived branch), and Send —
 * and no visible form fields. The repo arrives exclusively through the `@`
 * mention picker's chip; submitting without one nudges inline at the `@`
 * button instead of submitting. Submits `createAgentSession` through the
 * mutation hook and navigates to the new session's detail on success. Every
 * typed `AGENT_SESSION_*` code is surfaced inline — egress/model-endpoint
 * codes anchor to their Configuration fields and auto-open the popover; the
 * 503/unconfigured state renders a house callout above the box.
 */
export function NewSessionComposer() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const { create } = useAgentSessionMutations();
  const { repos, loading: reposLoading } = useRepos();
  const { sessions } = useAgentSessions();

  const [configOpen, setConfigOpen] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [repoChip, setRepoChip] = useState<RepoView | null>(null);
  const [sessionChips, setSessionChips] = useState<AgentSessionView[]>([]);
  const [repoNudge, setRepoNudge] = useState(false);
  const [mention, setMention] = useState<MentionState | null>(null);
  const [highlight, setHighlight] = useState(0);

  const taskRef = useRef<HTMLTextAreaElement | null>(null);
  const boxRef = useRef<HTMLDivElement | null>(null);
  /** True once the user hand-edits the branch — stops task-driven derivation. */
  const branchDirty = useRef(false);
  const listboxId = useId();

  const form = useForm<ComposerValues>({
    defaultValues: {
      task: "",
      branch: BRANCH_PREFIX,
      agent: "claude",
      model: "",
      modelEndpoint: "",
      egress: "",
    },
  });
  const {
    formState: { isSubmitting },
  } = form;
  const taskValue = form.watch("task");

  // ---- Mention state machine -----------------------------------------------

  const mentionQuery = useMemo(() => {
    if (!mention) return "";
    const tokenEnd =
      mention.level === "category"
        ? mention.start + 1
        : mention.start + mentionToken(mention.category!).length;
    return taskValue.slice(tokenEnd, mention.caret);
  }, [mention, taskValue]);

  const mentionOptions = useMemo<MentionOption[]>(() => {
    if (!mention) return [];
    if (mention.level === "category") {
      return MENTION_CATEGORIES.filter((category) => {
        const label =
          category === "repos"
            ? t("agentSessions.mentionCategoryRepos")
            : t("agentSessions.mentionCategorySessions");
        return (
          fuzzyMatch(mentionQuery, category) || fuzzyMatch(mentionQuery, label)
        );
      }).map((category) => ({ kind: "category" as const, category }));
    }
    if (mention.category === "repos") {
      return repos
        .filter((repo) => fuzzyMatch(mentionQuery, repo.fullName))
        .map((repo) => ({ kind: "repo" as const, repo }));
    }
    return sessions
      .filter((session) =>
        fuzzyMatch(mentionQuery, session.agentConfig.task || session.id),
      )
      .map((session) => ({ kind: "session" as const, session }));
  }, [mention, mentionQuery, repos, sessions, t]);

  // Reset the keyboard highlight whenever the filtered list changes shape.
  useEffect(() => {
    setHighlight(0);
  }, [mentionQuery, mention?.level, mention?.category]);

  const mentionEmptyText = !mention
    ? ""
    : mention.level === "items" && mention.category === "repos"
      ? reposLoading && repos.length === 0
        ? t("common.loading")
        : repos.length === 0
          ? t("agentSessions.mentionReposEmpty")
          : t("agentSessions.mentionNoResults")
      : mention.level === "items" && sessions.length === 0
        ? t("agentSessions.mentionSessionsEmpty")
        : t("agentSessions.mentionNoResults");

  const closeMention = useCallback(() => setMention(null), []);

  /** Re-validate the open mention against the new value/caret; close if stale. */
  const syncMention = useCallback((value: string, caret: number) => {
    setMention((current) => {
      if (!current) return current;
      if (value[current.start] !== "@" || caret <= current.start) return null;
      if (current.level === "items") {
        const token = mentionToken(current.category!);
        if (
          !value.startsWith(token, current.start) ||
          caret < current.start + token.length
        ) {
          return null;
        }
      }
      return { ...current, caret };
    });
  }, []);

  /** Set the task text (derives the branch) and restore the caret afterward. */
  const setTask = useCallback(
    (value: string, caret: number) => {
      form.setValue("task", value);
      if (!branchDirty.current) form.setValue("branch", deriveBranch(value));
      const el = taskRef.current;
      if (el) {
        el.focus();
        // After React flushes the controlled value.
        setTimeout(() => el.setSelectionRange(caret, caret), 0);
      }
    },
    [form],
  );

  function handleTaskChange(value: string, caret: number) {
    form.setValue("task", value);
    if (!branchDirty.current) form.setValue("branch", deriveBranch(value));
    if (mention) {
      syncMention(value, caret);
      return;
    }
    // A freshly typed `@` at a word boundary opens the category level.
    const prev = caret >= 2 ? value[caret - 2] : "";
    if (value[caret - 1] === "@" && (caret === 1 || /\s/.test(prev))) {
      setRepoNudge(false);
      setMention({ level: "category", start: caret - 1, caret });
    }
  }

  /** The toolbar `@` button: insert `@` at the caret and open the picker. */
  function openMentionFromButton() {
    setRepoNudge(false);
    if (mention) {
      closeMention();
      return;
    }
    const value = form.getValues("task");
    const el = taskRef.current;
    const caret = el?.selectionStart ?? value.length;
    setTask(value.slice(0, caret) + "@" + value.slice(caret), caret + 1);
    setMention({ level: "category", start: caret, caret: caret + 1 });
  }

  function selectOption(option: MentionOption) {
    if (!mention) return;
    const value = form.getValues("task");
    if (option.kind === "category") {
      // Swap the typed text for the category token and move to level 2.
      const token = mentionToken(option.category);
      const next =
        value.slice(0, mention.start) + token + value.slice(mention.caret);
      const caret = mention.start + token.length;
      setTask(next, caret);
      setMention({
        level: "items",
        category: option.category,
        start: mention.start,
        caret,
      });
      return;
    }
    // Picking an item removes the token text and embeds a chip instead.
    const next = value.slice(0, mention.start) + value.slice(mention.caret);
    setTask(next, mention.start);
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
    closeMention();
  }

  function handleTaskKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (!mention) return;
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        setHighlight((h) => Math.min(h + 1, mentionOptions.length - 1));
        return;
      case "ArrowUp":
        event.preventDefault();
        setHighlight((h) => Math.max(h - 1, 0));
        return;
      case "Enter": {
        event.preventDefault();
        const option = mentionOptions[highlight];
        if (option) selectOption(option);
        return;
      }
      case "Escape":
        event.preventDefault();
        closeMention();
        return;
    }
  }

  // Close the mention popup on any pointer press outside the prompt box.
  useEffect(() => {
    if (!mention) return;
    function onPointerDown(event: PointerEvent) {
      if (!boxRef.current?.contains(event.target as Node)) closeMention();
    }
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [mention, closeMention]);

  // The sidebar's New-session shortcut focuses the prompt box (w3/m45 t004).
  useEffect(() => {
    function onFocusRequest() {
      taskRef.current?.focus();
    }
    window.addEventListener(AGENT_COMPOSER_FOCUS_EVENT, onFocusRequest);
    return () =>
      window.removeEventListener(AGENT_COMPOSER_FOCUS_EVENT, onFocusRequest);
  }, []);

  // ---- Submit --------------------------------------------------------------

  const onSubmit = form.handleSubmit(
    async (values) => {
      setUnavailable(false);
      setSubmitError(null);

      // Never silently submit without a repo — nudge at the `@` button.
      if (!repoChip) {
        setRepoNudge(true);
        return;
      }

      // Enforce the `bex-agent/*` namespace here too: the field rule only runs
      // while the Configuration popover is mounted (react-hook-form skips
      // unmounted fields), and the branch must be valid either way.
      const branch = values.branch.trim();
      const branchError = !branch
        ? t("agentSessions.branchRequired")
        : !branch.startsWith(BRANCH_PREFIX) ||
            branch.length <= BRANCH_PREFIX.length
          ? t("agentSessions.branchInvalid")
          : null;
      if (branchError) {
        form.setError("branch", { message: branchError });
        setConfigOpen(true);
        return;
      }

      const egressAllowlist = parseEgress(values.egress);
      if (egressAllowlist.length > MAX_EGRESS) {
        form.setError("egress", { message: t("agentSessions.egressTooMany") });
        setConfigOpen(true);
        return;
      }

      // Session mentions ride along as context lines in the task prompt
      // (t002 — "mention a prior session id in the task text").
      const task = [
        values.task.trim(),
        ...sessionChips.map((s) => `Context: agent session ${s.id}`),
      ]
        .filter(Boolean)
        .join("\n\n");

      try {
        const ticket = await create({
          ownerId: currentWorkspaceId,
          repo: repoChip.fullName,
          branch,
          agent: values.agent,
          model: values.model.trim() || undefined,
          modelEndpoint: values.modelEndpoint.trim() || undefined,
          task,
          egressAllowlist,
        });
        await navigate({
          to: "/agents/$agentSessionId",
          params: { agentSessionId: ticket.session.id },
        });
      } catch (err) {
        handleCreateError(err);
      }
    },
    // A rejected Configuration field (branch/model/endpoint/egress) lives in
    // the popover — open it so the anchored message is actually visible.
    (errors) => {
      if (
        errors.branch ||
        errors.model ||
        errors.modelEndpoint ||
        errors.egress
      ) {
        setConfigOpen(true);
      }
    },
  );

  /** Route a caught create rejection onto the right inline surface. */
  function handleCreateError(err: unknown) {
    if (err instanceof AgentSessionsUnavailableError) {
      setUnavailable(true);
      return;
    }
    if (err instanceof AgentSessionError) {
      // Resolve the coded message through i18n, falling back to the server's
      // own message for any code we don't have a locale key for yet.
      const message = t(err.messageKey, {
        ...err.params,
        defaultValue: err.message,
      });
      // Anchor the message to the field the code names — both live in the
      // Configuration popover, so open it (the old Advanced auto-expand).
      if (err.code === "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID") {
        form.setError("egress", { message });
        setConfigOpen(true);
        return;
      }
      if (err.code === "AGENT_SESSION_MODEL_ENDPOINT_INVALID") {
        form.setError("modelEndpoint", { message });
        setConfigOpen(true);
        return;
      }
      setSubmitError(message);
      return;
    }
    setSubmitError(err instanceof Error ? err.message : String(err));
  }

  const sendDisabled = isSubmitting || taskValue.trim().length === 0;

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
          <div ref={boxRef} className="relative">
            <div className="bg-background focus-within:border-ring rounded-xl border shadow-sm">
              {repoChip || sessionChips.length > 0 ? (
                <div className="flex flex-wrap gap-1.5 px-3 pt-3">
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
                      label={session.agentConfig.task || session.id}
                      onRemove={() =>
                        setSessionChips((chips) =>
                          chips.filter((c) => c.id !== session.id),
                        )
                      }
                    />
                  ))}
                </div>
              ) : null}

              <FormField
                control={form.control}
                name="task"
                rules={{
                  validate: (v) =>
                    v.trim().length > 0 || t("agentSessions.taskRequired"),
                }}
                render={({ field }) => (
                  <FormItem className="gap-0">
                    <FormControl>
                      <Textarea
                        {...field}
                        ref={(el) => {
                          field.ref(el);
                          taskRef.current = el;
                        }}
                        onChange={(event) =>
                          handleTaskChange(
                            event.target.value,
                            event.target.selectionStart ??
                              event.target.value.length,
                          )
                        }
                        onSelect={(event) => {
                          const el = event.currentTarget;
                          syncMention(el.value, el.selectionStart ?? 0);
                        }}
                        onKeyDown={handleTaskKeyDown}
                        rows={4}
                        autoFocus
                        aria-label={t("agentSessions.taskLabel")}
                        role="combobox"
                        aria-expanded={mention != null}
                        aria-controls={listboxId}
                        aria-activedescendant={
                          mention && mentionOptions.length > 0
                            ? mentionOptionId(listboxId, highlight)
                            : undefined
                        }
                        placeholder={t("agentSessions.taskPlaceholder")}
                        className="min-h-24 resize-none rounded-none border-0 bg-transparent shadow-none focus-visible:ring-0 dark:bg-transparent"
                      />
                    </FormControl>
                    <FormMessage className="px-3 pb-1" />
                  </FormItem>
                )}
              />

              {/* Slim in-input toolbar: @ mention, Configuration, Send. */}
              <div className="flex items-center gap-1 px-2 pb-2">
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
                    <ConfigurationFields
                      form={form}
                      onBranchEdited={() => {
                        branchDirty.current = true;
                      }}
                    />
                  </PopoverContent>
                </Popover>

                <div className="flex-1" />

                <Button
                  type="submit"
                  size="icon"
                  className="size-8 rounded-full"
                  disabled={sendDisabled}
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

            {mention ? (
              <MentionPicker
                idBase={listboxId}
                options={mentionOptions}
                highlight={highlight}
                emptyText={mentionEmptyText}
                onHighlight={setHighlight}
                onSelect={selectOption}
              />
            ) : null}
          </div>
        </form>
      </Form>
    </div>
  );
}

/** A removable mention chip embedded above the prompt textarea. */
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
    <Badge variant="secondary" className="max-w-64 gap-1 pr-1">
      {icon}
      <span className="truncate">{label}</span>
      <button
        type="button"
        aria-label={t("agentSessions.chipRemove", { name: label })}
        onClick={onRemove}
        className="hover:bg-muted-foreground/20 rounded-sm p-0.5"
      >
        <X className="size-3" />
      </button>
    </Badge>
  );
}

/**
 * The Configuration popover body (t003): the relocated Advanced fields —
 * agent select, model, model endpoint, egress allowlist — plus the editable
 * auto-derived `bex-agent/*` branch. Lives on the same react-hook-form
 * instance as the composer, so values and server-anchored errors round-trip.
 */
function ConfigurationFields({
  form,
  onBranchEdited,
}: {
  form: UseFormReturn<ComposerValues>;
  onBranchEdited: () => void;
}) {
  const { t } = useTranslations();
  return (
    <>
      <FormField
        control={form.control}
        name="branch"
        rules={{
          validate: (v) => {
            const trimmed = v.trim();
            if (!trimmed) return t("agentSessions.branchRequired");
            if (
              !trimmed.startsWith(BRANCH_PREFIX) ||
              trimmed.length <= BRANCH_PREFIX.length
            ) {
              return t("agentSessions.branchInvalid");
            }
            return true;
          },
        }}
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.branchLabel")}</FormLabel>
            <FormControl>
              <Input
                {...field}
                onChange={(event) => {
                  onBranchEdited();
                  field.onChange(event);
                }}
                autoComplete="off"
                placeholder={t("agentSessions.branchPlaceholder")}
              />
            </FormControl>
            <FormDescription>{t("agentSessions.branchHint")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="agent"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.agentLabel")}</FormLabel>
            <Select
              value={field.value}
              onValueChange={(v) => field.onChange(v as AgentOption)}
            >
              <FormControl>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
              </FormControl>
              <SelectContent>
                <SelectItem value="claude">
                  {t("agentSessions.agentClaude")}
                </SelectItem>
                <SelectItem value="gemini">
                  {t("agentSessions.agentGemini")}
                </SelectItem>
                <SelectItem value="codex">
                  {t("agentSessions.agentCodex")}
                </SelectItem>
              </SelectContent>
            </Select>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="model"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.modelLabel")}</FormLabel>
            <FormControl>
              <Input
                {...field}
                autoComplete="off"
                placeholder={t("agentSessions.modelPlaceholder")}
              />
            </FormControl>
            <FormDescription>{t("agentSessions.modelHint")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="modelEndpoint"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.modelEndpointLabel")}</FormLabel>
            <FormControl>
              <Input
                {...field}
                autoComplete="off"
                inputMode="url"
                placeholder={t("agentSessions.modelEndpointPlaceholder")}
              />
            </FormControl>
            <FormDescription>
              {t("agentSessions.modelEndpointHint")}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="egress"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.egressLabel")}</FormLabel>
            <FormControl>
              <Textarea
                {...field}
                rows={3}
                className="font-mono text-sm"
                placeholder={t("agentSessions.egressPlaceholder")}
              />
            </FormControl>
            <FormDescription>{t("agentSessions.egressHint")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
  );
}
