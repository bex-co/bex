import { I18nextProvider } from "react-i18next";
import { ThemeProvider } from "@/common/providers/theme-provider";
import { Toaster } from "@/common/components/ui/sonner";
import { VisualViewportHeight } from "@/common/providers/visual-viewport-height";
import { WorkspaceProvider } from "@/features/workspaces/context";
import i18n from "@/i18n/init";

// WorkspaceProvider lives here, not inside DashboardLayout: every authenticated
// route calls its data hooks (useServices, useDatabases, ...) in the PAGE
// component's own body, before returning `<DashboardLayout>{...}</DashboardLayout>`
// — those hooks' components are ancestors of DashboardLayout, not descendants,
// so a provider mounted inside DashboardLayout can never reach them. Mounting
// it here, above every route's `<Outlet/>`, makes it available regardless of
// where in a page's body a workspace-scoped hook is called.
export const RootProvider = ({ children }: { children: React.ReactNode }) => {
  return (
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        <WorkspaceProvider>{children}</WorkspaceProvider>
        <VisualViewportHeight />
        <Toaster />
      </ThemeProvider>
    </I18nextProvider>
  );
};
