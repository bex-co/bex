import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WebhookEventPickerField } from "@/features/webhooks/components/webhook-event-picker-field";

const base = {
  retry: vi.fn().mockResolvedValue(undefined),
  value: new Set<string>(),
  onChange: vi.fn(),
};

describe("WebhookEventPickerField", () => {
  it("distinguishes loading, failure with retry, and an empty catalog", async () => {
    const user = userEvent.setup();
    const view = render(
      <WebhookEventPickerField
        {...base}
        eventTypes={[]}
        loading={true}
        error={undefined}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Loading event types");

    view.rerender(
      <WebhookEventPickerField
        {...base}
        eventTypes={[]}
        loading={false}
        error={new Error("offline")}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Event types couldn't be loaded",
    );
    await user.click(screen.getByRole("button", { name: "Retry event types" }));
    expect(base.retry).toHaveBeenCalledTimes(1);

    view.rerender(
      <WebhookEventPickerField
        {...base}
        eventTypes={[]}
        loading={false}
        error={undefined}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "No subscribable event types",
    );
  });

  it("restores the served picker after retry without owning form selection", () => {
    const selected = new Set(["deploy_started"]);
    render(
      <WebhookEventPickerField
        {...base}
        eventTypes={["deploy_started", "deploy_ended"]}
        loading={false}
        error={undefined}
        value={selected}
      />,
    );
    expect(
      screen.getByRole("checkbox", { name: "Deploy Started" }),
    ).toBeChecked();
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      "1 event selected",
    );
  });
});
