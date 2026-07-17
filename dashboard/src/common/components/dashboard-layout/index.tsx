import {
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from "@/common/components/ui/sidebar.tsx";
import { Authenticated } from "@/common/components/authenticated";
import { UserNav } from "@/common/components/user-nav.tsx";
import { useInviteRedemption } from "@/features/team/hooks/use-invite-redemption";
import { DashboardSidebar } from "./dashboard-sidebar";

/** Redeems a pending workspace-invite token once the caller is authenticated
 *  (w1/m33) — a child component (not a hook call inside DashboardLayout) so it
 *  only runs behind the Authenticated gate. */
function InviteRedemption() {
  useInviteRedemption();
  return null;
}

function DashboardHeader() {
  const { state, isMobile, openMobile } = useSidebar();

  // Only show trigger when sidebar is hidden
  const showTrigger = isMobile ? !openMobile : state === "collapsed";

  return (
    <header className="sticky top-0 z-50 h-12 shrink-0 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
      <div className="flex h-full items-center justify-between px-4">
        <div className="flex items-center gap-3">
          {showTrigger && <SidebarTrigger className="-ml-1" />}
        </div>
        <div className="flex items-center gap-1">
          <Authenticated>
            <UserNav />
          </Authenticated>
        </div>
      </div>
    </header>
  );
}

/**
 * Dashboard layout — persistent sidebar + header chrome wrapping every
 * authenticated page (preserved from the original scaffold's design).
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
