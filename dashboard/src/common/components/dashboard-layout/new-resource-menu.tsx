import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { NewServiceMenuItems } from "@/features/services/components/new-service-menu-items";

/** The global counterpart to Render's persistent “+ New” topbar menu. */
export function NewResourceMenu() {
  const { t } = useTranslations();
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
        <DropdownMenuLabel>{t("common.topbarCreate")}</DropdownMenuLabel>
        <NewServiceMenuItems triggerLabelKey="services.createTitle" />
        <DropdownMenuItem asChild>
          <Link to="/" search={{ new: "database" }}>
            {t("databases.createTitle")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/keyvalue/new">{t("keyvalue.createTitle")}</Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/blueprints/new">{t("blueprints.createTitle")}</Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/" search={{ new: "project" }}>
            {t("projects.createTitle")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/" search={{ new: "sandbox-group" }}>
            {t("sandboxGroups.createTitle")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/webhooks/new">{t("webhooks.newMenuLabel")}</Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
