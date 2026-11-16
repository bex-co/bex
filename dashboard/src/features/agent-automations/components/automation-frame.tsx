import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { PageFrame, PageHeader } from "@/common/components/page-frame";

/**
 * The Automations list page's frame: one scrolling region and a centered
 * column. The ready page and its pending skeleton both render through it, so
 * the outer bounds, padding and max-width cannot drift apart (the dashboard
 * skeleton rule).
 */
export function AutomationListFrame({ children }: { children: ReactNode }) {
  return (
    <DashboardLayout>
      <PageFrame>{children}</PageFrame>
    </DashboardLayout>
  );
}

/**
 * The create, edit and detail pages' frame: the shared page header with a
 * back link and the title, then the body. `title` is a node so a pending
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
      <PageFrame
        width={width === "form" ? "form" : "standard"}
        header={
          <PageHeader
            leading={
              <Button variant="ghost" size="icon" asChild>
                <Link to="/automations" aria-label={backLabel}>
                  <ArrowLeft />
                </Link>
              </Button>
            }
            title={title}
          />
        }
      >
        {children}
      </PageFrame>
    </DashboardLayout>
  );
}
