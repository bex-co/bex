import { SetPortDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UsePortResult {
  setPort: (id: string, port: number) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings port control to bex-api's `setPort` (w4/m121/t001) — the
 * container port bex routes to and injects as `$PORT`. Only valid for
 * web_service and private_service; the settings page hides the section for
 * cron_job / background_worker / static_site, which the backend refuses with a
 * 400 of its own.
 *
 * Saving bumps `restartedAt`, so it opens a deploy and rolls the pods; the
 * control's hint says so before the user commits.
 */
export function usePort(): UsePortResult {
  const { run, busy } = useFieldMutation(
    SetPortDocument,
    (id: string, port: number) => ({ id, port }),
    { success: "services.portSuccess", error: "services.portError" },
  );

  return { setPort: run, busy };
}
