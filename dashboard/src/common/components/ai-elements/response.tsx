import { cn } from "@/common/lib/utils/utils.ts";
import { MarkdownRenderer } from "@/common/components/markdown-renderer";

// The agent's natural-language answer (AI Elements `Response`). Upstream ships
// `streamdown`; the repo already has a sanitized markdown renderer
// (`MarkdownRenderer`, GFM + rehype-sanitize), so Response wraps it rather than
// adding a second markdown/syntax-highlight stack. Empty/whitespace content
// renders nothing so a still-streaming text part doesn't flash an empty block.

export function Response({
  children,
  className,
}: {
  children: string;
  className?: string;
}) {
  if (!children || !children.trim()) return null;
  return (
    <div className={cn("min-w-0", className)}>
      <MarkdownRenderer content={children} />
    </div>
  );
}
