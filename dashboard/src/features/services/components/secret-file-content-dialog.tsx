import { useEffect, useState } from "react";
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
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";

export function SecretFileContentDialog({
  open,
  name,
  content,
  disabled = false,
  reveal,
  onOpenChange,
  onSave,
}: {
  open: boolean;
  name: string;
  content: string | null;
  disabled?: boolean;
  reveal?: () => Promise<string>;
  onOpenChange: (open: boolean) => void;
  onSave: (content: string, changed: boolean) => void;
}) {
  const { t } = useTranslations();
  const [value, setValue] = useState(content ?? "");
  const [initial, setInitial] = useState(content ?? "");
  const [loading, setLoading] = useState(content == null && Boolean(reveal));
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!open || content != null || !reveal) return;
    let active = true;
    void reveal()
      .then((revealed) => {
        if (!active) return;
        setValue(revealed);
        setInitial(revealed);
      })
      .catch(() => {
        if (active) setError(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [content, open, reveal]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {t("services.secretFileContentDialogTitle", { name })}
          </DialogTitle>
          <DialogDescription>
            {t("services.secretFileContentDialogDescription")}
          </DialogDescription>
        </DialogHeader>
        {loading ? (
          <div className="text-muted-foreground flex min-h-48 items-center justify-center gap-2 text-sm">
            <Loader2 className="animate-spin" />{" "}
            {t("services.secretFileLoadingContent")}
          </div>
        ) : (
          <Textarea
            value={value}
            disabled={disabled}
            onChange={(event) => setValue(event.target.value)}
            className="min-h-64 font-mono text-sm"
            aria-label={t("services.secretFileColContent")}
            aria-invalid={error}
            autoFocus
          />
        )}
        {error ? (
          <p className="text-destructive text-sm" role="alert">
            {t("services.secretFileRevealError")}
          </p>
        ) : null}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("services.envCancel")}
          </Button>
          <Button
            disabled={loading || error || disabled}
            onClick={() => {
              if (disabled) return;
              onSave(value, value !== initial);
              onOpenChange(false);
            }}
          >
            {t("services.secretFileContentDone")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
