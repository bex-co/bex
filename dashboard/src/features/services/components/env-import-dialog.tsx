import { useRef, useState } from "react";
import { FileUp } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DotenvParseError,
  MAX_DOTENV_FILE_BYTES,
  parseDotenv,
  type DotenvEntry,
} from "@/features/services/lib/dotenv-import";

export function EnvImportDialog({
  open,
  onOpenChange,
  onImport,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImport: (entries: DotenvEntry[]) => void;
}) {
  const { t } = useTranslations();
  const inputRef = useRef<HTMLInputElement>(null);
  const [text, setText] = useState("");
  const [errorLine, setErrorLine] = useState<number | "file" | null>(null);

  function close() {
    setText("");
    setErrorLine(null);
    onOpenChange(false);
  }

  function submit() {
    try {
      const entries = parseDotenv(text);
      onImport(entries);
      close();
    } catch (error) {
      setErrorLine(error instanceof DotenvParseError ? error.line : "file");
    }
  }

  async function chooseFile(file: File | undefined) {
    if (!file) return;
    if (file.size > MAX_DOTENV_FILE_BYTES) {
      setErrorLine("file");
      return;
    }
    try {
      setText(await file.text());
      setErrorLine(null);
    } catch {
      setErrorLine("file");
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => (next ? onOpenChange(true) : close())}
    >
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("services.envImportTitle")}</DialogTitle>
          <DialogDescription>
            {t("services.envImportDescription")}
          </DialogDescription>
        </DialogHeader>
        <Textarea
          value={text}
          onChange={(event) => {
            setText(event.target.value);
            setErrorLine(null);
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
              event.preventDefault();
              submit();
            }
          }}
          className="min-h-56 font-mono text-sm"
          aria-label={t("services.envImportTextLabel")}
          aria-invalid={errorLine != null}
          placeholder={t("services.envImportPlaceholder")}
          autoFocus
        />
        {errorLine != null ? (
          <p className="text-destructive text-sm" role="alert">
            {errorLine === "file"
              ? t("services.envImportFileError")
              : t("services.envImportLineError", { line: errorLine })}
          </p>
        ) : null}
        <input
          ref={inputRef}
          className="sr-only"
          type="file"
          accept=".env,text/plain"
          onChange={(event) => {
            void chooseFile(event.target.files?.[0]);
            event.target.value = "";
          }}
        />
        <DialogFooter className="flex-col-reverse gap-2 sm:flex-row sm:justify-between">
          <Button
            type="button"
            variant="outline"
            onClick={() => inputRef.current?.click()}
          >
            <FileUp /> {t("services.envImportChooseFile")}
          </Button>
          <div className="flex flex-col-reverse gap-2 sm:flex-row">
            <Button type="button" variant="outline" onClick={close}>
              {t("services.envCancel")}
            </Button>
            <Button type="button" disabled={!text.trim()} onClick={submit}>
              {t("services.envImportAdd")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
