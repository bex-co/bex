import { useState } from "react";
import { useQuery } from "@apollo/client/react";
import { Loader2 } from "lucide-react";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { AgentSessionToolOutputDocument } from "@/graphql/definitions";
import {
  AgentSessionError,
  toAgentSessionError,
} from "@/features/agent-sessions/lib/errors";
import type { ActivityStep } from "@/features/agent-sessions/lib/activity-steps";

// "Show full output" for a clipped tool step (w10/m18, ADR051 amendment
// 2026-10-27). The durable transcript keeps only the head and tail of a long
// tool output inline; when the replayed part says the server kept the
// complete text, this loads it on demand from bex-api. Outputs clipped before
// full copies were kept say so instead.

type FullOutputStep = Extract<ActivityStep, { kind: "tool" | "terminal" }>;

// The server's field names (agenttranscript.FieldOutput/FieldErrorText/
// TerminalField) for the text this step shows clipped.
function stepFields(step: FullOutputStep): string[] {
  if (step.kind === "terminal") {
    return [step.terminalId ? `terminal:${step.terminalId}` : "terminal"];
  }
  return ["output", "errorText"];
}

/** Rendered beside a clipped step's badge. */
export function FullToolOutput({
  step,
  sessionId,
}: {
  step: FullOutputStep;
  sessionId?: string;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const retained =
    step.fullOutput === true &&
    step.turn !== undefined &&
    Boolean(step.toolCallId) &&
    Boolean(sessionId);
  if (!retained) {
    return <NotRetained />;
  }
  return (
    <div className="space-y-1">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="text-primary text-[11px] font-medium hover:underline"
      >
        {t(
          open
            ? "agentSessions.hideFullOutput"
            : "agentSessions.showFullOutput",
        )}
      </button>
      {open && (
        <FullOutputBody
          sessionId={sessionId!}
          turn={step.turn!}
          toolCallId={step.toolCallId!}
          fields={stepFields(step)}
        />
      )}
    </div>
  );
}

function NotRetained() {
  const { t } = useTranslations();
  return (
    <span className="text-muted-foreground ml-1.5 text-[11px]">
      {t("agentSessions.fullOutputNotRetained")}
    </span>
  );
}

function FullOutputBody({
  sessionId,
  turn,
  toolCallId,
  fields,
}: {
  sessionId: string;
  turn: number;
  toolCallId: string;
  fields: string[];
}) {
  const { t } = useTranslations();
  const { data, loading, error } = useQuery(AgentSessionToolOutputDocument, {
    variables: { sessionId, turn, toolCallId },
  });
  if (loading) {
    return (
      <p
        role="status"
        className="text-muted-foreground flex items-center gap-1.5 text-xs"
      >
        <Loader2 aria-hidden className="size-3 animate-spin" />
        {t("agentSessions.fullOutputLoading")}
      </p>
    );
  }
  const coded = error ? toAgentSessionError(error) : undefined;
  const notFound =
    coded instanceof AgentSessionError &&
    coded.code === "AGENT_SESSION_TOOL_OUTPUT_NOT_FOUND";
  if (error && !notFound) {
    return (
      <p className="text-destructive text-xs">
        {t("agentSessions.fullOutputError")}
      </p>
    );
  }
  const texts = (data?.agentSessionToolOutput.outputs ?? []).filter((output) =>
    fields.includes(output.field),
  );
  if (texts.length === 0) {
    return <NotRetained />;
  }
  return (
    <div className="space-y-1.5">
      {texts.map((output) => (
        <div key={output.field} className="relative">
          <pre
            data-full-output={output.field}
            className="border-border/60 bg-background text-muted-foreground max-h-96 overflow-auto rounded-md border px-2 py-1 pr-9 font-mono text-xs whitespace-pre-wrap"
          >
            {output.text}
          </pre>
          <div className="absolute top-1 right-1">
            <CopyButton
              value={output.text}
              label={t("agentSessions.fullOutputCopy")}
              successText={t("agentSessions.fullOutputCopied")}
              errorText={t("agentSessions.fullOutputCopyError")}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
