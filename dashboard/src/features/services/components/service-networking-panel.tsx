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
import { useServiceNetworking } from "@/features/services/hooks/use-service-networking";

/**
 * The service Settings Networking section: the inbound IP allowlist for
 * `web_service` and `static_site` (w7/m32, Render's ipAllowList). Mirrors
 * the keyvalue Networking panel in shape. The list is read from the service
 * detail already fetched by `useServer`; the panel only writes via mutation.
 */
export function ServiceNetworkingPanel({
  serviceId,
  currentAllowList,
  onSaved,
}: {
  serviceId: string;
  /** The current flat CIDR list from the service detail query (may be null/empty). */
  currentAllowList: Array<string | null> | null | undefined;
  /** Called after a successful save so the parent can refetch if needed. */
  onSaved?: () => void;
}) {
  const { t } = useTranslations();
  const networking = useServiceNetworking();

  const normalized = (currentAllowList ?? []).filter(
    (c): c is string => typeof c === "string",
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.networkingTitle")}</CardTitle>
        <CardDescription>{t("services.networkingDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        {/* key remounts the editable draft whenever the server list changes */}
        <AllowListEditor
          key={normalized.join(",")}
          serviceId={serviceId}
          serverList={normalized}
          networking={networking}
          onSaved={onSaved}
        />
      </CardContent>
    </Card>
  );
}

type Networking = ReturnType<typeof useServiceNetworking>;

function AllowListEditor({
  serviceId,
  serverList,
  networking,
  onSaved,
}: {
  serviceId: string;
  serverList: string[];
  networking: Networking;
  onSaved?: () => void;
}) {
  const { t } = useTranslations();
  const [draft, setDraft] = useState<string[]>(serverList);
  const [entry, setEntry] = useState("");

  const dirty =
    draft.length !== serverList.length ||
    draft.some((c, i) => c !== serverList[i]);

  function add() {
    const c = entry.trim();
    if (c && !draft.includes(c)) setDraft([...draft, c]);
    setEntry("");
  }

  async function save() {
    const ok = await networking.saveAllowList(serviceId, draft);
    if (ok) onSaved?.();
  }

  return (
    <section className="space-y-2">
      <p className="text-xs text-muted-foreground">
        {t("services.networkingHint")}
      </p>
      <div className="flex flex-wrap gap-2">
        {draft.length === 0 ? (
          <span className="text-sm text-muted-foreground">
            {t("services.networkingOpen")}
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
                aria-label={t("services.networkingRemove", { cidr: c })}
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
          {t("services.networkingAdd")}
        </Button>
        <Button
          size="sm"
          onClick={() => void save()}
          disabled={!dirty || networking.saving}
        >
          {networking.saving ? <Loader2 className="animate-spin" /> : null}
          {t("services.networkingSave")}
        </Button>
      </div>
    </section>
  );
}
