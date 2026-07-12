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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeleteService } from "@/features/services/hooks/use-delete-service";
import type { ServiceView } from "@/features/services/types";

export interface DeleteServiceCardProps {
  service: ServiceView;
}

/**
 * The Settings-tab danger zone (w5/m14): a destructive-bordered card whose
 * "Delete service" button opens a type-to-confirm dialog. Typing the service's
 * exact name arms the confirm button — a typo is a no-op, not a destroyed
 * service (Render's own delete flow gates on the same typed name). On success
 * the deleted App is evicted from the cache (see useDeleteService) and the user
 * lands back on the services list, where the deleted row is already gone.
 */
export function DeleteServiceCard({ service }: DeleteServiceCardProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { remove, deleting } = useDeleteService();
  const [open, setOpen] = useState(false);
  const [confirmation, setConfirmation] = useState("");
  const matches = confirmation === service.name;

  async function handleDelete() {
    if (!matches || deleting) return;
    const ok = await remove(service.id, service.name);
    if (!ok) return;
    setOpen(false);
    void navigate({ to: "/" });
  }

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (!next) setConfirmation("");
  }

  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle className="text-destructive">
          {t("services.dangerZoneTitle")}
        </CardTitle>
        <CardDescription>
          {t("services.dangerZoneDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button variant="destructive" onClick={() => setOpen(true)}>
          {t("services.deleteButton")}
        </Button>
      </CardContent>

      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("services.deleteConfirmTitle", { name: service.name })}
            </DialogTitle>
            <DialogDescription>
              {t("services.deleteConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="service-delete-confirm">
              {t("services.deleteConfirmPrompt", { name: service.name })}
            </Label>
            <Input
              id="service-delete-confirm"
              value={confirmation}
              onChange={(e) => setConfirmation(e.target.value)}
              autoComplete="off"
              placeholder={service.name}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={deleting}
            >
              {t("services.deleteCancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => void handleDelete()}
              disabled={!matches || deleting}
            >
              {deleting ? <Loader2 className="animate-spin" /> : null}
              {t("services.deleteConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
