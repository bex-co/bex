import type { ReactNode } from "react";
import { buttonVariants } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";

/**
 * The ready line's real text, transparent on a pulsing background: the bar
 * takes the line's exact wrap, width and height in every language.
 */
function TextBar({
  className,
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        "bg-accent animate-pulse rounded-md box-decoration-clone motion-reduce:animate-none group-data-[skeleton-frame=true]:!animate-none",
        className,
        "text-transparent",
      )}
    >
      {children}
    </span>
  );
}

/** One editable rule row of IPAllowListEditor: two inputs and three icon buttons. */
function RuleRowSkeleton() {
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-md border p-2">
      <Skeleton className="h-9 w-full max-w-xs min-w-0" />
      <Skeleton className="h-9 w-full max-w-xs min-w-0" />
      <Skeleton className="size-8" />
      <Skeleton className="size-8" />
      <Skeleton className="size-8" />
    </div>
  );
}

export interface WorkspaceIPRulesCardSkeletonProps {
  /** The workspace plan is below Scale (known once the workspace list is). */
  belowScale?: boolean;
  /**
   * The caller may edit (`can_manage`), read as the card reads it: false while
   * the grant loads, so the card is read-only then and this preview is too.
   */
  canManage?: boolean;
}

/**
 * The Workspace Settings → Inbound IP Rules card while its rules load
 * (w9/m388), at the ready card's geometry (dashboard/AGENTS.md skeleton rule)
 * for Render's seeded allow-all pair, the list every workspace starts with:
 *
 * - an Admin on Scale or higher gets the editor: the hint line, one row per
 *   seeded rule, and the add row with its Add source / Cancel / Save actions;
 * - below Scale, or for a member who cannot edit, the read-only view: the plan
 *   or Admin-only notice and the "open to all" line (an allow-all list offers
 *   no reset).
 *
 * The defaults are the editor: the route's own pending skeleton renders this
 * before the workspace list, so neither the plan nor the role is known yet.
 */
export function WorkspaceIPRulesCardSkeleton({
  belowScale = false,
  canManage = true,
}: WorkspaceIPRulesCardSkeletonProps) {
  const { t } = useTranslations();
  const editable = !belowScale && canManage;
  return (
    <Card
      data-testid="workspace-ip-rules-skeleton"
      data-variant={editable ? "editor" : "read-only"}
    >
      <CardHeader>
        <CardTitle>{t("workspaces.ipRulesTitle")}</CardTitle>
        <CardDescription>{t("workspaces.ipRulesDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2" aria-hidden>
          {editable ? (
            <>
              <p className="text-xs">
                <TextBar>{t("workspaces.ipRulesHint")}</TextBar>
              </p>
              <div className="space-y-2" data-skeleton-region="ip-rule-rows">
                <RuleRowSkeleton />
                <RuleRowSkeleton />
              </div>
              <div
                className="flex flex-wrap gap-2"
                data-skeleton-region="ip-rule-actions"
              >
                <Skeleton className="h-9 w-full max-w-xs min-w-0" />
                <Skeleton className="h-9 w-full max-w-xs min-w-0" />
                <Skeleton className="h-8 border border-transparent px-2.5 text-sm font-medium">
                  <span className="invisible flex h-full items-center gap-1.5">
                    <span className="size-4" />
                    {t("workspaces.ipRulesAdd")}
                  </span>
                </Skeleton>
                <Skeleton className="h-8 border border-transparent px-3 text-sm font-medium">
                  <span className="invisible flex h-full items-center">
                    {t("workspaces.ipRulesCancel")}
                  </span>
                </Skeleton>
                <Skeleton className="h-8 px-3 text-sm font-medium">
                  <span className="invisible flex h-full items-center">
                    {t("workspaces.ipRulesSave")}
                  </span>
                </Skeleton>
              </div>
            </>
          ) : (
            <>
              <p className="text-xs" data-skeleton-region="ip-rule-notice">
                {belowScale ? (
                  <>
                    <TextBar>{t("workspaces.ipRulesScalePlanOnly")}</TextBar>{" "}
                    {canManage ? (
                      <TextBar
                        className={cn(
                          buttonVariants({ variant: "link" }),
                          "h-auto p-0 text-xs",
                        )}
                      >
                        {t("workspaces.changePlanTrigger")}
                      </TextBar>
                    ) : (
                      <TextBar>{t("workspaces.ipRulesAdminOnly")}</TextBar>
                    )}
                  </>
                ) : (
                  <TextBar>{t("workspaces.ipRulesAdminOnly")}</TextBar>
                )}
              </p>
              <TextBar className="text-sm">
                {t("workspaces.ipRulesOpen")}
              </TextBar>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
