import { Link } from "@tanstack/react-router";
import {
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { SERVICE_TYPE_CREATE_ITEMS } from "@/features/services/lib/create-context";

/**
 * A "New service" submenu that lists each service type (Web Service, Private
 * Service, Background Worker, Cron Job, Static Site) — Render parity: its New
 * menu surfaces service types individually rather than one generic entry. Every
 * item deep-links the create wizard to its type via `?type=`, so e.g. Cron Job
 * lands on a cron-preselected form. Shared by every "New" menu so they don't
 * drift. `triggerLabelKey` lets each host keep its own wording.
 */
export function NewServiceMenuItems({
  triggerLabelKey,
}: {
  triggerLabelKey: string;
}) {
  const { t } = useTranslations();
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger>{t(triggerLabelKey)}</DropdownMenuSubTrigger>
      <DropdownMenuSubContent>
        {SERVICE_TYPE_CREATE_ITEMS.map(({ type, labelKey }) => (
          <DropdownMenuItem key={type} asChild>
            <Link to="/services/new" search={{ type }}>
              {t(labelKey)}
            </Link>
          </DropdownMenuItem>
        ))}
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  );
}
