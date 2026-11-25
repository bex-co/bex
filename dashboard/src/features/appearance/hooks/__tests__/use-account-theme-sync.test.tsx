import { useContext, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, waitFor } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import {
  UpdateUserLogThemeSettingDocument,
  UpdateUserThemeSettingDocument,
  UserThemeSettingsDocument,
} from "@/graphql/definitions";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { ThemeProvider } from "@/common/providers/theme-provider";
import { ThemeProviderContext } from "@/common/providers/theme-provider/context";
import { SETTINGS_STORAGE_KEY } from "@/common/providers/theme-provider/settings";
import type { ThemeProviderContextType } from "@/common/providers/theme-provider/type";
import { memoryStorage } from "@/test/mocks/storage";

// setup.ts stubs this hook for every other test; this one runs it.
vi.unmock("@/features/appearance/hooks/use-account-theme-sync");
const { useAccountThemeSync } = await import("../use-account-theme-sync");

const toastError = vi.hoisted(() => vi.fn());
vi.mock("sonner", () => ({ toast: { error: toastError } }));

// The signed-in shell reads the account's theme with Render's documents and
// writes every change back (w3/m531).

const USER_ID = "usr-d4a5cbbpdc73dcrkvg00";

const userQuery = (themeSetting: string, logThemeSetting: string) => ({
  request: { query: UserThemeSettingsDocument },
  result: {
    data: {
      user: { __typename: "User", id: USER_ID, themeSetting, logThemeSetting },
    },
  },
});

let theme: ThemeProviderContextType;
function Shell() {
  useAccountThemeSync();
  theme = useContext(ThemeProviderContext);
  return null;
}

function renderShell(mocks: object[]) {
  const client = new ApolloClient({
    link: new MockLink(mocks as never) as ApolloLink,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const Wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>
      <ThemeProvider>{children}</ThemeProvider>
    </ApolloProvider>
  );
  render(<Shell />, { wrapper: Wrapper });
  return client;
}

/** The account as the cache holds it: what the server last answered. */
const cachedUser = (client: ApolloClient) =>
  (client.cache.extract() as Record<string, unknown>)[`User:${USER_ID}`];

const root = () => document.documentElement.classList;
const stored = () =>
  JSON.parse(localStorage.getItem(SETTINGS_STORAGE_KEY) ?? "null");

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
  toastError.mockReset();
  root().remove("light", "dark", "high-contrast");
});
afterEach(() => {
  vi.unstubAllGlobals();
  root().remove("light", "dark", "high-contrast");
});

describe("useAccountThemeSync (w3/m531)", () => {
  it("opens a new browser in the account's stored theme", async () => {
    renderShell([userQuery("DARK_HIGH_CONTRAST", "ALWAYS_LIGHT")]);
    await waitFor(() => expect(root().contains("dark")).toBe(true));
    expect(root().contains("high-contrast")).toBe(true);
    expect(theme.logTheme).toBe("ALWAYS_LIGHT");
    expect(stored()).toEqual({
      themeSetting: "DARK_HIGH_CONTRAST",
      logThemeSetting: "ALWAYS_LIGHT",
    });
  });

  it("writes each change with Render's mutations", async () => {
    const themeWrite = (themeSetting: string) => ({
      request: {
        query: UpdateUserThemeSettingDocument,
        variables: { id: USER_ID, themeSetting },
      },
      result: {
        data: {
          updateUserThemeSetting: {
            __typename: "User",
            id: USER_ID,
            themeSetting,
          },
        },
      },
    });
    const client = renderShell([
      userQuery("SYSTEM", "MATCH_APP"),
      themeWrite("LIGHT"),
      themeWrite("LIGHT_HIGH_CONTRAST"),
      {
        request: {
          query: UpdateUserLogThemeSettingDocument,
          variables: { id: USER_ID, logThemeSetting: "ALWAYS_DARK" },
        },
        result: {
          data: {
            updateUserLogThemeSetting: {
              __typename: "User",
              id: USER_ID,
              logThemeSetting: "ALWAYS_DARK",
            },
          },
        },
      },
    ]);
    // The account is bound once its read is in the cache.
    await waitFor(() => expect(cachedUser(client)).toBeDefined());
    act(() => void theme.setTheme("light"));
    act(() => void theme.setHighContrast(true));
    act(() => void theme.setLogTheme("ALWAYS_DARK"));
    // Every write reached the server and came back.
    await waitFor(() =>
      expect(cachedUser(client)).toMatchObject({
        themeSetting: "LIGHT_HIGH_CONTRAST",
        logThemeSetting: "ALWAYS_DARK",
      }),
    );
    await waitFor(() =>
      expect(stored()).toEqual({
        themeSetting: "LIGHT_HIGH_CONTRAST",
        logThemeSetting: "ALWAYS_DARK",
      }),
    );
    expect(toastError).not.toHaveBeenCalled();
  });

  it("says a refused write was not saved, and puts the previous theme back", async () => {
    renderShell([
      userQuery("LIGHT", "MATCH_APP"),
      {
        request: {
          query: UpdateUserThemeSettingDocument,
          variables: { id: USER_ID, themeSetting: "DARK" },
        },
        error: new Error("network down"),
      },
    ]);
    await waitFor(() => expect(stored()?.themeSetting).toBe("LIGHT"));
    act(() => void theme.setTheme("dark"));
    expect(root().contains("dark")).toBe(true);
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        "Your theme wasn't saved. The previous theme is back.",
      ),
    );
    await waitFor(() => expect(root().contains("light")).toBe(true));
    expect(theme.theme).toBe("light");
    expect(stored()?.themeSetting).toBe("LIGHT");
  });
});
