import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { Authenticated } from "@/common/components/authenticated";
import { useInviteRedemption } from "@/features/team/hooks/use-invite-redemption";
import { DashboardSidebar } from "./dashboard-sidebar";
import { DashboardHeader } from "./dashboard-header";

/** Redeems a pending workspace-invite token once the caller is authenticated
 *  (w1/m33) — a child component (not a hook call inside DashboardLayout) so it
 *  only runs behind the Authenticated gate. */
function InviteRedemption() {
  useInviteRedemption();
  return null;
}

/**
 * Dashboard layout — persistent contextual sidebar + Render-style topbar
 * wrapping every authenticated page. The topbar supplies hierarchy, global
 * search, create, help, and account navigation regardless of the active route.
 * If sidebar is not provided, defaults to DashboardSidebar.
 */
export function DashboardLayout({
  children,
  sidebar,
}: {
  children?: React.ReactNode;
  sidebar?: React.ReactNode;
}) {
  return (
    <SidebarProvider>
      <Authenticated>
        <InviteRedemption />
      </Authenticated>
      <div className="flex h-(--visual-viewport-height,100vh) w-full">
        {sidebar || <DashboardSidebar />}
        <main className="flex flex-1 flex-col min-w-0 w-full">
          <DashboardHeader />
          {children}
        </main>
      </div>
    </SidebarProvider>
  );
}
