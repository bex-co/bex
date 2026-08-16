import { Check, Copy } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";

export interface CopyButtonProps {
  value: string;
  label: string;
  successText: string;
  errorText: string;
}

/**
 * A compact copy-to-clipboard icon button with a 2s "copied" tick. Shared by
 * any feature that needs to copy a value to the clipboard (connection
 * strings, API-key secrets, …); callers supply their own i18n'd toast text
 * rather than this component owning a translation namespace.
 */
export function CopyButton({
  value,
  label,
  successText,
  errorText,
}: CopyButtonProps) {
  const { copied, copy } = useCopyToClipboard({ successText, errorText });

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      onClick={() => void copy(value)}
      aria-label={label}
    >
      {copied ? <Check className="text-green-600" /> : <Copy />}
    </Button>
  );
}
