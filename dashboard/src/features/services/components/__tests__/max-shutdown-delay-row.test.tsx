import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n, { ensureLanguage } from "@/i18n/init";
import { MaxShutdownDelayRow } from "@/features/services/components/max-shutdown-delay-row";

const setMaxShutdownDelay = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-max-shutdown-delay", () => ({
  useMaxShutdownDelay: () => ({ setMaxShutdownDelay, busy: false }),
}));

vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({
    to,
    params,
    children,
  }: {
    to: string;
    params: { serviceId: string };
    children: ReactNode;
  }) => <a href={to.replace("$serviceId", params.serviceId)}>{children}</a>,
}));

beforeEach(() => {
  setMaxShutdownDelay.mockClear();
  setMaxShutdownDelay.mockResolvedValue(true);
});

describe("MaxShutdownDelayRow", () => {
  it("shows the 30-second platform default when the field is absent", () => {
    render(
      <MaxShutdownDelayRow serviceId="web" maxShutdownDelaySeconds={null} />,
    );

    const input = screen.getByRole("spinbutton", {
      name: "Max shutdown delay",
    });
    expect(input).toHaveValue(30);
    expect(input).toBeDisabled();
    expect(screen.getByText(/1–300 seconds/)).toBeInTheDocument();
  });

  it("persists an in-range integer and refetches the service", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <MaxShutdownDelayRow
        serviceId="worker"
        maxShutdownDelaySeconds={45}
        onChanged={onChanged}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit max shutdown delay" }),
    );
    const input = screen.getByRole("spinbutton", {
      name: "Max shutdown delay",
    });
    await user.clear(input);
    await user.type(input, "120");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // The save starts a deploy, so it confirms first (w4/616).
    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText("Change the Max shutdown delay to 120 seconds?"),
    ).toBeInTheDocument();
    expect(setMaxShutdownDelay).not.toHaveBeenCalled();
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );

    expect(setMaxShutdownDelay).toHaveBeenCalledWith("worker", 120);
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("does not allow fractions or values outside 1-300", async () => {
    const user = userEvent.setup();
    render(
      <MaxShutdownDelayRow serviceId="web" maxShutdownDelaySeconds={30} />,
    );
    await user.click(
      screen.getByRole("button", { name: "Edit max shutdown delay" }),
    );
    const input = screen.getByRole("spinbutton", {
      name: "Max shutdown delay",
    });
    const save = screen.getByRole("button", { name: "Save changes" });

    for (const value of ["0", "301", "1.5"]) {
      await user.clear(input);
      await user.type(input, value);
      expect(save).toBeDisabled();
      // w4/494: Save is not simply greyed out; the rule is stated, announced
      // and linked to the input.
      const message = screen.getByRole("alert");
      expect(message).toHaveTextContent(
        "Enter a whole number of seconds from 1 to 300.",
      );
      expect(input).toHaveAccessibleDescription(
        expect.stringContaining("Enter a whole number of seconds"),
      );
    }
    expect(setMaxShutdownDelay).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, "60");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(save).toBeEnabled();
  });

  // w6/m548: Render does not let a service with a disk change its shutdown
  // delay, so the row reads the default it runs with and says why.
  it("is read-only at 30 on a service with a disk, with Render's reason", () => {
    render(
      <MaxShutdownDelayRow
        serviceId="web"
        maxShutdownDelaySeconds={120}
        hasDisk
      />,
    );

    const input = screen.getByRole("spinbutton", {
      name: "Max shutdown delay",
    });
    expect(input).toHaveValue(30);
    expect(input).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Edit max shutdown delay" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        /You can't change the shutdown delay for a service with a persistent disk\./,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View disk" })).toHaveAttribute(
      "href",
      "/services/web/disk",
    );
    expect(screen.queryByText(/1–300 seconds/)).not.toBeInTheDocument();
  });

  it.each([
    ["zh", "挂载了持久磁盘的服务无法更改关闭延迟。", "查看磁盘"],
    [
      "ja",
      "永続ディスクがあるサービスのシャットダウン遅延は変更できません。",
      "ディスクを表示",
    ],
  ])("says why in %s", async (lang, reason, link) => {
    await ensureLanguage(lang);
    await i18n.changeLanguage(lang);
    try {
      render(
        <MaxShutdownDelayRow
          serviceId="web"
          maxShutdownDelaySeconds={null}
          hasDisk
        />,
      );
      expect(screen.getByText(new RegExp(reason))).toBeInTheDocument();
      expect(screen.getByRole("link", { name: link })).toBeInTheDocument();
    } finally {
      await i18n.changeLanguage("en");
    }
  });
});
