import { Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { TabsList, TabsTrigger, Tabs } from "@/common/components/ui/tabs";
import { useTranslations } from "@/common/hooks/use-translations";
import { AutomationListFrame } from "@/features/agent-automations/components/automation-frame";
import { AutomationRowShell } from "@/features/agent-automations/components/automation-row";

/** Representative row count; the ready list's count is data-dependent. */
const SKELETON_ROWS = 4;

/** The list's rows pending: the ready row's shell with bars in its slots. */
export function AutomationRowsSkeleton() {
  return (
    <ul
      className="divide-y rounded-md border"
      data-skeleton-region="automation-list"
    >
      {Array.from({ length: SKELETON_ROWS }, (_, index) => (
        <AutomationRowShell
          key={index}
          name={<Skeleton className="h-4 w-40 max-w-[60%]" />}
          trigger={<Skeleton className="h-3.5 w-56 max-w-[80%]" />}
          timing={<Skeleton className="h-3 w-48 max-w-[75%]" />}
          control={<Skeleton className="h-5 w-9 shrink-0 rounded-full" />}
        />
      ))}
    </ul>
  );
}

/**
 * /automations while its list loads: the ready page's frame, real heading,
 * description and tab labels (static copy), a disabled Create button, and the
 * list's rows as bars. Nothing is actionable before the viewer's role is known.
 */
export function AutomationsListSkeleton() {
  const { t } = useTranslations();
  return (
    <AutomationListFrame>
      <div
        aria-busy="true"
        data-skeleton-frame="true"
        data-route-skeleton="automations-list"
        className="space-y-6"
      >
        <header
          data-skeleton-region="page-header"
          className="flex flex-wrap items-start justify-between gap-3"
        >
          <div className="min-w-0 space-y-1">
            <h1 className="text-xl font-semibold">
              {t("agentAutomations.pageTitle")}
            </h1>
            <p className="text-muted-foreground text-sm">
              {t("agentAutomations.pageDescription")}
            </p>
          </div>
          <Button disabled>
            <Plus />
            {t("agentAutomations.create")}
          </Button>
        </header>
        <div data-skeleton-region="scope-tabs">
          <Tabs value="mine">
            <TabsList>
              <TabsTrigger value="mine" disabled>
                {t("agentAutomations.tabMine")}
              </TabsTrigger>
              <TabsTrigger value="all" disabled>
                {t("agentAutomations.tabAll")}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
        <AutomationRowsSkeleton />
      </div>
    </AutomationListFrame>
  );
}
