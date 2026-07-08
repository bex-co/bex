import { Check, Copy } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";

/**
 * A compact copy-to-clipboard icon button with a 2s "copied" tick, i18n'd
 * (unlike common/CodeBlock, whose copy toast is a hardcoded string). Shares the
 * copy mechanism with CodeBlock via useCopyToClipboard.
 */
export function CopyButton({ value, label }: { value: string; label: string }) {
  const { t } = useTranslations();
  const { copied, copy } = useCopyToClipboard({
    successText: t("databases.copied"),
    errorText: t("databases.copyError"),
  });

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
