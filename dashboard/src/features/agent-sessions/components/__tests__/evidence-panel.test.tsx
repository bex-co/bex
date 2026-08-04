import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { EvidencePanel } from "@/features/agent-sessions/components/evidence-panel";
import type { AgentSessionEvidenceView } from "@/features/agent-sessions/types";

const TRUNCATION_NOTE = "Some captured output was truncated.";

function evidence(
  over: Partial<AgentSessionEvidenceView> = {},
): AgentSessionEvidenceView {
  return {
    commandLog: [],
    testOutput: [],
    outputTail: null,
    changedFiles: [],
    commits: null,
    truncated: false,
    ...over,
  };
}

describe("EvidencePanel", () => {
  it("shows the empty state when there is no evidence at all", () => {
    render(<EvidencePanel evidence={null} />);
    expect(screen.getByText("No evidence captured yet.")).toBeInTheDocument();
  });

  it("shows the empty state when evidence is present but every field is blank", () => {
    render(<EvidencePanel evidence={evidence()} />);
    expect(screen.getByText("No evidence captured yet.")).toBeInTheDocument();
    expect(screen.queryByText(TRUNCATION_NOTE)).not.toBeInTheDocument();
  });

  it("renders the commit count and the changed-files list", () => {
    render(
      <EvidencePanel
        evidence={evidence({
          commits: 3,
          changedFiles: ["src/a.ts", "src/b.ts"],
        })}
      />,
    );
    expect(screen.getByText("3 commits")).toBeInTheDocument();
    expect(screen.getByText("Changed files")).toBeInTheDocument();
    expect(screen.getByText("src/a.ts")).toBeInTheDocument();
    expect(screen.getByText("src/b.ts")).toBeInTheDocument();
  });

  it("shows the truncation note when the wire flag is set, even with tiny content", () => {
    render(
      <EvidencePanel
        evidence={evidence({ changedFiles: ["one.ts"], truncated: true })}
      />,
    );
    expect(screen.getByText(TRUNCATION_NOTE)).toBeInTheDocument();
  });

  it("shows the truncation note when a local display cap drops content (wire flag false)", () => {
    // 41 changed files > MAX_CHANGED_FILES (100? no — command log cap is 40).
    const overLimit = Array.from({ length: 41 }, (_, i) => `cmd-${i}`);
    render(
      <EvidencePanel
        evidence={evidence({ commandLog: overLimit, truncated: false })}
      />,
    );
    // Honest truncation: the note fires from the local cap even though the
    // backend did not set `truncated`.
    expect(screen.getByText(TRUNCATION_NOTE)).toBeInTheDocument();
  });

  it("does NOT show the truncation note when content is within every cap", () => {
    const withinCap = Array.from({ length: 40 }, (_, i) => `cmd-${i}`);
    render(
      <EvidencePanel
        evidence={evidence({ commandLog: withinCap, truncated: false })}
      />,
    );
    expect(screen.getByText("Command log")).toBeInTheDocument();
    expect(screen.queryByText(TRUNCATION_NOTE)).not.toBeInTheDocument();
  });
});
