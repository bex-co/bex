import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HealthCheckPathRow } from "@/features/services/components/health-check-path-row";

const setHealthCheckPath = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-health-check-path", () => ({
  useHealthCheckPath: () => ({ setHealthCheckPath, busy: false }),
}));

beforeEach(() => {
  setHealthCheckPath.mockClear();
  setHealthCheckPath.mockResolvedValue(true);
});

describe("HealthCheckPathRow", () => {
  it("persists an explicit path unchanged", async () => {
    const user = userEvent.setup();
    render(<HealthCheckPathRow serviceId="web" healthCheckPath="/healthz" />);

    await user.click(
      screen.getByRole("button", { name: "Edit Health Check Path" }),
    );
    const input = screen.getByRole("textbox", { name: "Health Check Path" });
    await user.clear(input);
    await user.type(input, "/ready");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    expect(setHealthCheckPath).toHaveBeenCalledWith("web", "/ready");
  });

  // The regression this row shipped with: it coerced an empty draft to "/"
  // before calling the mutation, so clearing the field was impossible from the
  // dashboard and the TCP check — the platform default since w7/m80, and the
  // only mode that works for a service whose "/" is a 404 — was unreachable
  // here even once the API supported it.
  it("clears the path instead of coercing it back to /", async () => {
    const user = userEvent.setup();
    render(<HealthCheckPathRow serviceId="web" healthCheckPath="/healthz" />);

    await user.click(
      screen.getByRole("button", { name: "Edit Health Check Path" }),
    );
    await user.clear(screen.getByRole("textbox", { name: "Health Check Path" }));
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    expect(setHealthCheckPath).toHaveBeenCalledWith("web", "");
  });

  it("renders an absent path as empty rather than as /", () => {
    render(<HealthCheckPathRow serviceId="web" healthCheckPath={null} />);

    expect(
      screen.getByRole("textbox", { name: "Health Check Path" }),
    ).toHaveValue("");
  });
});
