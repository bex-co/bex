import { I18nextProvider } from "react-i18next";
import { IntlProvider } from "react-intl";
import { OryLocales } from "@ory/elements-react";
import { ThemeProvider } from "@/common/providers/theme-provider";
import { Toaster } from "@/common/components/ui/sonner";
import { VisualViewportHeight } from "@/common/providers/visual-viewport-height";
import { WorkspaceProvider } from "@/features/workspaces/context";
import { useTranslations } from "@/common/hooks/use-translations";
import i18n from "@/i18n/init";

/**
 * The `<Toaster/>`, wrapped in the intl context Ory Elements' toasts need.
 *
 * Ory's flow components raise their messages as toasts via sonner's `toast()`,
 * which PORTALS the toast into `<Toaster/>` — so the toast renders in the
 * Toaster's tree, not under the `<Settings>`/`<Login>` that fired it, and
 * therefore *outside* the IntlProvider those flow components mount internally.
 * Ory's `DefaultToast` calls `useIntl()`, so with no intl context above the
 * Toaster it throws "Could not find required `intl` object" and the root error
 * boundary takes down the entire page. That made /settings unusable the moment
 * a flow produced any message — e.g. landing there from a completed recovery,
 * or a rejected password — which is exactly how it shipped broken.
 *
 * Ory's own catalog (`OryLocales`) is passed so the toast stays translated
 * rather than falling back to the hardcoded English defaults; the locale
 * follows i18next, matching `useOryConfig()`'s `intl.locale`.
 */
function OryToaster() {
  const { i18n: i18next } = useTranslations();
  const locale = i18next.language;
  const messages = (OryLocales as Record<string, Record<string, string>>)[
    locale
  ];

  return (
    <IntlProvider
      locale={locale}
      defaultLocale="en"
      messages={messages ?? OryLocales.en}
    >
      <Toaster />
    </IntlProvider>
  );
}

// WorkspaceProvider lives here, not inside DashboardLayout: every authenticated
// route calls its data hooks (useServices, useDatabases, ...) in the PAGE
// component's own body, before returning `<DashboardLayout>{...}</DashboardLayout>`
// — those hooks' components are ancestors of DashboardLayout, not descendants,
// so a provider mounted inside DashboardLayout can never reach them. Mounting
// it here, above every route's `<Outlet/>`, makes it available regardless of
// where in a page's body a workspace-scoped hook is called.
export const RootProvider = ({
  children,
  initialWorkspaceId = null,
  onWorkspaceChange,
}: {
  children: React.ReactNode;
  initialWorkspaceId?: string | null;
  onWorkspaceChange?: () => void;
}) => {
  return (
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        <WorkspaceProvider
          initialWorkspaceId={initialWorkspaceId}
          onWorkspaceChange={onWorkspaceChange}
        >
          {children}
        </WorkspaceProvider>
        <VisualViewportHeight />
        <OryToaster />
      </ThemeProvider>
    </I18nextProvider>
  );
};
