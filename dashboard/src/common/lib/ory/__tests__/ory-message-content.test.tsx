import type { ComponentType } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { IntlProvider } from "react-intl";
import type { UiText } from "@ory/client-fetch";
import {
  OryLocales,
  type OryMessageContentProps,
  type OryToastProps,
} from "@ory/elements-react";
import i18n, { ensureLanguage } from "@/i18n/init";
import { orySettingsFormOverrides } from "../auth-form-overrides";

vi.mock("sonner", () => ({ toast: { dismiss: vi.fn() } }));

/** What bex-api's settings guard sends Kratos for an email change
 * (lego/backend/internal/settingsguard): attached to `traits.email`. */
const EMAIL_REFUSED: UiText = {
  id: 4100001,
  type: "error",
  text: "You can't change your email address while you belong to a workspace that requires Google login. Leave these workspaces first: Acme, Beta.",
  context: { workspaces: ["Acme", "Beta"] },
};

/** ...and for an unlink: a flow-level message, which the settings flow toasts. */
const UNLINK_REFUSED: UiText = {
  id: 4100002,
  type: "error",
  text: "You can't disconnect your Google account while you belong to a workspace that requires Google login. Leave these workspaces first: Acme.",
  context: { workspaces: ["Acme"] },
};

// The settings flow's own overrides, which the page passes to Ory's
// <Settings>: Ory's label renders a field's messages through Message.Content,
// and its settings card toasts the flow's through Message.Toast.
const Content = orySettingsFormOverrides.Message!
  .Content as ComponentType<OryMessageContentProps>;
const Toast = orySettingsFormOverrides.Message!
  .Toast as ComponentType<OryToastProps>;

function inOry(ui: React.ReactElement, locale = "en") {
  return render(
    <IntlProvider
      locale={locale}
      defaultLocale="en"
      messages={OryLocales[locale as keyof typeof OryLocales]}
    >
      {ui}
    </IntlProvider>,
  );
}

afterEach(async () => {
  await i18n.changeLanguage("en");
});

// w7/m465: Kratos leaves the 41xxxxx ids to web hooks, so Ory Elements has no
// translation for the guard's refusals and would show its English text in
// every language.
describe("the settings guard's refusals in account settings (w7/m465)", () => {
  it("words the email-change refusal under the field, naming the workspaces to leave", () => {
    inOry(<Content message={EMAIL_REFUSED} />);

    const message = screen.getByTestId("ory/message/4100001");
    expect(message).toHaveTextContent(
      "You can't change your email address while you belong to a workspace that requires Google login. Leave these workspaces first: Acme and Beta.",
    );
    // Ory's own error styling, as its default message has.
    expect(message).toHaveClass(
      "leading-normal",
      "text-interface-foreground-validation-danger",
    );
  });

  it("words the disconnect refusal in the settings flow's toast", () => {
    inOry(<Toast id="t1" message={UNLINK_REFUSED} />);

    const toast = screen.getByTestId("ory/message/4100002");
    expect(toast).toHaveTextContent("Could not update settings");
    expect(toast).toHaveTextContent(
      "You can't disconnect your Google account while you belong to a workspace that requires Google login. Leave these workspaces first: Acme.",
    );
  });

  it("words both in the member's language, with the list in its own form", async () => {
    await ensureLanguage("de");
    await i18n.changeLanguage("de");
    const { unmount } = inOry(<Content message={EMAIL_REFUSED} />, "de");
    expect(screen.getByTestId("ory/message/4100001")).toHaveTextContent(
      "Du kannst deine E-Mail-Adresse nicht ändern, solange du zu einem Workspace gehörst, der die Google-Anmeldung erfordert. Verlasse zuerst diese Workspaces: Acme und Beta.",
    );
    unmount();

    await i18n.changeLanguage("zh");
    inOry(<Toast id="t1" message={UNLINK_REFUSED} />, "zh");
    expect(screen.getByTestId("ory/message/4100002")).toHaveTextContent(
      "你属于要求使用 Google 登录的工作区，因此无法断开你的 Google 账号。请先离开这些工作区：Acme。",
    );
  });

  it("keeps the server's text for a refusal that names no workspace", () => {
    inOry(<Content message={{ ...EMAIL_REFUSED, context: {} }} />);
    expect(screen.getByTestId("ory/message/4100001")).toHaveTextContent(
      EMAIL_REFUSED.text,
    );
  });

  it("leaves every other message to Ory's own catalog", async () => {
    await i18n.changeLanguage("zh");
    inOry(
      <Content
        message={{
          id: 1050001,
          type: "success",
          text: "Your changes have been saved!",
        }}
      />,
      "zh",
    );
    const message = screen.getByTestId("ory/message/1050001");
    expect(message).toHaveTextContent(
      OryLocales.zh["identities.messages.1050001"],
    );
    expect(message).toHaveClass("text-interface-foreground-validation-success");
  });
});
