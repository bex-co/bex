import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Button } from "@/common/components/ui/button";
import {
  Alert,
  AlertTitle,
  AlertDescription,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeleteWorkspace } from "@/features/workspaces/hooks/use-delete-workspace";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  workspaceDeleteConfirmation,
  type WorkspaceView,
} from "@/features/workspaces/types";

export interface DeleteWorkspaceCardProps {
  workspace: WorkspaceView;
}

/**
 * The danger zone (w6/m3/t004, phrase-aligned in w6/m5/t002): typing Render's
 * "sudo delete workspace <name>" phrase arms the delete button — a typo is a
 * no-op, not a destroyed workspace (mirrors the backend's own confirmation
 * check, backend/internal/workspaces/service.go Delete). On success, lands the
 * caller in a remaining workspace (chosen from the pre-delete list — the deleted
 * one is already gone from any list a refetch would return) or `/new/workspace`
 * if none is left.
 */
export function DeleteWorkspaceCard({ workspace }: DeleteWorkspaceCardProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { remove, busy, error } = useDeleteWorkspace();
  const { workspaces, setCurrentWorkspaceId, refetch } = useWorkspace();

  const confirmPhrase = workspaceDeleteConfirmation(workspace.name);
  const [confirmation, setConfirmation] = useState("");
  const matches = confirmation === confirmPhrase;

  async function handleDelete() {
    if (!matches || busy) return;
    const ok = await remove(workspace.id, workspace.name, confirmPhrase);
    if (!ok) return;
    const fallback = workspaces.find((w) => w.id !== workspace.id);
    void refetch();
    if (fallback) {
      setCurrentWorkspaceId(fallback.id);
      void navigate({ to: "/" });
    } else {
      void navigate({ to: "/new/workspace" });
    }
  }

  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle className="text-destructive">
          {t("workspaces.dangerZoneTitle")}
        </CardTitle>
        <CardDescription>
          {t("workspaces.dangerZoneDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="delete-workspace-confirm">
            {t("workspaces.deleteConfirmLabel", { phrase: confirmPhrase })}
          </Label>
          <Input
            id="delete-workspace-confirm"
            value={confirmation}
            onChange={(e) => setConfirmation(e.target.value)}
            placeholder={confirmPhrase}
            autoComplete="off"
            className="max-w-sm"
          />
        </div>

        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("workspaces.deleteErrorTitle")}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}

        <Button
          variant="destructive"
          onClick={() => void handleDelete()}
          disabled={!matches || busy}
        >
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("workspaces.deleteSubmit")}
        </Button>
      </CardContent>
    </Card>
  );
}
