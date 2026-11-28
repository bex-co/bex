import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FailureCallout } from "@/features/agent-sessions/components/failure-callout";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";
import i18n from "@/i18n/init";

const steer = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ steer }),
}));

const navigate = vi.fn();
vi.mock("@tanstack/react-router", async (importActual) => ({
  ...(await importActual<typeof import("@tanstack/react-router")>()),
  useNavigate: () => navigate,
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (m: string) => toastSuccess(m),
    error: (m: string) => toastError(m),
  },
}));

beforeEach(() => {
  steer.mockReset();
  steer.mockResolvedValue({ session: { id: "as-1" } });
  toastSuccess.mockReset();
  toastError.mockReset();
  navigate.mockReset();
});

function view(
  phase: AgentSessionPhase,
  over: Partial<AgentSessionView> = {},
): AgentSessionView {
  return agentSessionView({ phase, task: "fix the failing tests", ...over });
}

describe("FailureCallout", () => {
  it("renders nothing for a non-failed session", () => {
    const { container } = render(<FailureCallout session={view("running")} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the recorded failureReason when the Completer named one", () => {
    render(
      <FailureCallout
        session={view("failed", { failureReason: "agent turn failed" })}
      />,
    );
    expect(screen.getByText("Session failed")).toBeInTheDocument();
    expect(screen.getByText("agent turn failed")).toBeInTheDocument();
  });

  it("names the session limit when the session's own ceiling stopped it (w21/005)", () => {
    const outcome = (modelUSD: string) => ({
      state: "failed",
      steerable: true,
      turn: 1,
      summary: null,
      deliveryNote: null,
      failureReasonCode: "AGENT_MODEL_SPEND_CAP_REACHED",
      result: null,
      cost: { modelUSD, computeUSD: null, totalUSD: null },
    });
    const { rerender } = render(
      <FailureCallout
        session={view("failed", {
          failureReasonCode: "AGENT_MODEL_SPEND_CAP_REACHED",
          maxSpendUSD: "5",
          outcome: outcome("5.01"),
        })}
      />,
    );
    expect(screen.getByText(/session limit of \$5\.00/)).toBeInTheDocument();
    // Under its own ceiling, the monthly workspace cap is what stopped it.
    rerender(
      <FailureCallout
        session={view("failed", {
          failureReasonCode: "AGENT_MODEL_SPEND_CAP_REACHED",
          maxSpendUSD: "5",
          outcome: outcome("1.20"),
        })}
      />,
    );
    expect(
      screen.getByText(/reached its agent model spend limit for the month/),
    ).toBeInTheDocument();
  });

  it("reads the model mint's spend refusals in the user's language (ADR091 D6)", () => {
    const { rerender } = render(
      <FailureCallout
        session={view("failed", {
          failureReason:
            "this workspace reached its monthly agent model spend cap (AGENT_MODEL_SPEND_CAP_REACHED); …",
          failureReasonCode: "AGENT_MODEL_SPEND_CAP_REACHED",
        })}
      />,
    );
    expect(
      screen.getByText(/reached its agent model spend limit for the month/),
    ).toBeInTheDocument();
    rerender(
      <FailureCallout
        session={view("failed", {
          failureReason: "a payment for this workspace failed; …",
          failureReasonCode: "BILLING_BALANCE_DUE",
        })}
      />,
    );
    expect(
      screen.getByText(/Pay the outstanding invoice in Billing/),
    ).toBeInTheDocument();
  });

  it("falls back to the lifecycle status for a background-provisioning failure", () => {
    // A dispatch failure stamps the reason on `status`, not `failureReason`.
    render(
      <FailureCallout
        session={view("failed", {
          failureReason: null,
          status: "sandbox create failed",
        })}
      />,
    );
    expect(screen.getByText("sandbox create failed")).toBeInTheDocument();
  });

  it("falls back to generic copy when the session carries no reason at all", () => {
    render(
      <FailureCallout
        session={view("failed", { failureReason: null, status: "" })}
      />,
    );
    expect(
      screen.getByText("The session failed to start."),
    ).toBeInTheDocument();
  });

  it("falls back to generic copy rather than showing an uninformative reason", () => {
    // Sessions that failed before the driver learned to describe a rejected
    // JSON-RPC error stored the literal "[object Object]"; `status` then only
    // restates the phase. Neither belongs under a heading that already reads
    // "Session failed".
    render(
      <FailureCallout
        session={view("failed", {
          failureReason: "[object Object]",
          status: "failed",
        })}
      />,
    );
    expect(screen.queryByText("[object Object]")).not.toBeInTheDocument();
    expect(screen.queryByText("failed")).not.toBeInTheDocument();
    expect(
      screen.getByText("The session failed to start."),
    ).toBeInTheDocument();
  });

  it("retries by re-running the original task through the steer mutation", async () => {
    const onRetried = vi.fn();
    const user = userEvent.setup();
    render(
      <FailureCallout
        session={view("failed", { status: "sandbox create failed" })}
        onRetried={onRetried}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "fix the failing tests"),
    );
    expect(toastSuccess).toHaveBeenCalled();
    expect(onRetried).toHaveBeenCalled();
  });

  it("offers an Upgrade plan CTA (Render's plan page on Pro) on a capacity failure", async () => {
    const user = userEvent.setup();
    // The code decides, whatever the sentence beside it says (w5/m132).
    render(
      <FailureCallout
        session={view("failed", {
          failureReason: "the workspace is at its sandbox limit",
          failureReasonCode: "SANDBOX_CAPACITY_LIMIT",
        })}
      />,
    );

    // Capacity-specific heading + copy replace the raw backend reason.
    expect(screen.getByText("Out of sandbox capacity")).toBeInTheDocument();
    expect(
      screen.getByText(/reached its plan's limit on concurrent sandboxes/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("the workspace is at its sandbox limit"),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Upgrade plan" }));
    // Render's plan page on Pro, coming back to the session (w3/m708).
    expect(navigate).toHaveBeenCalledWith({
      to: "/billing/update-plan",
      search: { plan: "professional", next: "/" },
    });
    // Retry stays for the "freed a sandbox" case.
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  // ADR109 D5: a failed setup step keeps the reason naming the step, and
  // leads to the repository environment where it is fixed.
  it("sends a failed setup step to the repository environment settings", async () => {
    const user = userEvent.setup();
    render(
      <FailureCallout
        session={view("failed", {
          failureReason: 'setup step "deps" exited with code 7',
          failureReasonCode: "AGENT_SESSION_SETUP_FAILED",
        })}
      />,
    );

    expect(screen.getByText("Repository setup failed")).toBeInTheDocument();
    expect(
      screen.getByText('setup step "deps" exited with code 7'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/the agent didn't start and no model spend accrued/),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Edit repository environment" }),
    );
    expect(navigate).toHaveBeenCalledWith({
      to: "/workspace/settings",
      hash: "repository-environments",
    });
    // Retry stays: it re-runs setup once the step is fixed.
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Upgrade plan" }),
    ).not.toBeInTheDocument();
  });

  it("offers the environment link only for a setup failure", () => {
    render(
      <FailureCallout
        session={view("failed", {
          failureReason: 'setup step "deps" exited with code 7',
        })}
      />,
    );
    expect(screen.getByText("Session failed")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit repository environment" }),
    ).not.toBeInTheDocument();
  });

  it("shows no Upgrade CTA for a non-capacity failure", () => {
    render(
      <FailureCallout
        session={view("failed", { status: "sandbox create failed" })}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Upgrade plan" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("shows no Upgrade CTA for a reason that only mentions sandbox capacity", () => {
    render(
      <FailureCallout
        session={view("failed", { failureReason: "sandbox capacity reached" })}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Upgrade plan" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("sandbox capacity reached")).toBeInTheDocument();
  });

  // w10/m497: a sandbox that did not start names its cause above the reason
  // bex-api wrote for it; a platform cause is never a plan limit.
  describe("a sandbox that did not start", () => {
    it.each([
      [
        "pool_full",
        "No sandbox machine had room for this session for 60 minutes, so it did not start. Send the message again to retry.",
        "No sandbox machine had room",
      ],
      [
        "image_pull",
        "The sandbox image could not be pulled, so the session did not start. Try again, and contact support if it keeps failing.",
        "The sandbox image could not be pulled",
      ],
      [
        "runtime",
        "The sandbox runtime could not start this session's machine. Try again, and contact support if it keeps failing.",
        "The sandbox runtime could not start",
      ],
      [
        "quota",
        "The workspace's sandbox quota refused this session's machine. Stop a running sandbox or session, then try again.",
        "The workspace's sandbox quota refused it",
      ],
      ["unknown", "sandbox did not start (platform issue)", "Platform issue"],
    ] as const)(
      "names the %s cause over its reason, with Retry and no upgrade",
      (cause, failureReason, line) => {
        render(
          <FailureCallout
            session={view("failed", {
              failureReason,
              failureReasonCode: "SANDBOX_START_FAILED",
              failureCause: cause,
            })}
          />,
        );
        expect(screen.getByText("Session failed")).toBeInTheDocument();
        const causeLine = screen.getByTestId("failure-cause");
        expect(causeLine).toHaveTextContent(line);
        expect(causeLine).toHaveAttribute("data-cause", cause);
        // The reason is shown as before, after the cause.
        const reason = screen.getByText(failureReason);
        expect(
          causeLine.compareDocumentPosition(reason) &
            Node.DOCUMENT_POSITION_FOLLOWING,
        ).toBeTruthy();
        expect(
          screen.queryByRole("button", { name: "Upgrade plan" }),
        ).not.toBeInTheDocument();
        expect(screen.queryByText("Out of sandbox capacity")).toBeNull();
        expect(
          screen.getByRole("button", { name: "Retry" }),
        ).toBeInTheDocument();
      },
    );

    it("reads a start failure recorded before causes as a platform issue", () => {
      render(
        <FailureCallout
          session={view("failed", {
            failureReason: "sandbox did not start (platform issue)",
            failureReasonCode: "SANDBOX_START_FAILED",
          })}
        />,
      );
      expect(screen.getByTestId("failure-cause")).toHaveTextContent(
        "Platform issue",
      );
      expect(
        screen.queryByRole("button", { name: "Upgrade plan" }),
      ).not.toBeInTheDocument();
    });

    it("draws no cause line for any other failure, the capacity limit included", () => {
      const { rerender } = render(
        <FailureCallout
          session={view("failed", { failureReason: "agent turn failed" })}
        />,
      );
      expect(screen.queryByTestId("failure-cause")).toBeNull();
      // A cause on a code other than SANDBOX_START_FAILED is not one.
      rerender(
        <FailureCallout
          session={view("failed", {
            failureReasonCode: "SANDBOX_CAPACITY_LIMIT",
            failureCause: "pool_full",
          })}
        />,
      );
      expect(screen.queryByTestId("failure-cause")).toBeNull();
      expect(screen.getByText("Out of sandbox capacity")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Upgrade plan" }),
      ).toBeInTheDocument();
    });

    it("names the cause in the viewer's language", async () => {
      await i18n.changeLanguage("zh");
      try {
        render(
          <FailureCallout
            session={view("failed", {
              failureReasonCode: "SANDBOX_START_FAILED",
              failureCause: "image_pull",
            })}
          />,
        );
        expect(screen.getByTestId("failure-cause")).toHaveTextContent(
          "无法拉取沙箱镜像",
        );
      } finally {
        await i18n.changeLanguage("en");
      }
    });
  });

  it("toasts an error and does not converge when the retry is rejected", async () => {
    steer.mockRejectedValue(new Error("boom"));
    const onRetried = vi.fn();
    const user = userEvent.setup();
    render(
      <FailureCallout
        session={view("failed", { status: "sandbox create failed" })}
        onRetried={onRetried}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(onRetried).not.toHaveBeenCalled();
  });
});
