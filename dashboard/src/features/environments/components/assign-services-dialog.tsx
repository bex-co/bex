import { useState } from "react";
import { Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Label } from "@/common/components/ui/label";
import { ScrollArea } from "@/common/components/ui/scroll-area";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { useSetEnvironmentServices } from "@/features/environments/hooks/use-set-environment-services";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import type { ServiceView } from "@/features/services/types";

export interface AssignServicesDialogProps {
  environment: EnvironmentView;
  /** Candidate services — the whole workspace's services (assigning auto-joins the project). */
  services: ServiceView[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * The environment's "Manage services" dialog: a checkbox list of the
 * workspace's services, pre-checked for those already in this environment.
 * Saving full-replaces the environment's membership
 * (`setEnvironmentServices`), which also auto-joins the checked services to the
 * environment's parent project (docs/ADR032-environments.md). One dialog covers
 * both assign and remove.
 */
export function AssignServicesDialog({
  environment,
  services,
  open,
  onOpenChange,
}: AssignServicesDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {/* Radix unmounts Content's children on close, so this form remounts
            each open — its checkbox state seeds fresh from the environment's
            current members without a sync effect. */}
        <AssignServicesForm
          environment={environment}
          services={services}
          onClose={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}

function AssignServicesForm({
  environment,
  services,
  onClose,
}: {
  environment: EnvironmentView;
  services: ServiceView[];
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const { setServices, busyId } = useSetEnvironmentServices();
  const [checked, setChecked] = useState<Set<string>>(
    () => new Set(environment.serviceIds),
  );
  const busy = busyId === environment.id;

  function toggle(id: string, next: boolean) {
    setChecked((prev) => {
      const nextSet = new Set(prev);
      if (next) nextSet.add(id);
      else nextSet.delete(id);
      return nextSet;
    });
  }

  async function handleSave() {
    // The hook refetches Environments + Projects (assignment auto-joins the
    // project), so closing is all that's left on success.
    const ok = await setServices(environment.id, environment.name, [...checked]);
    if (ok) onClose();
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {t("environments.manageTitle", { name: environment.name })}
        </DialogTitle>
        <DialogDescription>
          {t("environments.manageDescription")}
        </DialogDescription>
      </DialogHeader>

      {services.length === 0 ? (
        <p className="py-4 text-sm text-muted-foreground">
          {t("environments.manageNoServices")}
        </p>
      ) : (
        <ScrollArea className="max-h-72 pr-3">
          <div className="space-y-1">
            {services.map((service) => (
              <Label
                key={service.id}
                htmlFor={`assign-${service.id}`}
                className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-2 hover:bg-muted/50"
              >
                <Checkbox
                  id={`assign-${service.id}`}
                  checked={checked.has(service.id)}
                  onCheckedChange={(v) => toggle(service.id, v === true)}
                  disabled={busy}
                />
                <span className="flex-1 font-medium">{service.name}</span>
                <ServiceStatusBadge service={service} />
              </Label>
            ))}
          </div>
        </ScrollArea>
      )}

      <DialogFooter>
        <Button variant="outline" onClick={onClose} disabled={busy}>
          {t("environments.cancel")}
        </Button>
        <Button onClick={() => void handleSave()} disabled={busy}>
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("environments.manageSubmit")}
        </Button>
      </DialogFooter>
    </>
  );
}
