import { useState } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useValidateBlueprint } from "@/features/blueprints/hooks/use-validate-blueprint";

interface ValidatePanelProps {
  manifest: string;
}

export function ValidatePanel({ manifest }: ValidatePanelProps) {
  const { t } = useTranslations();
  const { validate, result, loading } = useValidateBlueprint();
  const [ran, setRan] = useState(false);

  async function handleValidate() {
    setRan(true);
    await validate(manifest);
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-base">
          {t("blueprints.validateTitle")}
        </CardTitle>
        <Button
          size="sm"
          variant="outline"
          onClick={() => void handleValidate()}
          disabled={loading || !manifest}
        >
          {loading ? t("common.loading") : t("blueprints.validateRun")}
        </Button>
      </CardHeader>

      {ran && (
        <CardContent>
          {loading ? null : result == null ? (
            <p className="text-sm text-muted-foreground">
              {t("blueprints.validateNoResult")}
            </p>
          ) : result.valid ? (
            <p className="text-sm font-medium text-green-600">
              {t("blueprints.validateValid")}
            </p>
          ) : (
            <div className="space-y-1">
              <p className="text-sm font-medium text-destructive">
                {t("blueprints.validateInvalid")}
              </p>
              <ul className="list-disc pl-4 text-sm text-muted-foreground">
                {result.errors.map((e, i) => (
                  <li key={i}>{e}</li>
                ))}
              </ul>
            </div>
          )}
        </CardContent>
      )}
    </Card>
  );
}
