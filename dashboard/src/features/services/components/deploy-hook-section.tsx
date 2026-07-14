import { useState } from "react";
import { Eye, EyeOff, RotateCw } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Skeleton } from "@/common/components/ui/skeleton";
import { CopyButton } from "@/common/components/copy-button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeployHook } from "@/features/services/hooks/use-deploy-hook";

export interface DeployHookSectionProps {
  serviceId: string;
}

const MASKED_DEPLOY_HOOK = "••••••••••••••••••••••••••••••••";

/**
 * Settings control for the service's secret Deploy Hook URL (w2/m33): masked by
 * default, reveal/copy on demand, and destructive rotation behind a warning.
 */
export function DeployHookSection({ serviceId }: DeployHookSectionProps) {
  const { t } = useTranslations();
  const { url, loading, error, regenerate, regenerating } =
    useDeployHook(serviceId);
  const [revealed, setRevealed] = useState(false);

  async function handleRegenerate() {
    if (await regenerate()) setRevealed(true);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.deployHookTitle")}</CardTitle>
        <CardDescription>{t("services.deployHookDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading && !url ? (
          <Skeleton className="h-9 w-full" />
        ) : error && !url ? (
          <p className="text-sm text-destructive">
            {t("services.deployHookLoadError")}
          </p>
        ) : (
          <div className="flex items-center gap-2">
            <Input
              aria-label={t("services.deployHookURLLabel")}
              value={revealed ? (url ?? "") : MASKED_DEPLOY_HOOK}
              readOnly
              className="font-mono text-xs"
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              disabled={!url}
              aria-label={
                revealed
                  ? t("services.deployHookHide")
                  : t("services.deployHookReveal")
              }
              onClick={() => setRevealed((shown) => !shown)}
            >
              {revealed ? <EyeOff /> : <Eye />}
            </Button>
            {url ? (
              <CopyButton
                value={url}
                label={t("services.deployHookCopy")}
                successText={t("services.deployHookCopied")}
                errorText={t("services.deployHookCopyError")}
              />
            ) : (
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                disabled
                aria-label={t("services.deployHookCopy")}
              />
            )}
          </div>
        )}

        <p className="text-sm text-muted-foreground">
          {t("services.deployHookSecretHint")}
        </p>

        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              type="button"
              variant="outline"
              disabled={!url || regenerating}
            >
              <RotateCw />
              {t("services.deployHookRegenerate")}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("services.deployHookRegenerateTitle")}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("services.deployHookRegenerateWarning")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>
                {t("services.deployHookCancel")}
              </AlertDialogCancel>
              <AlertDialogAction onClick={() => void handleRegenerate()}>
                {t("services.deployHookRegenerateConfirm")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </CardContent>
    </Card>
  );
}
