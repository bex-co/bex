import type { ComponentType } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { IntlProvider } from "react-intl";
import {
  OryLocales,
  type OryFormSsoRootProps,
  type OryNodeSettingsButton,
  type OryNodeSsoButtonProps,
  type OrySettingsSsoProps,
  type UiNodeInput,
} from "@ory/elements-react";
import i18n, { ensureLanguage } from "@/i18n/init";
import { SUPPORTED_LANGUAGES } from "@/i18n/config";
import {
  LoginWidgetSkeleton,
  RegistrationWidgetSkeleton,
} from "@/common/components/route-skeletons";
import {
  oryAuthFormOverrides,
  orySettingsFormOverrides,
} from "../auth-form-overrides";

// The components the login, sign-up and settings pages hand Ory (w7/m468).
const SsoButton = oryAuthFormOverrides.Node!
  .SsoButton as ComponentType<OryNodeSsoButtonProps>;
const SsoRoot = oryAuthFormOverrides.Form!
  .SsoRoot as ComponentType<OryFormSsoRootProps>;
const SettingsSsoButton = orySettingsFormOverrides.Node!
  .SsoButton as ComponentType<OryNodeSsoButtonProps>;
const SsoSettings = orySettingsFormOverrides.Form!
  .SsoSettings as ComponentType<OrySettingsSsoProps>;

/** Kratos's providers as the installation labels them (`label`). */
const LABELS: Record<string, string> = {
  github: "GitHub",
  google: "Google",
  gitlab: "GitLab",
  bitbucket: "Bitbucket",
};

// Kratos's message ids: sign in / sign up with, link, unlink {provider}.
const SIGN_IN = 1010002;
const SIGN_UP = 1040002;
const LINK = 1050002;
const UNLINK = 1050003;

const ENGLISH: Record<number, string> = {
  [SIGN_IN]: "Sign in with {provider}",
  [SIGN_UP]: "Sign up with {provider}",
  [LINK]: "Link {provider}",
  [UNLINK]: "Unlink {provider}",
};

/** An oidc node as Kratos v26.2.0 sends it. */
function oidcNode(
  provider: string,
  messageId = SIGN_IN,
  name = "provider",
): UiNodeInput {
  const label = LABELS[provider] ?? provider;
  return {
    type: "input",
    group: "oidc",
    attributes: {
      name,
      type: "submit",
      value: provider,
      node_type: "input",
      disabled: false,
    },
    messages: [],
    meta: {
      label: {
        id: messageId,
        type: "info",
        text: ENGLISH[messageId]!.replace("{provider}", label),
        context: { provider: label, provider_id: provider },
      },
    },
  } as UiNodeInput;
}

function buttonProps(node: UiNodeInput) {
  return {
    type: "submit" as const,
    name: node.attributes.name,
    value: node.attributes.value as string,
    onClick: () => undefined,
  };
}

function SignInButton({ node }: { node: UiNodeInput }) {
  return (
    <SsoButton
      node={node}
      attributes={node.attributes}
      buttonProps={buttonProps(node)}
      isSubmitting={false}
      provider={LABELS[node.attributes.value as string] ?? ""}
    />
  );
}

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

/** The logo a button or row draws: its provider's mark, or Ory's generic tile. */
function logoOf(element: HTMLElement): string | null | undefined {
  return element
    .querySelector("[data-sso-logo]")
    ?.getAttribute("data-sso-logo");
}

afterEach(async () => {
  // Unmount first: a mounted tree would re-render on the language change.
  cleanup();
  await i18n.changeLanguage("en");
});

describe("the sign-in buttons' logos", () => {
  it("draws Bitbucket's mark on its button, not Ory's generic tile", () => {
    inOry(<SignInButton node={oidcNode("bitbucket")} />);

    const button = screen.getByRole("button", {
      name: "Sign in with Bitbucket",
    });
    expect(logoOf(button)).toBe("bitbucket");
    const mark = button.querySelector('svg[data-sso-logo="bitbucket"]')!;
    expect(mark.querySelector('path[fill="#2684FF"]')).not.toBeNull();
    expect(mark.querySelector("linearGradient")).not.toBeNull();
    expect(button).toHaveTextContent(/^Bitbucket$/);
  });

  it.each([
    ["gitlab", "#E24329"],
    ["github", undefined],
    ["google", "#4285F4"],
  ])("draws %s's mark on its button", (provider, fill) => {
    inOry(<SignInButton node={oidcNode(provider)} />);

    const button = screen.getByRole("button", {
      name: `Sign in with ${LABELS[provider]}`,
    });
    expect(logoOf(button)).toBe(provider);
    if (fill) {
      expect(
        button.querySelector(`svg[data-sso-logo] path[fill="${fill}"]`),
      ).not.toBeNull();
    } else {
      // GitHub's monochrome mark follows the text colour (dark theme).
      expect(button.querySelector("svg[data-sso-logo]")).toHaveAttribute(
        "fill",
        "currentColor",
      );
    }
  });

  it("gives a provider with no mark Ory's generic tile with its initial", () => {
    inOry(<SignInButton node={oidcNode("acme")} />);

    const button = screen.getByRole("button", { name: "Sign in with acme" });
    expect(logoOf(button)).toBe("generic");
    expect(button.querySelector('[data-sso-logo="generic"]')).toHaveTextContent(
      "a",
    );
  });
});

