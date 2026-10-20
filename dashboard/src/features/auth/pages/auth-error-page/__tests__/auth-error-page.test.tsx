import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import AuthErrorPage from "@/features/auth/pages/auth-error-page";
import enAuth from "@/features/auth/locales/en";

const getFlowError = vi.hoisted(() => vi.fn());
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => (
    <a href={to}>{children}</a>
  ),
}));
vi.mock("@/common/lib/ory/frontend", () => ({
  createFrontendApi: () => ({ getFlowError }),
}));

const GENERIC = enAuth["auth.errorGeneric"].message;

beforeEach(() => {
  getFlowError.mockReset();
});

// w5/161: Kratos sends every self-service error to /auth/error?id=…; the page
// says what went wrong and leads back to sign in, never Not Found.
describe("AuthErrorPage", () => {
  it("shows the reason Kratos recorded for the error id", async () => {
    getFlowError.mockResolvedValue({
      id: "err-1",
      error: {
        code: 400,
        message: "The request was malformed or contained invalid parameters",
        reason: "The flow expired 3.2 minutes ago, please request a new one.",
      },
    });
    render(<AuthErrorPage id="err-1" />);
    expect(
      await screen.findByText(
        "The flow expired 3.2 minutes ago, please request a new one.",
      ),
    ).toBeInTheDocument();
    expect(getFlowError).toHaveBeenCalledWith({ id: "err-1" });
    expect(
      screen.getByRole("link", { name: "Back to sign in" }),
    ).toHaveAttribute("href", "/auth/login");
  });

  it("explains a known Kratos error id in the reader's language", async () => {
    getFlowError.mockResolvedValue({
      id: "err-2",
      error: {
        id: "self_service_flow_return_to_forbidden",
        reason:
          'Requested return_to URL "https://evil.example.com/" is not allowed.',
      },
    });
    render(<AuthErrorPage id="err-2" />);
    expect(
      await screen.findByText(enAuth["auth.errorReturnToForbidden"].message),
    ).toBeInTheDocument();
    expect(screen.queryByText(/evil\.example\.com/)).toBeNull();
  });

  it("shows no message, generic or otherwise, until Kratos answers", () => {
    getFlowError.mockReturnValue(new Promise(() => undefined));
    render(<AuthErrorPage id="pending" />);
    expect(screen.queryByText(GENERIC)).toBeNull();
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("falls back to the generic message for an unknown id", async () => {
    getFlowError.mockRejectedValue(new Error("404"));
    render(<AuthErrorPage id="gone" />);
    expect(await screen.findByText(GENERIC)).toBeInTheDocument();
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("asks Kratos nothing without an id", () => {
    render(<AuthErrorPage />);
    expect(screen.getByText(GENERIC)).toBeInTheDocument();
    expect(getFlowError).not.toHaveBeenCalled();
  });
});
