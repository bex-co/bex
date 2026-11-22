import { Fragment } from "react";
import { Link } from "@tanstack/react-router";
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { CREATE_TYPE_ITEMS } from "@/common/components/dashboard-layout/create-type-items";

/** Where the datastores start, after the service types. */
const FIRST_DATASTORE = CREATE_TYPE_ITEMS.findIndex((item) => !item.service);

/**
 * The create types as Render's New menu lists them: flat, each with its icon,
 * the service types, a rule, then the datastores, each linking its create
 * page (live capture 2026-11-21). Shared by every "New"
 * menu so they cannot drift; each host adds its own entries after.
 */
export function CreateTypeMenuItems() {
  const { t } = useTranslations();
  return (
    <>
      {CREATE_TYPE_ITEMS.map(({ id, labelKey, icon: Icon, to }, index) => (
        <Fragment key={id}>
          {index === FIRST_DATASTORE ? <DropdownMenuSeparator /> : null}
          <DropdownMenuItem asChild>
            <Link to={to}>
              <Icon />
              {t(labelKey)}
            </Link>
          </DropdownMenuItem>
        </Fragment>
      ))}
    </>
  );
}