describe("the sign-in buttons' labels in every language", () => {
  it.each(SUPPORTED_LANGUAGES)(
    "%s: each button shows the provider's name and is named by Kratos's label in Ory's words",
    (lang) => {
      const catalog = OryLocales[lang] as Record<string, string>;
      inOry(
        <>
          {Object.keys(LABELS).map((provider) => (
            <SignInButton key={provider} node={oidcNode(provider)} />
          ))}
          <SignInButton node={oidcNode("bitbucket", SIGN_UP)} />
        </>,
        lang,
      );

      for (const [provider, label] of Object.entries(LABELS)) {
        const name = catalog[`identities.messages.${SIGN_IN}`]!.replace(
          "{provider}",
          label,
        );
        const button = screen.getByRole("button", { name });
        expect(button).toHaveTextContent(new RegExp(`^${label}$`));
        expect(logoOf(button)).toBe(provider);
      }
      expect(
        screen.getByRole("button", {
          name: catalog[`identities.messages.${SIGN_UP}`]!.replace(
            "{provider}",
            "Bitbucket",
          ),
        }),
      ).toHaveTextContent(/^Bitbucket$/);
    },
  );
});

/** Ory's SsoRoot: the flow's oidc nodes, and one rendered child per node. */
function renderRows(providers: string[]) {
  const nodes = providers.map((provider) => oidcNode(provider));
  return inOry(
    <SsoRoot nodes={nodes}>
      {nodes.map((node) => (
        <SignInButton key={node.attributes.value as string} node={node} />
      ))}
    </SsoRoot>,
  );
}

function providerButtons(): string[] {
  return screen
    .queryAllByRole("button")
    .filter(
      (button) => button.dataset.testid === "ory/form/node/input/provider",
    )
    .map((button) => button.textContent ?? "");
}

describe("the provider buttons' layout, as on Render", () => {
  it("shows GitHub and Google side by side, labelled, and keeps GitLab and Bitbucket behind More options", () => {
    // Kratos lists its nodes by provider id.
    renderRows(["bitbucket", "github", "gitlab", "google"]);

    expect(providerButtons()).toEqual(["GitHub", "Google"]);
    const firstRow = screen.getByRole("button", {
      name: "Sign in with GitHub",
    }).parentElement!;
    expect(firstRow).toHaveClass("grid-cols-2");

    fireEvent.click(screen.getByRole("button", { name: "More options" }));

    expect(providerButtons()).toEqual([
      "GitHub",
      "Google",
      "GitLab",
      "Bitbucket",
    ]);
    expect(
      screen.queryByRole("button", { name: "More options" }),
    ).not.toBeInTheDocument();
    const secondRow = screen.getByRole("button", {
      name: "Sign in with GitLab",
    }).parentElement!;
    expect(secondRow).not.toBe(firstRow);
    expect(secondRow).toHaveClass("grid-cols-2");
    // The button that revealed them is gone: focus lands on the first of them.
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Sign in with GitLab" }),
    );
  });

  it("shows GitLab and Bitbucket directly when the installation offers no GitHub or Google", () => {
    renderRows(["bitbucket", "gitlab"]);

    expect(providerButtons()).toEqual(["GitLab", "Bitbucket"]);
    expect(
      screen.queryByRole("button", { name: "More options" }),
    ).not.toBeInTheDocument();
  });

  it("gives a lone provider the whole row", () => {
    renderRows(["github"]);

    const button = screen.getByRole("button", { name: "Sign in with GitHub" });
    expect(button).toHaveTextContent(/^GitHub$/);
    expect(button.parentElement).toHaveClass("grid-cols-1");
  });

  it("names More options in the page's language", async () => {
    await ensureLanguage("de");
    await i18n.changeLanguage("de");
    renderRows(["github", "gitlab"]);

    expect(
      screen.getByRole("button", { name: "Weitere Optionen" }),
    ).toBeInTheDocument();
  });
});

