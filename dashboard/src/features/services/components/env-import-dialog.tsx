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

const PARSE_ERROR_MESSAGE: Record<
  DotenvParseError["reason"],
  | "services.envImportLineError"
  | "services.envImportKeyError"
  | "services.envImportQuoteError"
  | "services.envImportTrailingError"
> = {
  assignment: "services.envImportLineError",
  key: "services.envImportKeyError",
  quote: "services.envImportQuoteError",
  trailing: "services.envImportTrailingError",
};

export function EnvImportDialog({
  open,
  onOpenChange,
  onImport,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImport: (entries: DotenvEntry[]) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        {/* The form's state lives inside the content, which unmounts however
            the dialog closes: each opening starts empty, and a file read
            still in flight then lands nowhere (w4/m222). */}
        <EnvImportForm
          onCancel={() => onOpenChange(false)}
          onImport={(entries) => {
            onImport(entries);
            onOpenChange(false);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}

function EnvImportForm({
  onCancel,
  onImport,
}: {
  onCancel: () => void;
  onImport: (entries: DotenvEntry[]) => void;
}) {
  const { t } = useTranslations();
  const inputRef = useRef<HTMLInputElement>(null);
  const [text, setText] = useState("");
  // A parse failure keeps its reason, so the message names what is wrong on
  // that line instead of one catch-all (w4/159).
  const [errorLine, setErrorLine] = useState<DotenvParseError | "file" | null>(
    null,
  );
  // The latest file choice or text edit owns the preview. File.text cannot be
  // aborted, so a read that completes after a newer choice or edit is ignored
  // rather than overwriting it (w4/m222).
  const generation = useRef(0);
  const [reading, setReading] = useState(false);

  function supersede() {
    generation.current += 1;
    setReading(false);
  }

  function submit() {
    // The latest file choice owns what is imported: never commit the previous
    // preview while its replacement is being read, or after it failed.
    if (reading || errorLine === "file") return;
    try {
      onImport(parseDotenv(text));
    } catch (error) {
      setErrorLine(error instanceof DotenvParseError ? error : "file");
    }
  }

  async function chooseFile(file: File | undefined) {
    if (!file) return;
    supersede();
    if (file.size > MAX_DOTENV_FILE_BYTES) {
      setErrorLine("file");
      return;
    }
    const read = generation.current;
    setReading(true);
    try {
      const contents = await file.text();
      if (read !== generation.current) return;
      setText(contents);
      setErrorLine(null);
    } catch {
      if (read !== generation.current) return;
      setErrorLine("file");
    } finally {
      if (read === generation.current) setReading(false);
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("services.envImportTitle")}</DialogTitle>
        <DialogDescription>
          {t("services.envImportDescription")}
        </DialogDescription>
      </DialogHeader>
      <Textarea
        value={text}
        onChange={(event) => {
          supersede();
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
        aria-busy={reading || undefined}
        placeholder={t("services.envImportPlaceholder")}
        autoFocus
      />
      {reading ? (
        <p className="text-muted-foreground text-sm" role="status">
          {t("services.envImportReading")}
        </p>
      ) : errorLine != null ? (
        <p className="text-destructive text-sm" role="alert">
          {errorLine === "file"
            ? t("services.envImportFileError")
            : t(PARSE_ERROR_MESSAGE[errorLine.reason], {
                line: errorLine.line,
              })}
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
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("services.envCancel")}
          </Button>
          <Button
            type="button"
            disabled={!text.trim() || reading || errorLine === "file"}
            onClick={submit}
          >
            {t("services.envImportAdd")}
          </Button>
        </div>
      </DialogFooter>
    </>
  );
}
