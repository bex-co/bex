import { useId, type ComponentType, type SVGProps } from "react";
import { cn } from "@/common/lib/utils/utils";

type LogoProps = SVGProps<SVGSVGElement> & { size?: number };

/**
 * The sign-in providers' brand marks, keyed by the Kratos oidc provider id
 * (the node's `attributes.value`, `scripts/lib/kratos-oidc-fragment.sh`).
 *
 * GitHub's, Google's and GitLab's are the marks Ory Elements 1.2.1 ships
 * (`provider-logos/*.svg`, Apache-2.0), which its own buttons drew before the
 * dashboard took them over; its map is internal to `@ory/elements-react/theme`
 * and has no Bitbucket. GitHub's follows the text colour (`currentColor`), as
 * the monochrome mark is meant to, so it stays visible on the dark theme.
 * Bitbucket's is Atlassian's mark.
 */
function GithubLogo({ size = 20, ...props }: LogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="currentColor"
      aria-hidden="true"
      {...props}
    >
      <path d="M12 0C5.374 0 0 5.373 0 12c0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.5 11.5 0 0 1 12 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576C20.566 21.797 24 17.3 24 12c0-6.627-5.373-12-12-12" />
    </svg>
  );
}

function GoogleLogo({ size = 20, ...props }: LogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      width={size}
      height={size}
      fill="none"
      aria-hidden="true"
      {...props}
    >
      <path
        fill="#4285F4"
        d="M30.001 16.31c0-1.15-.095-1.99-.301-2.861H16.287v5.195h7.873c-.159 1.291-1.016 3.236-2.92 4.542l-.027.174 4.24 3.22.294.029c2.699-2.443 4.254-6.036 4.254-10.298"
      />
      <path
        fill="#34A853"
        d="M16.286 30c3.857 0 7.095-1.244 9.46-3.391l-4.507-3.423c-1.207.825-2.826 1.4-4.953 1.4A8.58 8.58 0 0 1 8.16 18.77l-.167.014-4.41 3.344-.058.157C5.874 26.858 10.7 30 16.286 30"
      />
      <path
        fill="#FBBC05"
        d="M8.16 18.769a8.5 8.5 0 0 1-.476-2.77c0-.964.174-1.897.46-2.768l-.008-.185-4.465-3.399-.146.068A13.8 13.8 0 0 0 2.001 16c0 2.256.556 4.387 1.524 6.284z"
      />
      <path
        fill="#EB4335"
        d="M16.286 7.413c2.683 0 4.492 1.136 5.524 2.085l4.032-3.858C23.366 3.384 20.143 2 16.286 2 10.7 2 5.874 5.142 3.524 9.715l4.62 3.516c1.158-3.375 4.365-5.818 8.142-5.818"
      />
    </svg>
  );
}

function GitlabLogo({ size = 20, ...props }: LogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      aria-hidden="true"
      {...props}
    >
      <path
        fill="#E24329"
        d="m22.708 10.691-.031-.072-3.015-7.167a.74.74 0 0 0-.31-.34.87.87 0 0 0-.923.045.73.73 0 0 0-.268.37L16.125 9.2H7.881L5.845 3.527a.72.72 0 0 0-.268-.371.87.87 0 0 0-.923-.045.74.74 0 0 0-.31.34l-3.021 7.164-.03.072a4.67 4.67 0 0 0-.153 3.23c.335 1.063 1.04 1.998 2.01 2.664l.01.007.028.018 4.594 3.132 2.272 1.567 1.384.952c.162.112.36.172.563.172s.401-.06.563-.172l1.384-.952 2.273-1.567 4.62-3.151.012-.009c.968-.666 1.671-1.6 2.006-2.661a4.67 4.67 0 0 0-.15-3.226"
      />
      <path
        fill="#FC6D26"
        d="m22.708 10.691-.031-.072a10.7 10.7 0 0 0-4.055 1.66L12 16.839l4.218 2.904 4.621-3.152.012-.008c.969-.666 1.674-1.601 2.008-2.664a4.67 4.67 0 0 0-.15-3.228"
      />
      <path
        fill="#FCA326"
        d="m7.781 19.743 2.273 1.566 1.384.952c.162.112.36.172.563.172s.401-.06.563-.172l1.384-.952 2.273-1.566S14.255 18.389 12 16.839c-2.255 1.55-4.219 2.904-4.219 2.904"
      />
      <path
        fill="#FC6D26"
        d="M5.376 12.279a10.7 10.7 0 0 0-4.053-1.664l-.03.072a4.67 4.67 0 0 0-.153 3.23c.335 1.063 1.04 1.998 2.01 2.664l.01.007.028.018 4.594 3.132L12 16.836z"
      />
    </svg>
  );
}

function BitbucketLogo({ size = 20, ...props }: LogoProps) {
  // Ids are document-global: each instance names its own gradient.
  const gradient = useId();
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      width={size}
      height={size}
      fill="none"
      aria-hidden="true"
      {...props}
    >
      <path
        fill="#2684FF"
        d="M2.278 2.133a.97.97 0 0 0-.97 1.124l4.12 25.008a1.32 1.32 0 0 0 1.29 1.1h19.763a.97.97 0 0 0 .97-.815l4.12-25.288a.97.97 0 0 0-.97-1.124zm17.346 18.074h-6.308l-1.708-8.924h9.544z"
      />
      <path
        fill={`url(#${gradient})`}
        d="M29.81 11.283h-9.11l-1.528 8.924h-6.308l-7.448 8.84a1.31 1.31 0 0 0 .85.32h19.767a.97.97 0 0 0 .97-.814z"
      />
      <defs>
        <linearGradient
          id={gradient}
          x1="31.919"
          x2="16.968"
          y1="13.783"
          y2="25.452"
          gradientUnits="userSpaceOnUse"
        >
          <stop offset=".18" stopColor="#0052CC" />
          <stop offset="1" stopColor="#2684FF" />
        </linearGradient>
      </defs>
    </svg>
  );
}

const SSO_PROVIDER_LOGOS: Readonly<Record<string, ComponentType<LogoProps>>> = {
  github: GithubLogo,
  google: GoogleLogo,
  gitlab: GitlabLogo,
  bitbucket: BitbucketLogo,
};

/**
 * A sign-in provider's logo by its Kratos provider id. Like Ory's buttons it
 * reads the id up to its first `-` (`google-workspace` draws Google's), and a
 * provider with no mark gets Ory's generic tile with the label's first letter.
 */
export function SsoProviderLogo({
  provider,
  label,
  size,
}: {
  provider: string;
  label: string;
  size: number;
}) {
  const Logo = SSO_PROVIDER_LOGOS[provider.split("-")[0] ?? ""];
  if (Logo) return <Logo size={size} data-sso-logo={provider} />;
  return (
    <span
      aria-hidden="true"
      data-sso-logo="generic"
      style={{ width: size, height: size }}
      className={cn(
        "flex shrink-0 items-center justify-center rounded-buttons text-xs",
        "border-button-social-border-generic-provider bg-button-social-background-generic-provider text-button-social-foreground-generic-provider",
      )}
    >
      {label.slice(0, 1)}
    </span>
  );
}
