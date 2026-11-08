import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { cn } from "@/common/lib/utils/utils";

/**
 * The Automations list page's frame: one scrolling region and a centered
 * column. The ready page and its pending skeleton both render through it, so
 * the outer bounds, padding and max-width cannot drift apart (the dashboard
 * skeleton rule).
 */
export function AutomationListFrame({ children }: { children: ReactNode }) {
  return (
    <DashboardLayout>
      <div className="min-h-0 min-w-0 flex-1 overflow-auto">
        <div className="mx-auto w-full max-w-4xl space-y-6 p-4 sm:p-6">
          {children}
        </div>
      </div>
    </DashboardLayout>
  );
}

/**
 * The create, edit and detail pages' frame: a bordered header bar with a back
 * link and the title, then the scrolling body. `title` is a node so a pending
 * skeleton can put a bar where the automation's name will be.
 */
export function AutomationSubPageFrame({
  title,
  backLabel,
  width,
  children,
}: {
  title: ReactNode;
  backLabel: string;
  width: "form" | "detail";
  children: ReactNode;
}) {
  return (
    <DashboardLayout>
      <div className="flex items-center gap-3 border-b px-4 py-4 sm:px-6">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/automations" aria-label={backLabel}>
            <ArrowLeft />
          </Link>
        </Button>
        <h1 className="h-7 min-w-0 truncate text-xl font-semibold">{title}</h1>
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-4 sm:p-6">
        <div
          className={cn(
            "mx-auto w-full space-y-6",
            width === "form" ? "max-w-2xl" : "max-w-4xl",
          )}
        >
          {children}
        </div>
      </div>
    </DashboardLayout>
  );
}
