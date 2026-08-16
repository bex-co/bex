import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";

/**
 * A labelled text input with its hint and error copy — the block every form on
 * the dashboard hand-rolls as Label + Input + a `text-muted-foreground` hint
 * paragraph, re-typing the same class strings each time.
 *
 * `error` replaces the hint when set and marks the input invalid.
 */
export function TextField({
  id,
  label,
  value,
  onChange,
  placeholder,
  hint,
  error,
  invalid,
  autoComplete = "off",
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  hint?: string;
  error?: string;
  /** Marks the field invalid without showing error copy of its own. */
  invalid?: boolean;
  autoComplete?: string;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        autoComplete={autoComplete}
        aria-invalid={invalid || Boolean(error)}
      />
      {error ? (
        <p className="text-destructive text-sm" role="alert">
          {error}
        </p>
      ) : hint ? (
        <p className="text-muted-foreground text-sm">{hint}</p>
      ) : null}
    </div>
  );
}
