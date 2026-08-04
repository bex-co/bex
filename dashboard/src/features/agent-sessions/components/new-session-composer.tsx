import { useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "@tanstack/react-router";
import { ChevronDown, ChevronRight, Loader2 } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Textarea } from "@/common/components/ui/textarea";
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
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
} from "@/features/agent-sessions/lib/errors";

/** Max egress hostnames the backend accepts (ADR047 D9 — kept in sync as a
 *  friendly client-side pre-check; the backend re-validates per entry). */
const MAX_EGRESS = 32;

type AgentOption = "claude" | "gemini" | "codex";

interface ComposerValues {
  task: string;
  repo: string;
  branch: string;
  agent: AgentOption;
  model: string;
  modelEndpoint: string;
  /** Raw textarea — one hostname per line (also accepts commas). */
  egress: string;
}

/** Split the egress textarea into trimmed, non-empty hostnames. */
function parseEgress(raw: string): string[] {
  return raw
    .split(/[\n,]/)
    .map((h) => h.trim())
    .filter((h) => h.length > 0);
}

/**
 * The Devin-style new-session composer (ADR047 D9): a prominent task textarea,
 * repo + `bex-agent/*` branch, an agent select, and a collapsed Advanced
 * section (model / model endpoint / egress allowlist). Submits
 * `createAgentSession` through t001's mutation hook and navigates to the new
 * session's detail on success. Every typed `AGENT_SESSION_*` code is surfaced
 * inline (field-scoped where the code names a field, else a form-level alert),
 * and the 503/unconfigured state renders a house callout while the list keeps
 * rendering. Same-origin only — the create rides the existing Apollo client.
 */
export function NewSessionComposer() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const { create } = useAgentSessionMutations();

  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const form = useForm<ComposerValues>({
    defaultValues: {
      task: "",
      repo: "",
      branch: "bex-agent/",
      agent: "claude",
      model: "",
      modelEndpoint: "",
      egress: "",
    },
  });
  const {
    formState: { isSubmitting },
  } = form;

  const onSubmit = form.handleSubmit(async (values) => {
    setUnavailable(false);
    setSubmitError(null);

    const egressAllowlist = parseEgress(values.egress);
    if (egressAllowlist.length > MAX_EGRESS) {
      form.setError("egress", {
        message: t("agentSessions.egressTooMany"),
      });
      return;
    }

    try {
      const ticket = await create({
        ownerId: currentWorkspaceId,
        repo: values.repo.trim(),
        branch: values.branch.trim(),
        agent: values.agent,
        model: values.model.trim() || undefined,
        modelEndpoint: values.modelEndpoint.trim() || undefined,
        task: values.task.trim(),
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
      // Anchor the message to the field the code names, so the fix is obvious.
      if (err.code === "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID") {
        form.setError("egress", { message });
        setAdvancedOpen(true);
        return;
      }
      if (err.code === "AGENT_SESSION_MODEL_ENDPOINT_INVALID") {
        form.setError("modelEndpoint", { message });
        setAdvancedOpen(true);
        return;
      }
      setSubmitError(message);
      return;
    }
    setSubmitError(err instanceof Error ? err.message : String(err));
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("agentSessions.composerTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {unavailable ? (
          <Alert className="mb-4">
            <AlertTitle>{t("agentSessions.unavailableTitle")}</AlertTitle>
            <AlertDescription>
              {t("agentSessions.unavailableBody")}
            </AlertDescription>
          </Alert>
        ) : null}
        {submitError ? (
          <Alert variant="destructive" className="mb-4">
            <AlertTitle>{t("agentSessions.createErrorTitle")}</AlertTitle>
            <AlertDescription>{submitError}</AlertDescription>
          </Alert>
        ) : null}

        <Form {...form}>
          <form onSubmit={onSubmit} className="space-y-4">
            {/* Prominent task prompt */}
            <FormField
              control={form.control}
              name="task"
              rules={{
                validate: (v) =>
                  v.trim().length > 0 || t("agentSessions.taskRequired"),
              }}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("agentSessions.taskLabel")}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={4}
                      placeholder={t("agentSessions.taskPlaceholder")}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="repo"
                rules={{
                  validate: (v) =>
                    v.trim().length > 0 || t("agentSessions.repoRequired"),
                }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("agentSessions.repoLabel")}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        autoComplete="off"
                        placeholder={t("agentSessions.repoPlaceholder")}
                      />
                    </FormControl>
                    <FormDescription>
                      {t("agentSessions.repoHint")}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="branch"
                rules={{
                  validate: (v) =>
                    v.trim().length > 0 || t("agentSessions.branchRequired"),
                }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("agentSessions.branchLabel")}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        autoComplete="off"
                        placeholder={t("agentSessions.branchPlaceholder")}
                      />
                    </FormControl>
                    <FormDescription>
                      {t("agentSessions.branchHint")}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

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
                      <SelectTrigger className="w-full sm:w-56">
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

            {/* Collapsed Advanced section */}
            <div className="rounded-md border">
              <button
                type="button"
                onClick={() => setAdvancedOpen((o) => !o)}
                aria-expanded={advancedOpen}
                className="flex w-full items-center gap-1.5 px-4 py-2.5 text-sm font-medium"
              >
                {advancedOpen ? (
                  <ChevronDown className="size-4" />
                ) : (
                  <ChevronRight className="size-4" />
                )}
                {advancedOpen
                  ? t("agentSessions.advancedHide")
                  : t("agentSessions.advancedShow")}
              </button>
              {advancedOpen ? (
                <div className="space-y-4 border-t p-4">
                  <div className="grid gap-4 sm:grid-cols-2">
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
                          <FormDescription>
                            {t("agentSessions.modelHint")}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="modelEndpoint"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t("agentSessions.modelEndpointLabel")}
                          </FormLabel>
                          <FormControl>
                            <Input
                              {...field}
                              autoComplete="off"
                              inputMode="url"
                              placeholder={t(
                                "agentSessions.modelEndpointPlaceholder",
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t("agentSessions.modelEndpointHint")}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
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
                        <FormDescription>
                          {t("agentSessions.egressHint")}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              ) : null}
            </div>

            <div className="flex justify-end">
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? (
                  <>
                    <Loader2 className="animate-spin" />
                    {t("agentSessions.submitting")}
                  </>
                ) : (
                  t("agentSessions.submit")
                )}
              </Button>
            </div>
          </form>
        </Form>
      </CardContent>
    </Card>
  );
}
