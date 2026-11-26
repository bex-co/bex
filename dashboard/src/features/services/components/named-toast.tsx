import { aroundPlaceholder } from "@/common/lib/i18n-placeholder";

/**
 * A toast's sentence with the service's name in bold where each language puts
 * it, as Render's resume toasts read ("**web** has been resumed.", w3/m643).
 * `text` is the sentence translated with PLACEHOLDER_SENTINEL as its `{name}`.
 */
export function NamedToast({ text, name }: { text: string; name: string }) {
  const [before, after] = aroundPlaceholder(text);
  return (
    <>
      {before}
      <strong className="font-semibold">{name}</strong>
      {after}
    </>
  );
}
