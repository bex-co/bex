import { lazy, Suspense } from "react";
import { I18nextProvider } from "react-i18next";
import { ThemeProvider } from "@/common/providers/theme-provider";
import { VisualViewportHeight } from "@/common/providers/visual-viewport-height";
import { WorkspaceProvider } from "@/features/workspaces/context";
import { PaymentRequiredProvider } from "@/features/usage/context/payment-required";
import i18n from "@/i18n/init";

// Lazy so react-intl/formatjs (only OryToaster needs it) ships as its own async
// chunk instead of sitting in the always-mounted entry chunk (w9/m60 t004). The
// toaster is a portal target with nothing to paint until a flow raises a toast,
// so a `null` fallback is invisible and correct.
const OryToaster = lazy(() => import("./ory-toaster"));

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
          <PaymentRequiredProvider>{children}</PaymentRequiredProvider>
        </WorkspaceProvider>
        <VisualViewportHeight />
        <Suspense fallback={null}>
          <OryToaster />
        </Suspense>
      </ThemeProvider>
    </I18nextProvider>
  );
};
