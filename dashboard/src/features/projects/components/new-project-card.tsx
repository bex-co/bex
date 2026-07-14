import { Plus } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";

export interface NewProjectCardProps {
  onClick: () => void;
}

/** The dashed "+ Create new project" tile at the end of the project grid
 * (Render parity — the workspace homepage's project grid ends with one). */
export function NewProjectCard({ onClick }: NewProjectCardProps) {
  const { t } = useTranslations();
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed p-5 text-sm text-muted-foreground transition-colors hover:bg-accent/50"
    >
      <Plus className="size-4" />
      {t("projects.newProjectCard")}
    </button>
  );
}
