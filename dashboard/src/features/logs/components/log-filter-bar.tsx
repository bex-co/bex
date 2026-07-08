import { Search } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { Input } from "@/common/components/ui/input.tsx";
import { Switch } from "@/common/components/ui/switch.tsx";
import { Label } from "@/common/components/ui/label.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import {
  LOG_TYPE_ALL,
  LOG_TYPE_APP,
  LOG_TYPE_FILTERS,
  LOG_TYPE_REQUEST,
  type LogTypeFilter,
} from "../types";

// Render's Logs filter row (design source .pm/w5/m6/README.md): a type dropdown,
// a wide search box, and a live toggle. Time-range/level/status filters Render
// exposes are omitted — bex-api's GraphQL logs() can't honor them over raw pod
// logs (see the milestone's honored-vs-omitted table).
const TYPE_LABEL_KEYS: Record<LogTypeFilter, keyof typeof en> = {
  [LOG_TYPE_ALL]: "logs.typeAll",
  [LOG_TYPE_APP]: "logs.typeApplication",
  [LOG_TYPE_REQUEST]: "logs.typeRequest",
};

interface LogFilterBarProps {
  type: LogTypeFilter;
  onTypeChange: (type: LogTypeFilter) => void;
  text: string;
  onTextChange: (text: string) => void;
  live: boolean;
  onLiveChange: (live: boolean) => void;
}

export function LogFilterBar({
  type,
  onTypeChange,
  text,
  onTextChange,
  live,
  onLiveChange,
}: LogFilterBarProps) {
  const { t } = useTranslations();

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Select
        value={type}
        onValueChange={(v) => onTypeChange(v as LogTypeFilter)}
      >
        <SelectTrigger size="sm" className="w-44" aria-label={t("logs.typeLabel")}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {LOG_TYPE_FILTERS.map((f) => (
            <SelectItem key={f} value={f}>
              {t(TYPE_LABEL_KEYS[f])}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="relative min-w-48 flex-1">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={text}
          onChange={(e) => onTextChange(e.target.value)}
          placeholder={t("logs.searchPlaceholder")}
          aria-label={t("logs.searchPlaceholder")}
          className="h-9 pl-8"
        />
      </div>

      <div className="flex items-center gap-2">
        <Switch
          id="logs-live-toggle"
          checked={live}
          onCheckedChange={onLiveChange}
        />
        <Label htmlFor="logs-live-toggle" className="cursor-pointer text-sm">
          {t("logs.live")}
        </Label>
      </div>
    </div>
  );
}
