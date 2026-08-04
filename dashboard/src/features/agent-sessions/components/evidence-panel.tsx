import { AlertTriangle } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { CodeBlock } from "@/common/components/code-block";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentSessionEvidenceView } from "@/features/agent-sessions/types";

// Client-side display caps (ADR047 D4). The driver already bounds the evidence
// server-side; these mirror the caps so the panel is honest even against an
// over-large payload and never renders an unbounded blob. Any cap that drops
// content flips the local truncation flag alongside the wire `truncated`.
const MAX_COMMAND_LOG = 40;
const MAX_TEST_OUTPUT = 40;
const MAX_OUTPUT_TAIL_BYTES = 8 * 1024;
const MAX_CHANGED_FILES = 100;

/** Cap an array to `max`, reporting whether anything was dropped. */
function capList(
  items: string[],
  max: number,
): { items: string[]; dropped: boolean } {
  if (items.length <= max) return { items, dropped: false };
  return { items: items.slice(0, max), dropped: true };
}

/** Cap a string to `maxBytes` (UTF-16 length as a cheap proxy), keeping the tail. */
function capText(
  text: string,
  maxBytes: number,
): { text: string; dropped: boolean } {
  if (text.length <= maxBytes) return { text, dropped: false };
  return { text: text.slice(text.length - maxBytes), dropped: true };
}

export interface EvidencePanelProps {
  evidence: AgentSessionEvidenceView | null;
}

/**
 * The bounded evidence summary (ADR047 D4): command log, test output, an output
 * tail, and the changed-file list — each capped for display with an explicit
 * truncation note. The transcript column is the primary narrative; this is the
 * durable, verifiable digest beside it. Truncation is surfaced (never silent):
 * the wire `truncated` flag OR any local cap dropping content shows the note.
 */
export function EvidencePanel({ evidence }: EvidencePanelProps) {
  const { t } = useTranslations();

  if (!evidence) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            {t("agentSessions.evidenceTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-sm">
            {t("agentSessions.evidenceEmpty")}
          </p>
        </CardContent>
      </Card>
    );
  }

  const commandLog = capList(evidence.commandLog, MAX_COMMAND_LOG);
  const testOutput = capList(evidence.testOutput, MAX_TEST_OUTPUT);
  const changedFiles = capList(evidence.changedFiles, MAX_CHANGED_FILES);
  const outputTail = evidence.outputTail
    ? capText(evidence.outputTail, MAX_OUTPUT_TAIL_BYTES)
    : null;

  const truncated =
    evidence.truncated ||
    commandLog.dropped ||
    testOutput.dropped ||
    changedFiles.dropped ||
    (outputTail?.dropped ?? false);

  const isEmpty =
    commandLog.items.length === 0 &&
    testOutput.items.length === 0 &&
    changedFiles.items.length === 0 &&
    !outputTail &&
    evidence.commits == null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {t("agentSessions.evidenceTitle")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {isEmpty ? (
          <p className="text-muted-foreground text-sm">
            {t("agentSessions.evidenceEmpty")}
          </p>
        ) : null}

        {evidence.commits != null ? (
          <p className="text-muted-foreground text-sm">
            {t("agentSessions.evidenceCommits", { count: evidence.commits })}
          </p>
        ) : null}

        {commandLog.items.length > 0 ? (
          <EvidenceSection label={t("agentSessions.evidenceCommandLog")}>
            <CodeBlock code={commandLog.items.join("\n")} language="bash" />
          </EvidenceSection>
        ) : null}

        {testOutput.items.length > 0 ? (
          <EvidenceSection label={t("agentSessions.evidenceTestOutput")}>
            <CodeBlock code={testOutput.items.join("\n")} language="text" />
          </EvidenceSection>
        ) : null}

        {outputTail ? (
          <EvidenceSection label={t("agentSessions.evidenceOutputTail")}>
            <CodeBlock code={outputTail.text} language="text" />
          </EvidenceSection>
        ) : null}

        {changedFiles.items.length > 0 ? (
          <EvidenceSection label={t("agentSessions.evidenceChangedFiles")}>
            <ul className="space-y-0.5 font-mono text-xs">
              {changedFiles.items.map((file) => (
                <li key={file} className="truncate" title={file}>
                  {file}
                </li>
              ))}
            </ul>
          </EvidenceSection>
        ) : null}

        {truncated ? (
          <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
            <AlertTriangle className="size-3.5 shrink-0" />
            {t("agentSessions.evidenceTruncated")}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}

function EvidenceSection({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <p className="text-muted-foreground text-xs font-medium">{label}</p>
      {children}
    </div>
  );
}
