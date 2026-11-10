import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import { parseTagInput } from "@/features/agent-sessions/lib/tags";

/**
 * Replaces all of a session's tags (w7/m296, Devin's PUT …/tags). The server
 * owns the rules (at most 20, each [a-z0-9:_-]{1,64}) and its coded error is
 * shown inline; an empty field clears the tags. The caller mounts it only
 * while open, so each opening starts from the session's current tags.
 */
export function EditTagsDialog({
  sessionId,
  tags,
  onOpenChange,
}: {
  sessionId: string;
  tags: readonly string[];
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  const { setTags } = useAgentSessionMutations();
  const [text, setText] = useState(() => tags.join(", "));
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  function handleOpenChange(next: boolean) {
    if (!saving) onOpenChange(next);
  }

  async function handleSave() {
    const next = parseTagInput(text);
    if (next.join("\n") === tags.join("\n")) {
      onOpenChange(false);
      return;
    }
    setSaving(true);
    setError(null);
    try {
      // The mutation writes the server's view into the cache, so the header
      // and lists update without a refetch.
      await setTags(sessionId, next);
      toast.success(t("agentSessions.editTagsSaved"));
      onOpenChange(false);
    } catch (err) {
      setError(agentSessionErrorMessage(err, t));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent>
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSave();
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("agentSessions.editTagsTitle")}</DialogTitle>
            <DialogDescription>
              {t("agentSessions.editTagsDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Input
              value={text}
              onChange={(event) => setText(event.target.value)}
              aria-label={t("agentSessions.tagsLabel")}
              aria-invalid={error != null}
              aria-describedby="edit-tags-hint"
              autoFocus
            />
            <p id="edit-tags-hint" className="text-muted-foreground text-xs">
              {t("agentSessions.editTagsHint")}
            </p>
            {error ? (
              <p role="alert" className="text-destructive text-sm">
                {error}
              </p>
            ) : null}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={saving}>
              {saving ? <Loader2 className="animate-spin" /> : null}
              {t("agentSessions.editTagsSave")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
