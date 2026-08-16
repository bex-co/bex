import { Globe, Lock, Cpu, Clock, Layers } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import type { ServiceType } from "@/features/services/lib/create-context";

const SERVICE_TYPE_DEFS: {
  type: ServiceType;
  icon: React.ReactNode;
  labelKey: string;
  descKey: string;
}[] = [
  {
    type: "web_service",
    icon: <Globe className="size-4" />,
    labelKey: "services.typeWeb",
    descKey: "services.createTypeWebDesc",
  },
  {
    type: "private_service",
    icon: <Lock className="size-4" />,
    labelKey: "services.typePrivate",
    descKey: "services.createTypePrivateDesc",
  },
  {
    type: "background_worker",
    icon: <Cpu className="size-4" />,
    labelKey: "services.typeWorker",
    descKey: "services.createTypeWorkerDesc",
  },
  {
    type: "cron_job",
    icon: <Clock className="size-4" />,
    labelKey: "services.typeCron",
    descKey: "services.createTypeCronDesc",
  },
  {
    type: "static_site",
    icon: <Layers className="size-4" />,
    labelKey: "services.typeStatic",
    descKey: "services.createTypeStaticDesc",
  },
];

export function ServiceTypePicker({
  value,
  onChange,
}: {
  value: ServiceType;
  onChange: (type: ServiceType) => void;
}) {
  const { t } = useTranslations();
  return (
    <div role="radiogroup" className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {SERVICE_TYPE_DEFS.map(({ type, icon, labelKey, descKey }) => {
        const selected = type === value;
        return (
          <button
            key={type}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(type)}
            className={cn(
              "flex items-start gap-3 rounded-lg border p-3 text-left transition-colors",
              selected
                ? "border-primary ring-1 ring-primary"
                : "border-border hover:border-muted-foreground/50",
            )}
          >
            <span className="mt-0.5 shrink-0 text-muted-foreground">
              {icon}
            </span>
            <div>
              <div className="text-sm font-medium">{t(labelKey)}</div>
              <div className="text-xs text-muted-foreground">{t(descKey)}</div>
            </div>
          </button>
        );
      })}
    </div>
  );
}
