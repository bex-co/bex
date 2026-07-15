import { useState, type FormEvent } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRenameDatabase } from "@/features/databases/hooks/use-rename-database";
import type { DatabaseDetailView } from "@/features/databases/types";

const NAME_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,28}[a-z0-9])?$/;

export interface DatabaseNameSectionProps {
  database: DatabaseDetailView;
  onChanged: () => void;
}

export function DatabaseNameSection({
  database,
  onChanged,
}: DatabaseNameSectionProps) {
  const { t } = useTranslations();
  const { rename, busy } = useRenameDatabase();
  const [name, setName] = useState(database.name);
  const valid = NAME_PATTERN.test(name);
  const changed = name !== database.name;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!valid || !changed || busy) return;
    if (await rename(database.id, name)) onChanged();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("databases.nameTitle")}</CardTitle>
        <CardDescription>{t("databases.nameDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="space-y-3"
          onSubmit={(event) => void handleSubmit(event)}
        >
          <Label htmlFor="database-display-name">
            {t("databases.fieldName")}
          </Label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              id="database-display-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              autoComplete="off"
              aria-invalid={changed && !valid}
              aria-describedby="database-display-name-help"
            />
            <Button type="submit" disabled={!valid || !changed || busy}>
              {t("databases.nameSave")}
            </Button>
          </div>
          <p
            id="database-display-name-help"
            className={
              changed && !valid
                ? "text-sm text-destructive"
                : "text-sm text-muted-foreground"
            }
          >
            {changed && !valid
              ? t("databases.nameInvalid")
              : t("databases.nameDescription")}
          </p>
        </form>
      </CardContent>
    </Card>
  );
}
