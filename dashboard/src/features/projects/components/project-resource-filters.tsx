import { Search } from "lucide-react";
import { Input } from "@/common/components/ui/input";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import type {
  ProjectResourceCounts,
  ProjectResourceFilterState,
  ProjectResourceKind,
} from "@/features/projects/lib/resource-filter";

const KIND_KEYS: Array<{
  kind: ProjectResourceKind;
  key:
    | "projects.filterAll"
    | "projects.filterServices"
    | "projects.filterDatabases"
    | "projects.filterKeyValues"
    | "projects.filterEnvGroups";
}> = [
  { kind: "all", key: "projects.filterAll" },
  { kind: "services", key: "projects.filterServices" },
  { kind: "databases", key: "projects.filterDatabases" },
  { kind: "keyvalues", key: "projects.filterKeyValues" },
  { kind: "envgroups", key: "projects.filterEnvGroups" },
];

export function ProjectResourceFilters({
  environmentName,
  filter,
  counts,
  onChange,
}: {
  environmentName: string;
  filter: ProjectResourceFilterState;
  counts: ProjectResourceCounts;
  onChange: (filter: ProjectResourceFilterState) => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-3">
      <div
        className="flex flex-wrap gap-1"
        aria-label={t("projects.filterTypeLabel")}
      >
        {KIND_KEYS.map(({ kind, key }) => (
          <Button
            key={kind}
            size="sm"
            variant={filter.kind === kind ? "default" : "outline"}
            onClick={() => onChange({ ...filter, kind })}
          >
            {t(key)} ({counts[kind]})
          </Button>
        ))}
      </div>
      <div className="relative">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          type="search"
          value={filter.query}
          onChange={(event) =>
            onChange({ ...filter, query: event.target.value })
          }
          aria-label={t("projects.searchLabel", {
            environment: environmentName,
          })}
          placeholder={t("projects.searchPlaceholder", {
            environment: environmentName,
          })}
          className="pl-8"
        />
      </div>
    </div>
  );
}