/** Every subset of Render's four providers: 0 to 4 buttons. */
const SUBSETS = Array.from({ length: 16 }, (_, mask) =>
  Object.keys(LABELS).filter((_, bit) => mask & (1 << bit)),
);

describe("the login and sign-up skeletons draw the ready card's provider rows", () => {
  it.each(SUBSETS.map((subset) => [subset.join(",") || "none", subset]))(
    "%s",
    (_, providers) => {
      const ready = renderRows(providers as string[]);
      const readyFirstRow = providerButtons().length;
      const readyMore = !!screen.queryByRole("button", {
        name: "More options",
      });
      ready.unmount();

      for (const Skeleton of [
        LoginWidgetSkeleton,
        RegistrationWidgetSkeleton,
      ]) {
        const { container, unmount } = render(
          <Skeleton ssoProviders={providers as string[]} />,
        );
        const sso = container.querySelector("[data-skeleton-sso]");
        expect(Number(sso?.getAttribute("data-skeleton-sso") ?? 0)).toBe(
          readyFirstRow,
        );
        expect(
          sso?.firstElementChild?.querySelectorAll('[data-slot="skeleton"]')
            .length ?? 0,
        ).toBe(readyFirstRow);
        expect(!!container.querySelector("[data-skeleton-sso-more]")).toBe(
          readyMore,
        );
        unmount();
      }
    },
  );

  it("draws no provider row when Kratos did not say which it offers", () => {
    const { container } = render(<LoginWidgetSkeleton ssoProviders={null} />);
    expect(container.querySelector("[data-skeleton-sso]")).toBeNull();
  });
});

function settingsButton(
  provider: string,
  messageId: number,
  onClick = vi.fn(),
): OryNodeSettingsButton {
  const node = oidcNode(
    provider,
    messageId,
    messageId === UNLINK ? "unlink" : "link",
  );
  return {
    ...node,
    onClick,
    buttonProps: { ...buttonProps(node), onClick },
  };
}

describe("account settings' connected accounts", () => {
  it("draws a linked Bitbucket account's row with Bitbucket's mark, and names its unlink button", () => {
    const unlink = vi.fn();
    inOry(
      <SsoSettings
        linkButtons={["gitlab", "google", "github"].map((provider) =>
          settingsButton(provider, LINK),
        )}
        unlinkButtons={[settingsButton("bitbucket", UNLINK, unlink)]}
        isSubmitting={false}
      />,
    );

    const row = screen.getByTestId("sso-linked/bitbucket");
    expect(logoOf(row)).toBe("bitbucket");
    expect(row).toHaveTextContent("Bitbucket");
    fireEvent.click(
      within(row).getByRole("button", { name: "Unlink Bitbucket" }),
    );
    expect(unlink).toHaveBeenCalledTimes(1);

    // The link buttons read Kratos's whole label, with their marks, in
    // Render's order.
    const links = screen
      .getAllByRole("button")
      .filter((button) => button.dataset.testid === "ory/form/node/input/link");
    expect(links.map((button) => button.textContent)).toEqual([
      "Link GitHub",
      "Link Google",
      "Link GitLab",
    ]);
    expect(links.map(logoOf)).toEqual(["github", "google", "gitlab"]);
  });

  it("draws a linked GitLab account's row with GitLab's mark", () => {
    inOry(
      <SsoSettings
        linkButtons={[]}
        unlinkButtons={[settingsButton("gitlab", UNLINK)]}
        isSubmitting={false}
      />,
    );

    expect(logoOf(screen.getByTestId("sso-linked/gitlab"))).toBe("gitlab");
  });

  it.each(SUPPORTED_LANGUAGES)(
    "%s: the link and unlink controls read Ory's words for Bitbucket",
    (lang) => {
      const catalog = OryLocales[lang] as Record<string, string>;
      const linkBitbucket = oidcNode("bitbucket", LINK, "link");
      inOry(
        <>
          <SettingsSsoButton
            node={linkBitbucket}
            attributes={linkBitbucket.attributes}
            buttonProps={buttonProps(linkBitbucket)}
            isSubmitting={false}
            provider="bitbucket"
          />
          <SsoSettings
            linkButtons={[]}
            unlinkButtons={[settingsButton("bitbucket", UNLINK)]}
            isSubmitting={false}
          />
        </>,
        lang,
      );

      const link = catalog[`identities.messages.${LINK}`]!.replace(
        "{provider}",
        "Bitbucket",
      );
      expect(screen.getByRole("button", { name: link })).toHaveTextContent(
        link,
      );
      expect(
        screen.getByRole("button", {
          name: catalog[`identities.messages.${UNLINK}`]!.replace(
            "{provider}",
            "Bitbucket",
          ),
        }),
      ).toBeInTheDocument();
    },
  );
});
