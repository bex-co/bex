import { useMemo, useState } from "react";
import { ListFilter } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { ScrollArea } from "@/common/components/ui/scroll-area";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  SERVICE_EVENT_GROUPS,
  SERVICE_EVENT_TYPES,
  serviceEventLabelKey,
} from "@/features/events/service-event-catalog";

type TriState = boolean | "indeterminate";

function triState(selected: number, total: number): TriState {
  if (selected === 0) return false;
  return selected === total ? true : "indeterminate";
}

export function ServiceEventFilter({
  value,
  onChange,
}: {
  value: Set<string>;
  onChange: (next: Set<string>) => void;
}) {
  const { t } = useTranslations();
  const [search, setSearch] = useState("");
  const needle = search.trim().toLowerCase();
  const visible = useMemo(
    () =>
      SERVICE_EVENT_GROUPS.flatMap((group) => {
        const groupLabel = t(`services.eventsFilterGroup.${group.key}`);
        if (groupLabel.toLowerCase().includes(needle)) return [group];
        const types = group.types.filter((type) => {
          const label = t(serviceEventLabelKey(type));
          return (
            label.toLowerCase().includes(needle) ||
            type.toLowerCase().includes(needle)
          );
        });
        return types.length > 0 ? [{ ...group, types }] : [];
      }),
    [needle, t],
  );
  const allState = triState(value.size, SERVICE_EVENT_TYPES.length);

  function setMany(types: string[], checked: boolean) {
    const next = new Set(value);
    for (const type of types) {
      if (checked) next.add(type);
      else next.delete(type);
    }
    onChange(next);
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm">
          <ListFilter className="size-3.5" aria-hidden="true" />
          {value.size === SERVICE_EVENT_TYPES.length
            ? t("services.eventsFilter")
            : t("services.eventsFilterSelected", { count: value.size })}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 p-0">
        <div className="space-y-2 border-b p-3">
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("services.eventsFilterSearch")}
            aria-label={t("services.eventsFilterSearch")}
            autoComplete="off"
          />
          <label className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium hover:bg-muted/50">
            <Checkbox
              checked={allState}
              onCheckedChange={() =>
                onChange(
                  allState === true ? new Set() : new Set(SERVICE_EVENT_TYPES),
                )
              }
              aria-label={t("services.eventsFilterAll")}
            />
            {t("services.eventsFilterAll")}
          </label>
        </div>
        <ScrollArea viewportClassName="max-h-80">
          <div className="space-y-4 p-3">
            {visible.map((group) => {
              const selected = group.types.filter((type) => value.has(type));
              const state = triState(selected.length, group.types.length);
              const groupLabel = t(`services.eventsFilterGroup.${group.key}`);
              return (
                <section key={group.key} aria-label={groupLabel}>
                  <label className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm font-semibold hover:bg-muted/50">
                    <Checkbox
                      checked={state}
                      onCheckedChange={() =>
                        setMany(group.types, state !== true)
                      }
                      aria-label={groupLabel}
                    />
                    {groupLabel}
                  </label>
                  <div className="space-y-0.5 ps-6">
                    {group.types.map((type) => (
                      <label
                        key={type}
                        className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-muted/50"
                      >
                        <Checkbox
                          checked={value.has(type)}
                          onCheckedChange={(checked) =>
                            setMany([type], checked === true)
                          }
                          aria-label={t(serviceEventLabelKey(type))}
                        />
                        {t(serviceEventLabelKey(type))}
                      </label>
                    ))}
                  </div>
                </section>
              );
            })}
            {visible.length === 0 ? (
              <p className="text-muted-foreground px-2 py-3 text-sm">
                {t("services.eventsFilterNoMatches")}
              </p>
            ) : null}
          </div>
        </ScrollArea>
      </PopoverContent>
    </Popover>
  );
}
