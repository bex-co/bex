import { useAccountThemeSync } from "../hooks/use-account-theme-sync";

/** The signed-in shell's mount for `useAccountThemeSync` (w3/m531). */
export function AccountThemeSync() {
  useAccountThemeSync();
  return null;
}
