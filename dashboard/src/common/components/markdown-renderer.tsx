import { memo } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkBreaks from "remark-breaks";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import { CodeBlock } from "@/common/components/code-block";
import { classifyHref } from "@/common/lib/external-url";
import { ExternalLink } from "lucide-react";
import type { Components } from "react-markdown";

interface MarkdownRendererProps {
  content: string;
  className?: string;
}

const components: Components = {
  code({ className, children }) {
    const match = /language-(\w+)/.exec(className || "");
    const language = match ? match[1] : undefined;
    const code = String(children).replace(/\n$/, "");
    const inline = !className || !className.startsWith("language-");

    return <CodeBlock code={code} language={language} inline={inline} />;
  },

  a({ children, href, ...props }) {
    const { safeHref, isExternal } = classifyHref(href);
    if (!safeHref) {
      // Refused destination (e.g. javascript:/data:) — keep the text, drop the link.
      return <span {...props}>{children}</span>;
    }
    return (
      <a
        href={safeHref}
        target={isExternal ? "_blank" : undefined}
        rel={isExternal ? "noopener noreferrer" : undefined}
        className="underline hover:opacity-80 inline-flex items-center gap-1 font-medium"
        {...props}
      >
        {children}
        {isExternal && <ExternalLink className="h-3 w-3" />}
      </a>
    );
  },

  table({ children, ...props }) {
    return (
      <div className="overflow-x-auto my-4 max-w-full">
        <table className="min-w-full border-collapse" {...props}>
          {children}
        </table>
      </div>
    );
  },

  th({ children, ...props }) {
    return (
      <th
        className="border border-border px-4 py-2 bg-muted font-semibold text-left"
        {...props}
      >
        {children}
      </th>
    );
  },

  td({ children, ...props }) {
    return (
      <td className="border border-border px-4 py-2 text-left" {...props}>
        {children}
      </td>
    );
  },

  blockquote({ children, ...props }) {
    return (
      <blockquote
        className="border-l-4 border-current/20 pl-4 my-4 italic opacity-80"
        {...props}
      >
        {children}
      </blockquote>
    );
  },

  ul({ children, ...props }) {
    return (
      <ul className="list-disc ml-6 my-3 space-y-1" {...props}>
        {children}
      </ul>
    );
  },

  ol({ children, ...props }) {
    return (
      <ol className="list-decimal ml-6 my-3 space-y-1" {...props}>
        {children}
      </ol>
    );
  },

  h1({ children, ...props }) {
    return (
      <h1 className="text-2xl font-semibold mt-6 mb-3" {...props}>
        {children}
      </h1>
    );
  },

  h2({ children, ...props }) {
    return (
      <h2 className="text-xl font-semibold mt-6 mb-3" {...props}>
        {children}
      </h2>
    );
  },

  h3({ children, ...props }) {
    return (
      <h3 className="text-lg font-semibold mt-6 mb-3" {...props}>
        {children}
      </h3>
    );
  },

  p({ children, ...props }) {
    return (
      <p className="my-3 leading-relaxed" {...props}>
        {children}
      </p>
    );
  },
};

// Memoized on its props (content + className): rich Markdown parsing is the
// dominant per-block cost when a long transcript re-renders during streaming, so
// a block whose text has not changed must not re-parse (scan finding #9).
export const MarkdownRenderer = memo(function MarkdownRenderer({
  content,
  className,
}: MarkdownRendererProps) {
  return (
    <div
      className={`prose prose-sm dark:prose-invert max-w-full overflow-x-auto break-words ${className ?? ""}`}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkBreaks]}
        rehypePlugins={[rehypeRaw, rehypeSanitize]}
        components={components}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
});
