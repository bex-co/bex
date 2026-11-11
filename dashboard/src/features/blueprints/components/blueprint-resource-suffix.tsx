import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";

/**
 * The suffix a Blueprint adds to its resource names, shown beside the
 * Blueprint's name so several Blueprints made from one file can be told apart
 * (w8/m153). Inline and smaller than the name, so it changes neither the list
 * row's nor the detail header's height. Renders nothing for a Blueprint that
 * manages the names its file gives.
 */
export function BlueprintResourceSuffix({
  suffix,
  className,
}: {
  suffix: string;
  className?: string;
}) {
  const { t } = useTranslations();
  if (!suffix) return null;
  const label = t("blueprints.resourceSuffixLabel");
  return (
    <span
      title={label}
      className={cn(
        "font-mono text-xs font-normal whitespace-nowrap text-muted-foreground",
        className,
      )}
    >
      <span className="sr-only">{label} </span>
      {`-${suffix}`}
    </span>
  );
}
