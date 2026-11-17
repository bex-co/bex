import { useState } from "react";
import { Card, CardContent } from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { useSetWorkspacePrivacy } from "@/features/workspaces/hooks/use-set-workspace-privacy";
import {
  WorkspacePrivacyCardHeader,
  WorkspacePrivacyOption,
} from "@/features/workspaces/components/workspace-privacy-card-skeleton";
import {
  DEFAULT_WORKSPACE_PRIVACY,
  WORKSPACE_PRIVACY_OPTIONS,
  type WorkspacePrivacy,
  type WorkspaceView,
} from "@/features/workspaces/types";

/**
 * Workspace Settings → Workspace privacy (w7/m464, ADR115), Render's setting
 * for a workspace in an organization: Public (any org member can add
 * themselves) or Invite-only (only workspace admins invite people), set by the
 * workspace's admins. The route mounts it only for an org workspace and only
 * for an admin (`can_manage`), so it has no read-only variant;
 * `WorkspacePrivacyCardSkeleton` previews this anatomy.
 */
export function WorkspacePrivacyCard({
  workspace,
}: {
  workspace: WorkspaceView;
}) {
  // Keyed on the workspace: the switcher swaps the prop without remounting,
  // and a draft must never carry over to another workspace.
  return <WorkspacePrivacyForm key={workspace.id} workspace={workspace} />;
}

function WorkspacePrivacyForm({ workspace }: { workspace: WorkspaceView }) {
  const { t } = useTranslations();
  const { save, saving } = useSetWorkspacePrivacy(workspace.id);
  // null follows the saved setting, so a save (merged into the cached
  // workspace) re-syncs it.
  const [draft, setDraft] = useState<WorkspacePrivacy | null>(null);
  // An org workspace always carries one.
  const saved = workspace.privacy ?? DEFAULT_WORKSPACE_PRIVACY;
  const selected = draft ?? saved;
  const radio = useCardRadioGroup(
    WORKSPACE_PRIVACY_OPTIONS,
    selected,
    setDraft,
    saving,
  );

  return (
    <Card>
      <WorkspacePrivacyCardHeader />
      <CardContent className="space-y-4">
        <div
          role="radiogroup"
          aria-label={t("workspaces.privacyTitle")}
          aria-disabled={saving || undefined}
          className="grid grid-cols-1 gap-2"
        >
          {WORKSPACE_PRIVACY_OPTIONS.map((option) => {
            const { ref, ...keyboard } = radio(option);
            return (
              <WorkspacePrivacyOption
                key={option}
                {...keyboard}
                buttonRef={ref}
                privacy={option}
                checked={option === selected}
                disabled={saving}
                onClick={() => setDraft(option)}
              />
            );
          })}
        </div>
        <Button
          size="sm"
          disabled={saving || selected === saved}
          onClick={async () => {
            if (await save(selected)) setDraft(null);
          }}
        >
          {saving ? t("workspaces.privacySaving") : t("workspaces.privacySave")}
        </Button>
      </CardContent>
    </Card>
  );
}
