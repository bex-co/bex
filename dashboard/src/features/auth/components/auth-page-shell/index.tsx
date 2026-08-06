import { useEffect } from "react";
import type { LucideIcon } from "lucide-react";
import { LanguageSwitcher } from "@/features/i18n/language-switcher";
import { stashInviteTokenFromURL } from "@/common/lib/invite-token";

export type AuthFeature = {
  icon: LucideIcon;
  title: string;
  description: string;
  iconColor: string;
  iconBg: string;
};

export type AuthPageShellProps = {
  title: string;
  subtitle: string;
  features?: AuthFeature[];
  children: React.ReactNode;
};

/**
 * Two-column auth page layout (form + feature highlights) preserved from the
 * original scaffold's login/registration design — reused across every auth
 * page so the form content (now rendered by Ory Elements) sits in the same
 * chrome regardless of which flow it is.
 */
export function AuthPageShell({
  title,
  subtitle,
  features,
  children,
}: AuthPageShellProps) {
  // The dedicated /invite route normally stashes and scrubs the bearer before
  // arriving here. Keep accepting the old /auth/*?invite= link shape during
  // rollout; the shared helper now validates and removes it from browser
  // history before the Kratos round-trip.
  useEffect(() => {
    stashInviteTokenFromURL();
  }, []);
  return (
    <div className="min-h-screen flex bg-background relative">
      <div className="absolute top-4 right-4 z-10">
        <LanguageSwitcher />
      </div>
      <div className="flex-1 flex items-center justify-center py-12 px-6 lg:px-8">
        {/* 30rem (480px) matches the Ory Elements card's own max width, so the
            rendered <Login>/<Registration> card fills the column exactly instead
            of overflowing a narrower max-w-md — and the loading skeleton, which
            is w-full of this column, is the same width as the card it stands in
            for (no width jump when the flow swaps in). */}
        <div className="w-full max-w-[30rem] space-y-8">
          <div className="space-y-2">
            <h1 className="text-3xl font-bold text-foreground">{title}</h1>
            <p className="text-muted-foreground">{subtitle}</p>
          </div>
          {children}
        </div>
      </div>

      {features && (
        <div className="hidden lg:flex flex-1 items-center justify-center bg-muted/30 py-12 px-4 sm:px-6 lg:px-8">
          <div className="w-full max-w-md space-y-8">
            {features.map((feature) => {
              const Icon = feature.icon;
              return (
                <div key={feature.title} className="space-y-4">
                  <div
                    className={`w-16 h-16 rounded-full ${feature.iconBg} flex items-center justify-center`}
                  >
                    <Icon className={`w-8 h-8 ${feature.iconColor}`} />
                  </div>
                  <div className="space-y-2">
                    <h3 className="text-xl font-semibold text-foreground">
                      {feature.title}
                    </h3>
                    <p className="text-muted-foreground">
                      {feature.description}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
