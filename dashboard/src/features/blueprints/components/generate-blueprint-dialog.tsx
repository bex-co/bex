import { useMemo, useState } from "react";
import { useLazyQuery } from "@apollo/client/react";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useServices } from "@/features/services/hooks/use-services";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { GenerateBlueprintDocument } from "@/features/blueprints/api/operations";

function SelectableList({
  label,
  items,
  selected,
  onToggle,
}: {
  label: string;
  items: { id: string; name: string }[];
  selected: Set<string>;
  onToggle: (id: string, next: boolean) => void;
}) {
  if (items.length === 0) return null;
  return (
    <div className="space-y-1.5">
      <p className="text-sm font-medium">{label}</p>
      <div className="max-h-36 space-y-1 overflow-y-auto rounded-md border p-2">
        {items.map((item) => (
          <label
            key={item.id}
            className="flex cursor-pointer items-center gap-2 text-sm"
          >
            <Checkbox
              checked={selected.has(item.id)}
              onCheckedChange={(next) => onToggle(item.id, next === true)}
            />
            <span className="truncate">{item.name}</span>
          </label>
        ))}
      </div>
    </div>
  );
}

/**
 * Render's "Generate Blueprint" for bex (w8/m22): select existing resources,
 * preview the generated render.yaml (secret values omitted as sync:false),
 * copy or download it. Nothing is persisted server-side.
 */
export function GenerateBlueprintDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const { services } = useServices();
  const { databases } = useDatabases();
  const { keyValues } = useKeyValues();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [manifest, setManifest] = useState<string | null>(null);
  const [generate, { loading }] = useLazyQuery(GenerateBlueprintDocument, {
    fetchPolicy: "no-cache",
  });

  const serviceIds = useMemo(
    () => services.filter((s) => selected.has(s.id)).map((s) => s.id),
    [services, selected],
  );
  const postgresIds = useMemo(
    () => databases.filter((d) => selected.has(d.id)).map((d) => d.id),
    [databases, selected],
  );
  const keyValueIds = useMemo(
    () => keyValues.filter((k) => selected.has(k.id)).map((k) => k.id),
    [keyValues, selected],
  );
  const selectionEmpty =
    serviceIds.length + postgresIds.length + keyValueIds.length === 0;

  function toggle(id: string, next: boolean) {
    setSelected((current) => {
      const out = new Set(current);
      if (next) {
        out.add(id);
      } else {
        out.delete(id);
      }
      return out;
    });
    setManifest(null);
  }

  async function handleGenerate() {
    try {
      const res = await generate({
        variables: {
          ownerId: currentWorkspaceId,
          serviceIds,
          postgresIds,
          keyValueIds,
        },
      });
      const out = res.data?.generateBlueprint;
      if (!out) {
        toast.error(t("blueprints.generateError"));
        return;
      }
      setManifest(out.manifest);
    } catch {
      toast.error(t("blueprints.generateError"));
    }
  }

  function handleDownload() {
    if (!manifest) return;
    const blob = new Blob([manifest], { type: "application/yaml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "render.yaml";
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (!next) {
          setSelected(new Set());
          setManifest(null);
        }
      }}
    >
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("blueprints.generateTitle")}</DialogTitle>
          <DialogDescription>
            {t("blueprints.generateDescription")}
          </DialogDescription>
        </DialogHeader>
        {manifest === null ? (
          <div className="space-y-3">
            <SelectableList
              label={t("blueprints.previewServices")}
              items={services}
              selected={selected}
              onToggle={toggle}
            />
            <SelectableList
              label={t("blueprints.previewDatabases")}
              items={databases}
              selected={selected}
              onToggle={toggle}
            />
            <SelectableList
              label={t("blueprints.previewKeyValue")}
              items={keyValues}
              selected={selected}
              onToggle={toggle}
            />
            {selectionEmpty ? (
              <p className="text-sm text-muted-foreground">
                {t("blueprints.generateEmptyHint")}
              </p>
            ) : null}
          </div>
        ) : (
          <div className="space-y-2">
            <pre className="max-h-72 overflow-auto rounded-md border bg-muted/50 p-3 text-xs">
              {manifest}
            </pre>
            <p className="text-sm text-muted-foreground">
              {t("blueprints.generateSecretsNote")}
            </p>
          </div>
        )}
        <DialogFooter>
          {manifest === null ? (
            <Button
              onClick={() => void handleGenerate()}
              disabled={selectionEmpty || loading}
            >
              {loading ? <Loader2 className="animate-spin" /> : null}
              {t("blueprints.generateAction")}
            </Button>
          ) : (
            <>
              <Button variant="outline" onClick={() => setManifest(null)}>
                {t("blueprints.generateBack")}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  void navigator.clipboard.writeText(manifest);
                  toast.success(t("blueprints.generateCopied"));
                }}
              >
                {t("blueprints.generateCopy")}
              </Button>
              <Button onClick={handleDownload}>
                {t("blueprints.generateDownload")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
