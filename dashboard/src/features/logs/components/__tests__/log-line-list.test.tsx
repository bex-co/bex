import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LogLineList } from "../log-line-list";
import type { LogLine } from "../../types";

function line(over: Partial<LogLine> = {}): LogLine {
  return {
    key: over.key ?? "k",
    timestamp: "2026-07-05T10:36:01.709Z",
    time: "10:36:01",
    instance: "bv612",
    message: "hello world",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    ...over,
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
});
