export type SupportedLanguage = "en" | "zh";

export function resolveSupportedLanguage(
  languageCode: string | null | undefined,
): SupportedLanguage {
  return languageCode?.toLocaleLowerCase() === "zh" ? "zh" : "en";
}
