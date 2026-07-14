import { useState } from "react";
import { Loader2, Plus } from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Checkbox } from "@/common/components/ui/checkbox";
import { ScrollArea } from "@/common/components/ui/scroll-area";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCreateWebhook } from "@/features/webhooks/hooks/use-create-webhook";
import { useWebhookEventTypes } from "@/features/webhooks/hooks/use-webhook-event-types";
import type { CreatedWebhookEndpoint } from "@/features/webhooks/types";

export interface CreateWebhookDialogProps {
  /** Called once a create succeeds (before the secret dialog is dismissed). */
  onCreated: () => void;
}

/**
 * The register flow (w3/m11/t007): name + destination URL + an event-type
 * checkbox list (served by the backend, never hardcoded), then the signing
 * secret shown exactly once with a copy affordance and an explicit "won't see
 * this again" warning. `created` is local component state only — never
 * written to Apollo's cache (the hook uses `fetchPolicy: "no-cache"`) — and
 * is discarded on close, so there is no path to re-display a secret after the
 * dialog dismisses. Uncontrolled dialog: Radix fires `onOpenChange` for every
 * close path, which is all the reset needs (the API-key mint dialog pattern).
 */
export function CreateWebhookDialog({ onCreated }: CreateWebhookDialogProps) {
  const { t } = useTranslations();
  const { create, busy } = useCreateWebhook();
  const { eventTypes, loading: typesLoading } = useWebhookEventTypes();

  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [checked, setChecked] = useState<Set<string>>(() => new Set());
  const [created, setCreated] = useState<CreatedWebhookEndpoint | null>(null);

  function toggle(eventType: string, next: boolean) {
    setChecked((prev) => {
      const nextSet = new Set(prev);
      if (next) nextSet.add(eventType);
      else nextSet.delete(eventType);
      return nextSet;
    });
  }

  const submittable = url.trim() !== "" && checked.size > 0 && !busy;

  async function handleSubmit() {
    if (!submittable) return;
    const endpoint = await create(name.trim(), url.trim(), [...checked]);
    if (endpoint) {
      setCreated(endpoint);
      onCreated(); // refresh the list now; the secret dialog stays open independently
    }
  }

  return (
    <Dialog
      onOpenChange={(next) => {
        if (!next) {
          setName("");
          setUrl("");
          setChecked(new Set());
          setCreated(null);
        }
      }}
    >
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Plus /> {t("webhooks.create")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        {created ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("webhooks.createdTitle")}</DialogTitle>
              <DialogDescription>
                {t("webhooks.createdWarning")}
              </DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2 rounded-md border bg-muted/50 p-3">
              <code className="flex-1 overflow-x-auto font-mono text-sm break-all">
                {created.secret}
              </code>
              <CopyButton
                value={created.secret}
                label={t("webhooks.copy")}
                successText={t("webhooks.copied")}
                errorText={t("webhooks.copyError")}
              />
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button>{t("webhooks.createdDone")}</Button>
              </DialogClose>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("webhooks.createTitle")}</DialogTitle>
              <DialogDescription>
                {t("webhooks.createDescription")}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="webhook-name">{t("webhooks.fieldName")}</Label>
                <Input
                  id="webhook-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t("webhooks.fieldNamePlaceholder")}
                  autoComplete="off"
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="webhook-url">{t("webhooks.fieldUrl")}</Label>
                <Input
                  id="webhook-url"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="https://example.com/webhooks/bex"
                  autoComplete="off"
                  inputMode="url"
                />
              </div>
              <div className="space-y-2">
                <Label>{t("webhooks.fieldEvents")}</Label>
                {typesLoading && eventTypes.length === 0 ? (
                  <p className="py-2 text-sm text-muted-foreground">
                    {t("webhooks.eventsLoading")}
                  </p>
                ) : (
                  <ScrollArea className="max-h-56 rounded-md border pr-3">
                    <div className="space-y-1 p-1">
                      {eventTypes.map((eventType) => (
                        <Label
                          key={eventType}
                          htmlFor={`webhook-event-${eventType}`}
                          className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-2 hover:bg-muted/50"
                        >
                          <Checkbox
                            id={`webhook-event-${eventType}`}
                            checked={checked.has(eventType)}
                            onCheckedChange={(v) => toggle(eventType, v === true)}
                            disabled={busy}
                          />
                          <span className="flex-1 font-mono text-sm">
                            {eventType}
                          </span>
                        </Label>
                      ))}
                    </div>
                  </ScrollArea>
                )}
              </div>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline" disabled={busy}>
                  {t("webhooks.createCancel")}
                </Button>
              </DialogClose>
              <Button onClick={() => void handleSubmit()} disabled={!submittable}>
                {busy ? <Loader2 className="animate-spin" /> : null}
                {t("webhooks.createSubmit")}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
