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
          <span className="hidden sm:inline">{t("common.topbarNew")}</span>
          <span className="sr-only sm:hidden">{t("common.topbarNew")}</span>
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
          <Link to="/webhooks/new">{t("webhooks.newTitle")}</Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
