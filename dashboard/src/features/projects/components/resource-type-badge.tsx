import { Badge } from "@/common/components/ui/badge.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import type { ResourceKind } from "@/features/projects/types";

const LABEL_KEY: Record<ResourceKind, string> = {
  service: "projects.typeService",
  database: "projects.typeDatabase",
  keyvalue: "projects.typeKeyValue",
  envgroup: "projects.typeEnvGroup",
};

/**
 * The "Type" column badge on the unified Projects page's merged table — a
 * service, database, or key-value row all share one `<Table>`, so this badge
 * is what tells them apart (Render parity: a project page lists every
 * resource kind together).
 */
export function ResourceTypeBadge({ kind }: { kind: ResourceKind }) {
  const { t } = useTranslations();
  return <Badge variant="outline">{t(LABEL_KEY[kind])}</Badge>;
}
