import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
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
import { useCreateWorkspace } from "@/features/workspaces/hooks/use-create-workspace";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";
import {
  WORKSPACE_NAME_RE,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

export const Route = createFileRoute("/new/workspace")({
  staticData: { chrome: true },
  component: NewWorkspacePage,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("workspaces.newTitle", match),
});

/**
 * `/new/workspace` (w6/m3/t002): name + Render's plan cards, then create ->
 * switch -> land in the new (empty) workspace. A plan-limit refusal (the
 * 6th Hobby workspace) surfaces inline via the create hook's `error`, not
 * just a toast — the DoD calls for it right where the user acted.
 */
export function NewWorkspacePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { create, busy, error } = useCreateWorkspace();
  const { setCurrentWorkspaceId } = useWorkspace();

  const [name, setName] = useState("");
  const [plan, setPlan] = useState<WorkspacePlanId>("hobby");

  const nameValid = WORKSPACE_NAME_RE.test(name);
  const showNameError = name.length > 0 && !nameValid;
  const canSubmit = nameValid && !busy;

  async function handleSubmit() {
    if (!canSubmit) return;
    const workspace = await create(name, plan);
    if (!workspace) return;
    // create() adds the returned workspace to the shared list cache before it
    // resolves, so WorkspaceProvider recognizes this id and cannot fall back
    // to the first old workspace while navigation is in flight.
    setCurrentWorkspaceId(workspace.id);
    await navigate({ to: "/", replace: true });
  }

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>
                <h1 className="text-xl font-semibold">
                  {t("workspaces.createTitle")}
                </h1>
              </CardTitle>
              <CardDescription>
                {t("workspaces.createDescription")}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="workspace-name">
                  {t("workspaces.fieldName")}
                </Label>
                <Input
                  id="workspace-name"
                  value={name}
                  onChange={(e) => setName(e.target.value.toLowerCase())}
                  placeholder={t("workspaces.fieldNamePlaceholder")}
                  autoComplete="off"
                  autoFocus
                  aria-invalid={showNameError}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void handleSubmit();
                  }}
                />
                {showNameError ? (
                  <p className="text-sm text-destructive">
                    {t("workspaces.fieldNameError")}
                  </p>
                ) : null}
              </div>

              <div className="space-y-2">
                <Label>{t("workspaces.fieldPlan")}</Label>
                <PlanPicker selected={plan} onSelect={setPlan} />
              </div>

              {error ? (
                <Alert variant="destructive">
                  <AlertTitle>{t("workspaces.createErrorTitle")}</AlertTitle>
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              ) : null}
            </CardContent>
            <CardFooter className="flex justify-end gap-2 border-t pt-4">
              <Button
                variant="outline"
                onClick={() => void navigate({ to: "/" })}
                disabled={busy}
              >
                {t("workspaces.createCancel")}
              </Button>
              <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
                {busy ? <Loader2 className="animate-spin" /> : null}
                {t("workspaces.createSubmit")}
              </Button>
            </CardFooter>
          </Card>
        </div>
      </div>
    </DashboardLayout>
  );
}
