import { Layers } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";
import { useOpenGenerateBlueprint } from "@/features/blueprints/hooks/use-open-generate-blueprint";
import type { BlueprintExport } from "@/features/blueprints/lib/blueprint-export";

/**
 * Generate Blueprint in a resource row's menu or a project environment's
 * menu (w3/m681): opens the Create Blueprint page with `handed`'s resources,
 * disabled with the role's reason for a role that cannot create a Blueprint.
 */
export function GenerateBlueprintMenuItem({
  handed,
  disabled,
}: {
  handed: BlueprintExport;
  disabled?: boolean;
}) {
  const { t } = useTranslations();
  const { reason, open } = useOpenGenerateBlueprint();
  return (
    <PermissionMenuItem
      permissionReason={reason}
      disabled={disabled}
      onSelect={() => open(handed)}
    >
      <Layers />
      {t("blueprints.generateButton")}
    </PermissionMenuItem>
  );
}
