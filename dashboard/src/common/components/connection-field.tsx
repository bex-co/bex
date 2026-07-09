import { CopyButton } from "@/common/components/copy-button";

export interface ConnectionFieldProps {
  label: string;
  value: string;
  copiedText: string;
  copyErrorText: string;
}

/**
 * A labeled, monospace, horizontally-scrollable connection string + copy
 * button — the row shape shared by every datastore's connection-info panel
 * (databases, Key Value, …). Doesn't own a translation namespace (like
 * `CopyButton`): callers supply their own feature's copy/error text.
 */
export function ConnectionField({
  label,
  value,
  copiedText,
  copyErrorText,
}: ConnectionFieldProps) {
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium">{label}</span>
        <CopyButton
          value={value}
          label={label}
          successText={copiedText}
          errorText={copyErrorText}
        />
      </div>
      <div className="overflow-x-auto rounded-md border bg-muted px-3 py-2">
        <code className="font-mono text-xs whitespace-pre">{value || "—"}</code>
      </div>
    </div>
  );
}
