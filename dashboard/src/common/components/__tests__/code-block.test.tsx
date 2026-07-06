import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { CodeBlock } from "../code-block";

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

describe("CodeBlock", () => {
  beforeEach(() => {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      writable: true,
      configurable: true,
    });
  });

  it("renders inline code when inline prop is true", () => {
    const { container } = render(<CodeBlock code="const x = 1" inline />);
    const code = container.querySelector("code");
    expect(code).toBeInTheDocument();
    expect(code).toHaveTextContent("const x = 1");
    // No pre element in inline mode
    expect(container.querySelector("pre")).not.toBeInTheDocument();
  });

  it("renders block code when inline is false", () => {
    const { container } = render(<CodeBlock code="const x = 1" />);
    expect(container.querySelector("pre")).toBeInTheDocument();
    expect(container.querySelector("code")).toHaveTextContent("const x = 1");
  });

  it("shows language label when language prop is provided", () => {
    render(<CodeBlock code="const x = 1" language="typescript" />);
    expect(screen.getByText("typescript")).toBeInTheDocument();
  });

  it("shows copy button in block mode", () => {
    render(<CodeBlock code="const x = 1" />);
    expect(
      screen.getByRole("button", { name: /copy code/i }),
    ).toBeInTheDocument();
  });

  it("calls clipboard.writeText and shows success toast when copy button is clicked", async () => {
    const { toast } = await import("sonner");
    render(<CodeBlock code="const x = 1" />);
    fireEvent.click(screen.getByRole("button", { name: /copy code/i }));
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith("const x = 1");
      expect(toast.success).toHaveBeenCalledWith("Copied to clipboard");
    });
  });

  it("shows Check icon after copy", async () => {
    const { container } = render(<CodeBlock code="const x = 1" />);
    fireEvent.click(container.querySelector('[aria-label="Copy code"]')!);
    await waitFor(() => {
      const checkIcon = container.querySelector(".lucide-check");
      expect(checkIcon).toBeInTheDocument();
    });
  });
});
