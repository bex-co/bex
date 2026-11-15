import { MarkdownRenderer } from "@/common/components/markdown-renderer";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from "@/common/components/ai-elements/reasoning";
import {
  formatStreamDuration,
  useStreamDuration,
} from "@/features/agent-sessions/lib/stream-duration";

// The agent's chain-of-thought, rendered with the vendored AI Elements
// `Reasoning` disclosure. Collapsed by default so a long transcript stays
// scannable; the "Thought for <Ns>" label prefers persisted source timestamps.
export function ReasoningBlock({
  text,
  streaming,
  sourceTimesMs,
  ...props
}: {
  text: string;
  streaming: boolean;
  sourceTimesMs: readonly number[];
  className?: string;
}) {
  const { t } = useTranslations();
  const duration = useStreamDuration(text.length, !streaming, sourceTimesMs);
  const summary =
    duration.ms >= 1000
      ? t("agentSessions.groupThoughtFor", {
          duration: formatStreamDuration(duration),
        })
      : t("agentSessions.groupThought");
  return (
    <Reasoning {...props}>
      <ReasoningTrigger>{summary}</ReasoningTrigger>
      <ReasoningContent>
        <MarkdownRenderer content={text} />
      </ReasoningContent>
    </Reasoning>
  );
}
