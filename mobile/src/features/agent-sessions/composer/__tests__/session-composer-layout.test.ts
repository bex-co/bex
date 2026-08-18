import fs from "node:fs";
import path from "node:path";

const source = fs.readFileSync(
  path.resolve(
    process.cwd(),
    "src/features/agent-sessions/composer/session-composer-screen.tsx",
  ),
  "utf8",
);

describe("agent-session compact composer layout", () => {
  it("keeps one keyboard-safe task surface ahead of the context controls", () => {
    expect(source.includes("<KeyboardAvoidingView")).toBe(true);
    const prompt = source.indexOf('testID="agent-session-prompt"');
    const repository = source.indexOf(
      'testID="agent-session-repository-select"',
    );
    const branch = source.indexOf('testID="agent-session-branch"');
    const agent = source.indexOf('testID="agent-session-agent-select"');
    expect(prompt > 0).toBe(true);
    expect(repository > prompt).toBe(true);
    expect(branch > repository).toBe(true);
    expect(agent > branch).toBe(true);
  });

  it("submits directly while retaining the shared action lifecycle", () => {
    expect(source.includes("<SafeActionPanel")).toBe(true);
    expect(source.includes('confirmationMode="server-only"')).toBe(true);
    expect(source.includes("feedbackContainerStyle=")).toBe(true);
    expect(source.includes("renderTrigger=")).toBe(true);
    expect(source.includes('testID="agent-session-composer-submit"')).toBe(
      true,
    );
  });

  it("opens the agent choice above the keyboard toolbar", () => {
    expect(source.includes('placement="above"')).toBe(true);
  });

  it("enters the chat window directly on create, with no intermediate step", () => {
    // A successful create pushes the session route and closes the composer —
    // the old "Session assigned / Open session" confirmation card is gone.
    expect(source.includes("router.push(`/sessions/${id}`)")).toBe(true);
    expect(source.includes("agentSessions.composer.openSession")).toBe(false);
    expect(source.includes("agentSessions.composer.submitted")).toBe(false);
  });
});
