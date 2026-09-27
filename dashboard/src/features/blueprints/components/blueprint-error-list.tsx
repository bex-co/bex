import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import type { BlueprintValidationError } from "../types";

/**
 * A Blueprint's validation errors, each with where it is: "line 6, column 21"
 * and the field path the API reports (w8/019). The dashboard used to print
 * only the flat message, so "Blueprint is not valid YAML" arrived with no
 * location (w4/151). Shared by the Validate panel, the New Blueprint review,
 * and the Sync dialog. Falls back to the plain messages when the API returned
 * no details (an older bex-api, or a fetch-level error string).
 */
export function BlueprintErrorList({
  errors,
  details,
  className,
}: {
  errors: string[];
  details: BlueprintValidationError[];
  className?: string;
}) {
  const { t } = useTranslations();
  return (
    <ul className={cn("list-disc space-y-1 pl-4", className)}>
      {details.length > 0
        ? details.map((d, i) => {
            const where =
              d.line > 0
                ? d.column > 0
                  ? t("blueprints.errorAtLineColumn", {
                      line: d.line,
                      column: d.column,
                    })
                  : t("blueprints.errorAtLine", { line: d.line })
                : "";
            return (
              <li key={i}>
                {where || d.path ? (
                  <span className="mr-1.5 font-mono text-xs">
                    {[where, d.path].filter(Boolean).join(" · ")}
                  </span>
                ) : null}
                {d.error}
              </li>
            );
          })
        : errors.map((e, i) => <li key={i}>{e}</li>)}
    </ul>
  );
}
