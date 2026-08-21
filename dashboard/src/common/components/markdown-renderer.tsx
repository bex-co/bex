import { memo } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkBreaks from "remark-breaks";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, {
  type Options as SanitizeOptions,
} from "rehype-sanitize";
import { CodeBlock } from "@/common/components/code-block";
import { classifyHref } from "@/common/lib/external-url";
import { ExternalLink } from "lucide-react";
import type { Components } from "react-markdown";

interface MarkdownRendererProps {
  content: string;
  className?: string;
}

// Raw HTML is retained for Markdown compatibility, but it must pass through a
// schema owned by this component. Do not rely on rehype-sanitize's package
// default: a dependency upgrade should not silently expand the transcript's
// HTML vocabulary. This is intentionally a small, presentation-only subset;
// scripting, embedding, forms, SVG, and event/style attributes are excluded.
const markdownSanitizeSchema: SanitizeOptions = {
  tagNames: [
    "a",
    "b",
    "blockquote",
    "br",
    "code",
    "dd",
    "del",
    "details",
    "div",
    "dl",
    "dt",
    "em",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "hr",
    "i",
    "img",
    "input",
    "ins",
    "kbd",
    "li",
    "ol",
    "p",
    "picture",
    "pre",
    "q",
    "rp",
    "rt",
    "ruby",
    "s",
    "samp",
    "section",
    "source",
    "span",
    "strike",
    "strong",
    "sub",
    "summary",
    "sup",
    "table",
    "tbody",
    "td",
    "tfoot",
    "th",
    "thead",
    "tr",
    "tt",
    "ul",
    "var",
  ],
  attributes: {
    "*": ["dir", "id", "lang", "tabIndex", "title"],
    a: [
      "href",
      "title",
      "dataFootnoteBackref",
      "dataFootnoteRef",
      ["className", "data-footnote-backref"],
    ],
    blockquote: ["cite"],
    code: [["className", /^language-[A-Za-z0-9_-]+$/]],
    h2: [["className", "sr-only"]],
    img: ["alt", "height", "longDesc", "src", "title", "width"],
    input: [
      ["disabled", true],
      ["type", "checkbox"],
    ],
    li: [["className", "task-list-item"]],
    ol: ["start", ["className", "contains-task-list"]],
    section: ["dataFootnotes", ["className", "footnotes"]],
    source: ["srcSet"],
    td: ["colSpan", "rowSpan"],
    th: ["colSpan", "rowSpan"],
    ul: [["className", "contains-task-list"]],
  },
  clobber: ["id", "name"],
  clobberPrefix: "user-content-",
  protocols: {
    cite: ["http", "https"],
    href: ["http", "https", "mailto", "tel"],
    longDesc: ["http", "https"],
    src: ["http", "https"],
  },
  strip: [
    "base",
    "embed",
    "form",
    "iframe",
    "link",
    "meta",
    "object",
    "script",
    "style",
  ],
};

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
        rehypePlugins={[rehypeRaw, [rehypeSanitize, markdownSanitizeSchema]]}
        components={components}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
});
