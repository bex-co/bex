import { Link } from "@tanstack/react-router";
import {
  Boxes,
  FolderKanban,
  Layers,
  Package,
  Plus,
  Webhook,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { CreateTypeMenuItems } from "@/common/components/dashboard-layout/create-type-menu-items";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

/**
 * The global counterpart to Render's persistent “+ New” topbar menu: flat, as
 * Render's is — the create types, then bex's other creates.
 */
export function NewResourceMenu() {
  const { t } = useTranslations();
  const { canCreate } = useCapabilities();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="gap-1.5 px-2 sm:px-3">
          <Plus />
          {/* One label, visually hidden below sm: the menu takes its name
              from the trigger's text, hidden copies included, so two spans
              named it "New New" (w4/545). */}
          <span className="sr-only sm:not-sr-only">
            {t("common.topbarNew")}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <CreateTypeMenuItems />
        <DropdownMenuSeparator />
        {/* Render's bare names ("Project", "Blueprint"), then bex's own. */}
        <DropdownMenuItem asChild>
          <Link to="/" search={{ new: "project" }}>
            <FolderKanban />
            {t("common.createTypeProject")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/blueprint/new">
            <Layers />
            {t("common.createTypeBlueprint")}
          </Link>
        </DropdownMenuItem>
        {canCreate ? (
          <DropdownMenuItem asChild>
            <Link to="/build-sources/new">
              <Package />
              {t("common.createTypeBuildSource")}
            </Link>
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem asChild>
          <Link to="/" search={{ new: "sandbox-group" }}>
            <Boxes />
            {t("common.createTypeSandboxGroup")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/webhooks/new">
            <Webhook />
            {t("common.createTypeWebhook")}
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
