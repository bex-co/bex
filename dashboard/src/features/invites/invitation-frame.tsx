import type { ReactNode } from "react";
import { Users } from "lucide-react";

/** Shared bounds for invitation loading, auth entry, review and error states. */
export function InvitationFrame({ children }: { children: ReactNode }) {
  return (
    <main className="flex min-h-svh items-center justify-center bg-background px-4 py-12 text-foreground">
      <section
        aria-label="bex"
        className="flex min-h-[420px] w-full max-w-md flex-col gap-6 rounded-xl border bg-card p-6 shadow-sm sm:p-8"
      >
        <div className="flex items-center gap-3 font-semibold">
          <Users className="size-5 text-primary" aria-hidden="true" /> bex
        </div>
        {children}
      </section>
    </main>
  );
}
