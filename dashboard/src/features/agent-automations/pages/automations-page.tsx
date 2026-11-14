import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { Loader2, Plus, Timer } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/common/components/ui/tabs";
import { useTranslations } from "@/common/hooks/use-translations";
import { AutomationListFrame } from "@/features/agent-automations/components/automation-frame";
import { AutomationLoadError } from "@/features/agent-automations/components/automation-load-state";
import { AutomationRow } from "@/features/agent-automations/components/automation-row";
import { AutomationRowsSkeleton } from "@/features/agent-automations/components/automations-list-skeleton";
import { useAgentAutomations } from "@/features/agent-automations/hooks/use-agent-automations";
import { useAutomationOperations } from "@/features/agent-automations/hooks/use-automation-operations";
import { useAutomationViewer } from "@/features/agent-automations/hooks/use-automation-viewer";

type Scope = "mine" | "all";

/**
 * /automations (ADR101 D9): the workspace's agent automations with Mine and All
 * tabs. Each row shows the trigger in words, the next and last run, a paused
 * badge with its reason, and a switch that pauses and resumes.
 */
export function AutomationsPage() {
  const { t } = useTranslations();
  const [scope, setScope] = useState<Scope>("mine");
  const viewer = useAutomationViewer();
  const ops = useAutomationOperations();
  const {
    automations,
    loading,
    error,
    refetch,
    loadMore,
    loadingMore,
    hasMore,
  } = useAgentAutomations(scope === "mine");

  return (
    <AutomationListFrame>
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h1 className="text-xl font-semibold">
            {t("agentAutomations.pageTitle")}
          </h1>
          <p className="text-muted-foreground text-sm">
            {t("agentAutomations.pageDescription")}
          </p>
        </div>
        {viewer.canCreate ? (
          <Button asChild>
            <Link to="/automations/new">
              <Plus />
              {t("agentAutomations.create")}
            </Link>
          </Button>
        ) : (
          <Button disabled>
            <Plus />
            {t("agentAutomations.create")}
          </Button>
        )}
      </header>

      <Tabs value={scope} onValueChange={(next) => setScope(next as Scope)}>
        <TabsList>
          {/* The list below is not a Tabs panel, so the tabs name none (an
              aria-controls to a missing id is invalid ARIA). */}
          <TabsTrigger value="mine" aria-controls={undefined}>
            {t("agentAutomations.tabMine")}
          </TabsTrigger>
          <TabsTrigger value="all" aria-controls={undefined}>
            {t("agentAutomations.tabAll")}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      {loading ? (
        <AutomationRowsSkeleton />
      ) : error && automations.length === 0 ? (
        <AutomationLoadError error={error} onRetry={() => void refetch()} />
      ) : automations.length === 0 ? (
        <EmptyAutomations scope={scope} canCreate={viewer.canCreate} />
      ) : (
        <ul
          className="divide-y rounded-md border"
          data-testid="automation-list"
          aria-label={t("agentAutomations.pageTitle")}
        >
          {automations.map((automation) => (
            <AutomationRow
              key={automation.id}
              automation={automation}
              canOperate={viewer.actionsFor(automation.createdBy).operate}
              busy={ops.busy}
              onToggle={(a) => void ops.toggle(a)}
            />
          ))}
        </ul>
      )}

      {hasMore ? (
        <div className="flex justify-center">
          <Button
            variant="outline"
            size="sm"
            disabled={loadingMore}
            onClick={() => void loadMore()}
          >
            {loadingMore ? <Loader2 className="animate-spin" /> : null}
            {t("agentAutomations.loadMore")}
          </Button>
        </div>
      ) : null}
    </AutomationListFrame>
  );
}

function EmptyAutomations({
  scope,
  canCreate,
}: {
  scope: Scope;
  canCreate: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div
      className="flex flex-col items-center gap-3 rounded-md border px-6 py-12 text-center"
      data-testid="automations-empty"
    >
      <Timer className="text-muted-foreground size-8" aria-hidden />
      <div className="space-y-1">
        <p className="font-medium">
          {scope === "mine"
            ? t("agentAutomations.emptyMineTitle")
            : t("agentAutomations.emptyAllTitle")}
        </p>
        <p className="text-muted-foreground max-w-md text-sm">
          {t("agentAutomations.emptyBody")}
        </p>
      </div>
      {canCreate ? (
        <Button asChild>
          <Link to="/automations/new">
            <Plus />
            {t("agentAutomations.create")}
          </Link>
        </Button>
      ) : null}
    </div>
  );
}
