import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("@/common/components/code-block", () => ({
  CodeBlock: ({
    code,
    language,
    inline,
  }: {
    code: string;
    language?: string;
    inline?: boolean;
  }) => (
    <span
      data-testid="code-block"
      data-language={language}
      data-inline={inline}
    >
      {code}
    </span>
  ),
}));

vi.mock("lucide-react", () => ({
  ExternalLink: () => <span data-testid="external-link-icon" />,
}));

import { MarkdownRenderer } from "../markdown-renderer";

describe("MarkdownRenderer", () => {
  it("renders plain text", () => {
    render(<MarkdownRenderer content="Hello world" />);
    expect(screen.getByText("Hello world")).toBeInTheDocument();
  });

  it("renders headings", () => {
    render(
      <MarkdownRenderer
        content={`# Heading 1\n\n## Heading 2\n\n### Heading 3`}
      />,
    );
    expect(screen.getByText("Heading 1")).toBeInTheDocument();
    expect(screen.getByText("Heading 2")).toBeInTheDocument();
    expect(screen.getByText("Heading 3")).toBeInTheDocument();
  });

  it("renders links with external icon for http links", () => {
    render(<MarkdownRenderer content="[Example](https://example.com)" />);
    const link = screen.getByText("Example");
    expect(link).toBeInTheDocument();
    expect(link.closest("a")).toHaveAttribute("href", "https://example.com/");
    expect(link.closest("a")).toHaveAttribute("target", "_blank");
    expect(screen.getByTestId("external-link-icon")).toBeInTheDocument();
  });

  it("renders tables", () => {
    const tableMarkdown = `| Col A | Col B |
| --- | --- |
| A1 | B1 |`;
    render(<MarkdownRenderer content={tableMarkdown} />);
    expect(screen.getByText("Col A")).toBeInTheDocument();
    expect(screen.getByText("A1")).toBeInTheDocument();
  });

  it("renders unordered lists", () => {
    render(<MarkdownRenderer content={`- Item 1\n- Item 2`} />);
    expect(screen.getByText("Item 1")).toBeInTheDocument();
    expect(screen.getByText("Item 2")).toBeInTheDocument();
  });

  it("renders blockquotes", () => {
    render(<MarkdownRenderer content="> This is a quote" />);
    expect(screen.getByText("This is a quote")).toBeInTheDocument();
  });

  it("renders inline code via CodeBlock", () => {
    render(<MarkdownRenderer content="Use `npm install` to install" />);
    expect(screen.getByTestId("code-block")).toHaveTextContent("npm install");
  });

  it("applies custom className", () => {
    const { container } = render(
      <MarkdownRenderer content="Test" className="custom-class" />,
    );
    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("has prose classes by default", () => {
    const { container } = render(<MarkdownRenderer content="Test" />);
    expect(container.firstChild).toHaveClass("prose");
    expect(container.firstChild).toHaveClass("dark:prose-invert");
  });

  it("renders bash code block with # comments as plain text, not [object Object]", () => {
    const markdown = "```bash\n# Install dependencies\nnpm install\n```";
    render(<MarkdownRenderer content={markdown} />);
    const codeBlock = screen.getByTestId("code-block");
    expect(codeBlock).toHaveTextContent("# Install dependencies");
    expect(codeBlock.textContent).not.toContain("[object Object]");
  });

  it("sanitizes script tags in markdown content", () => {
    const markdown = '<script>alert("xss")</script>\n\nSafe content';
    const { container } = render(<MarkdownRenderer content={markdown} />);
    expect(container.querySelector("script")).toBeNull();
    expect(screen.getByText("Safe content")).toBeInTheDocument();
  });

  it("strips event handlers and SVG from raw HTML", () => {
    const markdown =
      '<img src="https://example.com/image.png" onerror="alert(1)">' +
      '<svg onload="alert(2)"><circle /></svg>';
    const { container } = render(<MarkdownRenderer content={markdown} />);

    const image = container.querySelector("img");
    expect(image).not.toBeNull();
    expect(image).toHaveAttribute("src", "https://example.com/image.png");
    expect(image).not.toHaveAttribute("onerror");
    expect(container.querySelector("svg")).toBeNull();
  });

  it("treats a protocol-relative link as external, not internal", () => {
    render(<MarkdownRenderer content="[Login](//attacker.example/login)" />);
    const anchor = screen.getByText("Login").closest("a");
    expect(anchor).not.toBeNull();
    expect(anchor).toHaveAttribute("target", "_blank");
    expect(anchor).toHaveAttribute("rel", "noopener noreferrer");
    // Resolved to an absolute origin — never navigates the current tab as a
    // silent "internal" relative link.
    expect(anchor?.getAttribute("href")).toMatch(
      /^https?:\/\/attacker\.example\//,
    );
  });
});
