import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { LogLineList } from "../log-line-list";
import { needsAnsiParse, parseAnsi } from "../../lib/ansi";
import type { LogLine } from "../../types";

function line(over: Partial<LogLine> = {}): LogLine {
  const message = over.message ?? "hello world";
  return {
    key: over.key ?? "k",
    timestamp: "2026-07-05T10:36:01.709Z",
    time: "10:36:01",
    instance: "bv612",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    ...over,
    message,
    // Mirror ingest (makeLogLine): ANSI parsed once, so the list reads
    // `line.spans` (w9/m63 t001).
    spans: needsAnsiParse(message) ? parseAnsi(message) : null,
  };
}

describe("LogLineList request-line rendering (w5/008)", () => {
  it("shows method + status chips for a request line", () => {
    render(
      <LogLineList
        lines={[
          line({
            key: "r1",
            type: "request",
            method: "GET",
            statusCode: "200",
            message: '{"RequestPath":"/health"}',
          }),
        ]}
      />,
    );
    expect(screen.getByText("GET")).toBeInTheDocument();
    expect(screen.getByText("200")).toBeInTheDocument();
    // The message is still rendered (honest — no data hidden).
    expect(screen.getByText('{"RequestPath":"/health"}')).toBeInTheDocument();
  });

  it("does not render method/status chips for an app line", () => {
    render(
      <LogLineList
        lines={[line({ key: "a1", type: "app", message: "starting up" })]}
      />,
    );
    expect(screen.getByText("starting up")).toBeInTheDocument();
    expect(screen.queryByText("GET")).not.toBeInTheDocument();
    expect(screen.queryByText("200")).not.toBeInTheDocument();
  });

  it("interprets a build line's ANSI instead of leaking the parameter tail", () => {
    const esc = "\u001b";
    const { container } = render(
      <LogLineList
        lines={[
          line({
            key: "b1",
            type: "build",
            message: `#11 94.34 ${esc}[2m│${esc}[22m   ${esc}[33m^--${esc}[39m oops`,
          }),
        ]}
      />,
    );

    // The reader sees the text, never `[2m` / `[22m` / `[33m`.
    expect(container.textContent).toContain("#11 94.34 │   ^-- oops");
    expect(container.textContent).not.toContain("[2m");
    expect(container.textContent).not.toContain("[22m");
    expect(container.textContent).not.toContain("[33m");
    expect(container.textContent).not.toContain(esc);

    expect(screen.getByText("│").className).toContain("opacity-70");
    expect(screen.getByText("^--").className).toContain("text-amber-700");
  });

  it("leaves a plain line as a single unstyled text node", () => {
    render(<LogLineList lines={[line({ message: "no escapes here" })]} />);
    const message = screen.getByText("no escapes here");
    expect(message.tagName).toBe("SPAN");
    expect(message.querySelector("span")).toBeNull();
  });

  it("shows a short clickable pod slug and filters with the full instance", () => {
    const onInstanceFilter = vi.fn();
    const instance = "hello-go-6f7d8f9c4b-bv612";
    render(
      <LogLineList
        lines={[line({ instance })]}
        onInstanceFilter={onInstanceFilter}
      />,
    );

    const instanceButton = screen.getByRole("button", {
      name: `Filter logs by instance ${instance}`,
    });
    expect(instanceButton).toHaveTextContent("[bv612]");
    expect(instanceButton).toHaveAttribute("title", instance);

    fireEvent.click(instanceButton);
    expect(onInstanceFilter).toHaveBeenCalledWith(instance);
  });
});
