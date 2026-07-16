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
import { useRenameKeyValue } from "@/features/keyvalue/hooks/use-rename-key-value";
import type { KeyValueView } from "@/features/keyvalue/types";

const NAME_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,28}[a-z0-9])?$/;

export interface KeyValueNameSectionProps {
  keyValue: KeyValueView;
  onChanged: () => void;
}

export function KeyValueNameSection({
  keyValue,
  onChanged,
}: KeyValueNameSectionProps) {
  const { t } = useTranslations();
  const { rename, busy } = useRenameKeyValue();
  const [name, setName] = useState(keyValue.name);
  const valid = NAME_PATTERN.test(name);
  const changed = name !== keyValue.name;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!valid || !changed || busy) return;
    if (await rename(keyValue.id, name)) onChanged();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.nameTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.nameDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="space-y-3"
          onSubmit={(event) => void handleSubmit(event)}
        >
          <Label htmlFor="key-value-display-name">
            {t("keyvalue.fieldName")}
          </Label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              id="key-value-display-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              autoComplete="off"
              aria-invalid={changed && !valid}
              aria-describedby="key-value-display-name-help"
            />
            <Button type="submit" disabled={!valid || !changed || busy}>
              {t("keyvalue.nameSave")}
            </Button>
          </div>
          <p
            id="key-value-display-name-help"
            className={
              changed && !valid
                ? "text-sm text-destructive"
                : "text-sm text-muted-foreground"
            }
          >
            {changed && !valid
              ? t("keyvalue.nameInvalid")
              : t("keyvalue.nameDescription")}
          </p>
        </form>
      </CardContent>
    </Card>
  );
}
