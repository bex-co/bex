import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DISK_MAX_SIZE_GB,
  DISK_MIN_SIZE_GB,
  DISK_SIZE_CHIPS,
} from "@/features/services/lib/disk";

/** The five things Render tells you before you attach a disk, verbatim in meaning. */
export function DiskWarnings() {
  const { t } = useTranslations();
  return (
    <div className="rounded-md border border-amber-500/40 bg-amber-500/5 p-4 text-sm">
      <p className="font-medium">{t("services.diskWarningsTitle")}</p>
      <ul className="mt-2 list-disc space-y-1 pl-5 text-muted-foreground">
        <li>{t("services.diskWarningZeroDowntime")}</li>
        <li>{t("services.diskWarningSingleInstance")}</li>
        <li>{t("services.diskWarningOnePerService")}</li>
        <li>{t("services.diskWarningMountPathOnly")}</li>
        <li>{t("services.diskWarningNoSharing")}</li>
      </ul>
    </div>
  );
}

/**
 * Render's size picker: the quick-select chips and a free-text box, which
 * honors the same floor as the chips; the API refuses below it anyway, and
 * saying so up front beats a failed submit.
 */
export function DiskSizeControl({
  id,
  value,
  onChange,
  invalid = false,
  describedBy,
}: {
  id: string;
  value: number;
  onChange: (sizeGB: number) => void;
  invalid?: boolean;
  describedBy?: string;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-wrap items-center gap-2">
      {DISK_SIZE_CHIPS.map((size) => (
        <Button
          key={size}
          type="button"
          variant={value === size ? "default" : "outline"}
          size="sm"
          onClick={() => onChange(size)}
        >
          {t("services.diskSizeChip", { size: String(size) })}
        </Button>
      ))}
      <Input
        id={id}
        type="number"
        min={DISK_MIN_SIZE_GB}
        max={DISK_MAX_SIZE_GB}
        className="w-28"
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        aria-invalid={invalid}
        aria-describedby={describedBy}
      />
    </div>
  );
}
