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
        JSON.stringify(output, null, 2),
      ]);
    },
  );
});
