import { useState } from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useKeyValueNetworking } from "@/features/keyvalue/hooks/use-key-value-networking";

/**
 * The Key Value detail's Networking section: the external-endpoint IP
 * allowlist (editable CIDR list) — Render's Networking control, mirroring the
 * allowlist section of databases' AccessControlPanel. The allowlist only gates
 * the public SNI endpoint, so an internal-only store shows a note instead of
 * pretending the list has any effect.
 */
export function KeyValueNetworkingPanel({
  id,
  isPublic,
}: {
  id: string;
  isPublic: boolean;
}) {
  const { t } = useTranslations();
  const networking = useKeyValueNetworking(id);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.networkingTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.networkingDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        {isPublic ? null : (
          <p className="text-xs text-muted-foreground">
            {t("keyvalue.networkingInternalOnly")}
          </p>
        )}
        {/* key remounts the editable draft from the server list whenever it
            changes (e.g. after a save), avoiding an effect-based state sync. */}
        <AllowListEditor
          key={networking.allowList.join(",")}
          networking={networking}
        />
      </CardContent>
    </Card>
  );
}

type Networking = ReturnType<typeof useKeyValueNetworking>;

function AllowListEditor({ networking }: { networking: Networking }) {
  const { t } = useTranslations();
  // Local editable copy, seeded from the server list; dirty until saved.
  const [draft, setDraft] = useState<string[]>(networking.allowList);
  const [entry, setEntry] = useState("");

  const dirty =
    draft.length !== networking.allowList.length ||
    draft.some((c, i) => c !== networking.allowList[i]);

  function add() {
    const c = entry.trim();
    if (c && !draft.includes(c)) setDraft([...draft, c]);
    setEntry("");
  }

  return (
    <section className="space-y-2">
      <p className="text-xs text-muted-foreground">
        {t("keyvalue.networkingHint")}
      </p>
      <div className="flex flex-wrap gap-2">
        {draft.length === 0 ? (
          <span className="text-sm text-muted-foreground">
            {t("keyvalue.networkingOpen")}
          </span>
        ) : (
          draft.map((c) => (
            <span
              key={c}
              className="inline-flex items-center gap-1 rounded-md border bg-muted px-2 py-1 text-xs"
            >
              <code className="font-mono">{c}</code>
              <button
                type="button"
                aria-label={t("keyvalue.networkingRemove", { cidr: c })}
                onClick={() => setDraft(draft.filter((x) => x !== c))}
                className="text-muted-foreground hover:text-foreground"
              >
                <Trash2 className="size-3" />
              </button>
            </span>
          ))
        )}
      </div>
      <div className="flex gap-2">
        <Input
          value={entry}
          onChange={(e) => setEntry(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), add())}
          placeholder="203.0.113.0/24"
          className="max-w-xs"
        />
        <Button
          variant="outline"
          size="sm"
          onClick={add}
          disabled={!entry.trim()}
        >
          <Plus />
          {t("keyvalue.networkingAdd")}
        </Button>
        <Button
          size="sm"
          onClick={() => void networking.saveAllowList(draft)}
          disabled={!dirty || networking.savingAllowList}
        >
          {networking.savingAllowList ? (
            <Loader2 className="animate-spin" />
          ) : null}
          {t("keyvalue.networkingSave")}
        </Button>
      </div>
    </section>
  );
}
