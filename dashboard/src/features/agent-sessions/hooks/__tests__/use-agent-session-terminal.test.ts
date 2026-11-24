import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

import { useAgentSessionTerminal } from "@/features/agent-sessions/hooks/use-agent-session-terminal";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";
import { ShellUnavailableError } from "@/features/services/hooks/use-shell-session";

beforeEach(() => mockUseMutation.mockReset());

function minted(sandbox: string) {
  return vi.fn().mockResolvedValue({
    data: {
      createAgentSessionShellSession: {
        ticket: "tkt",
        url: "wss://ssh.bex.co/shell",
        expiresAt: "2026-11-23T00:00:00Z",
        sandbox,
      },
    },
  });
}

// w10/m615: the Terminal's ticket says whether the shell opens at once or
// after the sandbox starts, and its refusals keep their codes.
describe("useAgentSessionTerminal", () => {
  it.each([
    ["starting", true],
    ["up", false],
  ])("reads sandbox %s as starting=%s", async (sandbox, starting) => {
    const mutate = minted(sandbox);
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useAgentSessionTerminal());
    await expect(result.current.createTerminal("ags-1")).resolves.toEqual({
      ticket: "tkt",
      url: "wss://ssh.bex.co/shell",
      starting,
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "ags-1" },
      fetchPolicy: "no-cache",
    });
  });

  it("reports an unconfigured browser terminal as ShellUnavailableError", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(codedGraphQLError("SHELL_UNAVAILABLE", {})),
    ]);
    const { result } = renderHook(() => useAgentSessionTerminal());
    await expect(result.current.createTerminal("ags-1")).rejects.toBeInstanceOf(
      ShellUnavailableError,
    );
  });

  it("keeps a refusal's agent-session code and params", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(
        codedGraphQLError("AGENT_SESSION_NOT_ATTACHABLE", {
          phase: "completed",
          sandbox: "closed",
        }),
      ),
    ]);
    const { result } = renderHook(() => useAgentSessionTerminal());
    const err = await result.current.createTerminal("ags-1").catch((e) => e);
    expect(err).toBeInstanceOf(AgentSessionError);
    expect(err.code).toBe("AGENT_SESSION_NOT_ATTACHABLE");
    expect(err.params.sandbox).toBe("closed");
  });
});
