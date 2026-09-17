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
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeleteService } from "@/features/services/hooks/use-delete-service";
import {
  publiclyRoutable,
  serviceSudoPhrase,
  sudoServiceTypeWords,
} from "@/features/services/lib/service-type";
import type { ServiceView } from "@/features/services/types";

export interface DeleteServiceCardProps {
  service: ServiceView;
}

/**
 * The Settings-tab danger zone (w5/m14): a destructive-bordered card whose
 * "Delete service" button opens a type-to-confirm dialog. Typing Render's
 * exact "sudo delete <type words> <name>" phrase arms the confirm button
 * (live capture: docs/render-artifacts/protected-environments.md) — a typo is
 * a no-op, not a destroyed service. On success the deleted App is evicted
 * from the cache (see useDeleteService) and the user lands back on the
 * services list, where the deleted row is already gone.
 */
export function DeleteServiceCard({ service }: DeleteServiceCardProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { remove, deleting } = useDeleteService();
  const [open, setOpen] = useState(false);
  const [confirmation, setConfirmation] = useState("");
  const [requiredConfirmation, setRequiredConfirmation] = useState<
    string | null
  >(null);
  const expectedConfirmation =
    requiredConfirmation ?? serviceSudoPhrase("delete", service);
  const matches = confirmation === expectedConfirmation;

  async function handleDelete() {
    if (!matches || deleting) return;
    const result = await remove(
      service.id,
      service.name,
      requiredConfirmation ?? undefined,
    );
    if (result.status === "confirmation_required") {
      setRequiredConfirmation(result.confirmation);
      setConfirmation("");
      return;
    }
    if (result.status !== "success") return;
    setOpen(false);
    void navigate({ to: "/", replace: true });
  }

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (!next) {
      setConfirmation("");
      setRequiredConfirmation(null);
    }
  }

  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle className="text-destructive">
          {t("services.dangerZoneTitle")}
        </CardTitle>
        <CardDescription>
          {/* Only a publicly routable type actually has a URL to remove — a
              private service has an internal address and a worker/cron job has
              none at all, so the URL clause would be untrue for them (w6/029). */}
          {t(
            publiclyRoutable(service.type)
              ? "services.dangerZoneDescription"
              : "services.dangerZoneDescriptionNoUrl",
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button variant="destructive" onClick={() => setOpen(true)}>
          {t("services.deleteButton")}
        </Button>
      </CardContent>

      {/* AlertDialog via ConfirmDialog (w7/053). Deleting a service destroys
          data and leaves no second signal that it happened, so it must
          announce as `alertdialog` and refuse outside-click dismissal — a
          stray click should not discard a half-typed sudo phrase. The gate
          stays this card's own SudoCommandField, riding confirmDisabled. */}
      <ConfirmDialog
        open={open}
        onOpenChange={handleOpenChange}
        title={t("services.deleteConfirmTitle", { name: service.name })}
        description={
          requiredConfirmation
            ? t("services.protectedConfirmationBody", { name: service.name })
            : t("services.deleteConfirmBody", {
                type: sudoServiceTypeWords(service),
              })
        }
        cancelLabel={t("services.deleteCancel")}
        confirmLabel={
          <>
            {deleting ? <Loader2 className="animate-spin" /> : null}
            {t("services.deleteConfirm")}
          </>
        }
        confirmDisabled={!matches}
        pending={deleting}
        // A protected environment answers confirmation_required with a phrase
        // the user must type in this same dialog, so it must survive a confirm
        // that did not delete anything. Success navigates away.
        closeOnConfirm={false}
        onConfirm={() => void handleDelete()}
      >
        <SudoCommandField
          id="service-delete-confirm"
          promptKey={
            requiredConfirmation
              ? "services.protectedConfirmationPrompt"
              : "services.deleteConfirmPrompt"
          }
          phrase={expectedConfirmation}
          value={confirmation}
          onValueChange={setConfirmation}
        />
      </ConfirmDialog>
    </Card>
  );
}
