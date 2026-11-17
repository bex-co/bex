import type { Translate } from "@/common/hooks/use-translations";

/** The SSH keys table's columns, shared by the panel and its frame. */
export function sshKeysColumns(t: Translate): string[] {
  return [t("sshKeys.name"), t("sshKeys.fingerprint"), t("sshKeys.created")];
}
