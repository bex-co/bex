import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ActivityGroup } from "../activity-group";

describe("native tool activity", () => {
  it.each([{}, { ok: true }, { success: true }, { status: "ok" }, true, null])(
    "omits a bare success acknowledgement %j",
    async (output) => {
      const { container } = render(
        <ActivityGroup
          steps={[
            {
              kind: "tool",
              name: "List files",
              state: "output-available",
              input: { command: "ls" },
              output,
            },
          ]}
        />,
      );
      await userEvent.click(screen.getByRole("button"));
      expect(screen.getByText("List files")).toBeInTheDocument();
      expect(screen.getByText("ls")).toBeInTheDocument();
      expect(container.querySelector("pre")).toBeNull();
    },
  );

  it.each([{ hits: 2 }, { ok: false }, { ok: true, extra: 1 }, "details", []])(
    "keeps informative output %j and errors",
    async (output) => {
      const { container } = render(
        <ActivityGroup
          steps={[
            {
              kind: "tool",
              name: "Search",
              state: "output-error",
              input: { q: "needle" },
              output,
              errorText: "Search failed",
            },
          ]}
        />,
      );
      await userEvent.click(screen.getByRole("button"));
      expect(screen.getByText("Search failed")).toBeInTheDocument();
      expect(
        Array.from(
          container.querySelectorAll("pre"),
          (node) => node.textContent,
        ),
      ).toEqual([
        JSON.stringify({ q: "needle" }, null, 2),
        // A string output renders verbatim, not JSON-quoted.
        typeof output === "string" ? output : JSON.stringify(output, null, 2),
      ]);
    },
  );
});

describe("signal-first summary line (w10/m16 t005)", () => {
  const done = "output-available";
  it("groups consecutive calls by kind in one collapsed line", () => {
    render(
      <ActivityGroup
        steps={[
          // The backend replay shape: only the final tool-input-available.
          {
            kind: "tool",
            toolCallId: "a",
            name: "Terminal",
            state: done,
            input: { command: "ls" },
          },
          {
            kind: "tool",
            toolCallId: "b",
            name: "cd /workspace && go test ./...",
            state: done,
            input: { command: "go test ./..." },
          },
          {
            kind: "tool",
            toolCallId: "c",
            name: "Read lego/types/tiers.go",
            state: done,
            input: { file_path: "lego/types/tiers.go" },
          },
          {
            kind: "tool",
            toolCallId: "d",
            name: "Edit",
            state: done,
            input: { file_path: "a.go", old_string: "x", new_string: "y" },
          },
          // An ACP call plus its diff content counts once, as an edit.
          {
            kind: "tool",
            toolCallId: "e",
            name: "acp_agent",
            state: done,
            input: {},
          },
          {
            kind: "diff",
            toolCallId: "e",
            path: "b.go",
            oldText: "",
            newText: "z",
          },
          {
            kind: "tool",
            toolCallId: "f",
            name: "Task",
            state: done,
            input: { subagent_type: "general", prompt: "scan" },
          },
          {
            kind: "tool",
            toolCallId: "g",
            name: "mcp__bex__list_services",
            state: done,
            input: {},
          },
        ]}
      />,
    );
    expect(
      screen.getByText(
        "Ran 2 commands · Read 1 file · Edited 2 files · Ran 1 subagent · Used 1 tool",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button")).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    expect(screen.queryByText("go test ./...")).not.toBeInTheDocument();
  });

  it("surfaces a failed call without expanding, and once when expanded", async () => {
    render(
      <ActivityGroup
        steps={[
          {
            kind: "tool",
            toolCallId: "a",
            name: "Terminal",
            state: done,
            input: { command: "ls" },
          },
          {
            kind: "tool",
            toolCallId: "b",
            name: "Terminal",
            state: "output-error",
            input: { command: "go vet", description: "Vet the module" },
            errorText: "```\nExit code 1\nvet: bad\n```",
          },
        ]}
      />,
    );
    expect(screen.getByText(/1 failed/)).toBeInTheDocument();
    expect(screen.getByText("Vet the module")).toBeInTheDocument();
    expect(screen.getByText(/: Exit code 1/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button"));
    expect(screen.queryByText("Vet the module")).not.toBeInTheDocument();
    expect(screen.getByText(/vet: bad/)).toBeInTheDocument();
  });

  it("badges an output the server clipped", async () => {
    render(
      <ActivityGroup
        steps={[
          {
            kind: "tool",
            name: "Terminal",
            state: done,
            input: { command: "cat big.log" },
            output: "head…\n[bex: 41234 chars clipped]\n…tail",
          },
        ]}
      />,
    );
    await userEvent.click(screen.getByRole("button"));
    expect(screen.getByText("41,234 characters clipped")).toBeInTheDocument();
    expect(screen.getByText(/head…/)).toBeInTheDocument();
  });

  it("badges a structured output the server clipped string by string", async () => {
    // The backend clips every string inside an object output (ACP rawOutput).
    render(
      <ActivityGroup
        steps={[
          {
            kind: "tool",
            name: "Terminal",
            state: done,
            input: { command: "make test" },
            output: {
              stdout: "head\n\n[bex: 1200 chars clipped]\n\ntail",
              stderr: "a\n\n[bex: 34 chars clipped]\n\nb",
              exitCode: 0,
            },
          },
        ]}
      />,
    );
    await userEvent.click(screen.getByRole("button"));
    expect(screen.getByText("1,234 characters clipped")).toBeInTheDocument();
  });
});
