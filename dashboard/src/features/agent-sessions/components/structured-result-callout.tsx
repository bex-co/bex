import { Braces, Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatResultJson } from "@/features/agent-sessions/lib/format-result";
import { useAgentSessionTurnResult } from "@/features/agent-sessions/hooks/use-agent-session-turn-result";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * The turn's schema-validated structured result (ADR094 D5, w21/003) as a
 * collapsible JSON block under the conversation. The final value is
 * `outcome.result`; while the turn runs (or if it ended without declaring one
 * final) the agent's latest saved value shows as a labelled "draft" read from
 * the transcript turn. Renders nothing for a free-form turn, so like the
 * failure callout it adds no region to the loading skeleton.
 */
export function StructuredResultCallout({
  session,
}: {
  session: AgentSessionView;
}) {
  const { t } = useTranslations();
  const finalResult = session.outcome?.result ?? null;
  const draft = useAgentSessionTurnResult(session.id, !session.isTerminal);
  // outcome.result describes the latest turn only; a stale final value from an
  // earlier turn never reaches here because the server resets it per turn.
  const isDraft = finalResult == null;
  const text = finalResult ?? (draft && !draft.final ? draft.result : null);
  if (text == null) return null;
  const json = formatResultJson(text);

  async function copy() {
    try {
      await navigator.clipboard.writeText(json);
      toast.success(t("agentSessions.resultCopied"));
    } catch (err) {
      toast.error(mutationErrorMessage(err, t("common.errorDefaultMessage")));
    }
  }

  return (
    <details
      open
      className="bg-card rounded-lg border text-sm"
      data-testid="agent-session-result"
      data-result-state={isDraft ? "draft" : "final"}
    >
      <summary className="flex cursor-pointer items-center gap-2 px-3 py-2 font-medium">
        <Braces aria-hidden className="text-muted-foreground size-4" />
        {isDraft
          ? t("agentSessions.resultDraftTitle")
          : t("agentSessions.resultTitle")}
      </summary>
      <div className="space-y-2 border-t px-3 py-2">
        {isDraft ? (
          <p className="text-muted-foreground text-xs">
            {t("agentSessions.resultDraftHint")}
          </p>
        ) : null}
        <pre className="bg-muted max-h-80 overflow-auto rounded-md p-2 font-mono text-xs break-words whitespace-pre-wrap">
          {json}
        </pre>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => void copy()}
        >
          <Copy className="size-4" />
          {t("agentSessions.resultCopy")}
        </Button>
      </div>
    </details>
  );
}
