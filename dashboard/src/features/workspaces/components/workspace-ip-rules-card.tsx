import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  ALLOW_ALL_PAIR_ENTRIES,
  ipAllowListEntryKey,
} from "@/common/lib/ip-allow-list";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useSetWorkspaceIPAllowList } from "@/features/workspaces/hooks/use-set-workspace-ip-allow-list";
import { useWorkspaceIPAllowList } from "@/features/workspaces/hooks/use-workspace-ip-allow-list";
import { WorkspaceIPRulesCardSkeleton } from "@/features/workspaces/components/workspace-ip-rules-card-skeleton";
import {
  inboundIpRulesPlan,
  type WorkspaceView,
} from "@/features/workspaces/types";

export interface WorkspaceIPRulesCardProps {
  workspace: WorkspaceView;
  /** Opens the change-plan dialog on this page (`?plan=change`). */
  onChangePlan: () => void;
}

/**
 * Workspace Settings → Inbound IP Rules (w9/m388), Render's workspace-level
 * rules on Scale and Enterprise: they apply to every web service, static site,
 * Postgres and Key Value in the workspace, on top of environment and service
 * rules. The card reads them itself (`workspace(id)`, never the polled
 * workspace list). Only Admins edit them (`can_manage`, fail-closed:
 * read-only while the grant is loading or unknown). Below Scale the stored
 * list stays listed and enforced, and an Admin can only reset it to Render's
 * allow-all pair.
 */
export function WorkspaceIPRulesCard({
  workspace,
  onChangePlan,
}: WorkspaceIPRulesCardProps) {
  // Keyed on the workspace: the switcher swaps the prop without remounting,
  // and a draft or a refusal must never carry over to another workspace.
  return (
    <WorkspaceIPRulesForm
      key={workspace.id}
      workspace={workspace}
      onChangePlan={onChangePlan}
    />
  );
}

function WorkspaceIPRulesForm({
  workspace,
  onChangePlan,
}: WorkspaceIPRulesCardProps) {
  const { t } = useTranslations();
  const { canManage } = useCapabilities();
  const { entries, loading } = useWorkspaceIPAllowList(workspace.id);
  const { save, saving, refusal } = useSetWorkspaceIPAllowList();
  const belowScale = !inboundIpRulesPlan(workspace.plan);

  // The plan (from the workspace list) and the grant are known by now, so the
  // preview is the variant the ready card is about to render.
  if (loading) {
    return (
      <WorkspaceIPRulesCardSkeleton
        belowScale={belowScale}
        canManage={canManage}
      />
    );
  }

  const changePlan = (
    <Button
      type="button"
      variant="link"
      className="h-auto p-0 text-xs"
      onClick={onChangePlan}
    >
      {t("workspaces.changePlanTrigger")}
    </Button>
  );

  // One editor element whatever the mode, so a grant withdrawn while a check
  // is in flight (focus, reconnect) keeps the draft for when it returns.
  const readOnly = canManage
    ? undefined
    : {
        hint: belowScale ? (
          <>
            {t("workspaces.ipRulesScalePlanOnly")}{" "}
            {t("workspaces.ipRulesAdminOnly")}
          </>
        ) : (
          t("workspaces.ipRulesAdminOnly")
        ),
      };
  const clearOnly =
    belowScale && canManage
      ? {
          hint: (
            <>
              {t("workspaces.ipRulesScalePlanOnly")} {changePlan}
            </>
          ),
          clearAll: t("workspaces.ipRulesResetAllowAll"),
        }
      : undefined;

  return (
    <Card data-testid="workspace-ip-rules-card">
      <CardHeader>
        <CardTitle>{t("workspaces.ipRulesTitle")}</CardTitle>
        <CardDescription>{t("workspaces.ipRulesDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* A failed read is not "no rules": an empty list is deny-all, so
            nothing is claimed without the list. The key remounts the draft
            whenever the saved list changes (a save, another Admin). */}
        {entries ? (
          <IPAllowListEditor
            key={ipAllowListEntryKey(entries)}
            entries={entries}
            saving={saving}
            onSave={(next) => save(workspace.id, next)}
            clearOnly={clearOnly}
            readOnly={readOnly}
            emptyBlocks={{
              open: t("workspaces.ipRulesOpen"),
              allowAll: ALLOW_ALL_PAIR_ENTRIES,
            }}
            labels={{
              hint: t("workspaces.ipRulesHint"),
              empty: t("workspaces.ipRulesBlocked"),
              descriptionPlaceholder: t(
                "workspaces.ipRulesDescriptionPlaceholder",
              ),
              cidr: t("workspaces.ipRulesCIDR"),
              cidrRule: (number) => t("workspaces.ipRulesCIDRRule", { number }),
              description: t("workspaces.ipRulesDescriptionLabel"),
              descriptionRule: (number) =>
                t("workspaces.ipRulesDescriptionRule", { number }),
              add: t("workspaces.ipRulesAdd"),
              save: t("workspaces.ipRulesSave"),
              cancel: t("workspaces.ipRulesCancel"),
              invalid: t("workspaces.ipRulesInvalid"),
              remove: (cidr) => t("workspaces.ipRulesRemove", { cidr }),
              moveUp: (cidr) => t("workspaces.ipRulesMoveUp", { cidr }),
              moveDown: (cidr) => t("workspaces.ipRulesMoveDown", { cidr }),
            }}
          />
        ) : (
          <p className="text-sm text-destructive">
            {t("workspaces.ipRulesUnreadable")}
          </p>
        )}
        {refusal ? (
          <Alert variant="destructive">
            <AlertTitle>{t("workspaces.ipRulesSaveErrorTitle")}</AlertTitle>
            <AlertDescription>
              {refusal.kind === "plan" ? (
                <p>
                  {t("workspaces.ipRulesPlanRefused")} {changePlan}
                </p>
              ) : refusal.kind === "forbidden" ? (
                t("workspaces.ipRulesAdminOnly")
              ) : (
                refusal.message
              )}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
    </Card>
  );
}
