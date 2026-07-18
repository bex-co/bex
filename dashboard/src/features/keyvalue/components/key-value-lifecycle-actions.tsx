import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button, buttonVariants } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useKeyValueLifecycle } from "@/features/keyvalue/hooks/use-key-value-lifecycle";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface KeyValueLifecycleActionsProps {
  keyValue: KeyValueView;
  /** Called after a successful suspend/resume so the caller can refetch. */
  onChanged: () => void;
}

/**
 * The detail page's suspend/resume control, matching Render's captured KV
 * detail "Suspend Key Value Instance" action (docs/render-artifacts/key-value.md)
 * — bex's scale-to-zero via `spec.suspended` (w2/m7). Suspend is disruptive
 * (drops connections), so it requires Render's exact typed sudo confirmation;
 * resume is a safe recovery and runs immediately.
 */
export function KeyValueLifecycleActions({
  keyValue,
  onChanged,
}: KeyValueLifecycleActionsProps) {
  const { t } = useTranslations();
  const { pending, run } = useKeyValueLifecycle();
  const [confirmSuspend, setConfirmSuspend] = useState(false);
  const [confirmation, setConfirmation] = useState("");
  const busy = pending !== null;
  const confirmPhrase = `sudo suspend key value ${keyValue.name}`;
  const canSuspend = confirmation === confirmPhrase && !busy;

  async function handleSuspend() {
    if (!canSuspend) return;
    handleOpenChange(false);
    const ok = await run("suspend", keyValue.id, keyValue.name);
    if (ok) onChanged();
  }

  async function handleResume() {
    const ok = await run("resume", keyValue.id, keyValue.name);
    if (ok) onChanged();
  }

  function handleOpenChange(open: boolean) {
    setConfirmSuspend(open);
    if (!open) setConfirmation("");
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.lifecycleTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.lifecycleDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {keyValue.suspended ? (
          <Button onClick={() => void handleResume()} disabled={busy}>
            {pending === "resume" ? <Loader2 className="animate-spin" /> : null}
            {t("keyvalue.actionResume")}
          </Button>
        ) : (
          <Button
            variant="destructive"
            onClick={() => handleOpenChange(true)}
            disabled={busy}
          >
            {pending === "suspend" ? (
              <Loader2 className="animate-spin" />
            ) : null}
            {t("keyvalue.actionSuspend")}
          </Button>
        )}
      </CardContent>

      <AlertDialog open={confirmSuspend} onOpenChange={handleOpenChange}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("keyvalue.confirmSuspendTitle", { name: keyValue.name })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("keyvalue.confirmSuspendBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-2">
            <Label htmlFor="kv-suspend-confirm">
              {t("keyvalue.confirmSuspendPrompt", { phrase: confirmPhrase })}
            </Label>
            <Input
              id="kv-suspend-confirm"
              value={confirmation}
              onChange={(event) => setConfirmation(event.target.value)}
              autoComplete="off"
              placeholder={confirmPhrase}
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("keyvalue.confirmCancel")}</AlertDialogCancel>
            <AlertDialogAction
              className={buttonVariants({ variant: "destructive" })}
              onClick={() => void handleSuspend()}
              disabled={!canSuspend}
            >
              {t("keyvalue.actionSuspend")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
