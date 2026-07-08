import { Check, Copy } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";

interface CodeBlockProps {
  code: string;
  language?: string;
  inline?: boolean;
}

export function CodeBlock({ code, language, inline }: CodeBlockProps) {
  const { copied, copy } = useCopyToClipboard({
    successText: "Copied to clipboard",
    errorText: "Failed to copy",
  });
  const handleCopy = () => void copy(code);

  // Inline code
  if (inline) {
    return (
      <code className="bg-muted px-1.5 py-0.5 rounded text-sm font-mono">
        {code}
      </code>
    );
  }

  // Block code
  return (
    <div className="relative group my-4 max-w-full">
      <div className="flex items-center justify-between bg-muted px-4 py-2 rounded-t-lg border border-border">
        {language && (
          <span className="text-xs text-muted-foreground font-mono uppercase">
            {language}
          </span>
        )}
        {!language && <div />}
        <Button
          variant="ghost"
          size="icon"
          onClick={handleCopy}
          className="h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity"
          aria-label="Copy code"
        >
          {copied ? (
            <Check className="h-4 w-4 text-green-600" />
          ) : (
            <Copy className="h-4 w-4" />
          )}
        </Button>
      </div>
      <pre className="overflow-x-auto p-4 rounded-b-lg bg-muted border border-t-0 border-border">
        <code className={language ? `language-${language}` : undefined}>
          {code}
        </code>
      </pre>
    </div>
  );
}
