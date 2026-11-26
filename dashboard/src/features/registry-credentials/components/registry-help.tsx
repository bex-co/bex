import { Fragment, type ReactNode } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import type { RenderRegistry } from "@/features/registry-credentials/lib/registries";

const MARK = "\u0000";

/**
 * A translated message with React nodes (a link, inline code) in its
 * `{placeholders}`, so each language orders the sentence around them.
 */
export function MessageWithNodes({
  id,
  nodes,
}: {
  id: keyof typeof en;
  nodes: Record<string, ReactNode>;
}) {
  const { t } = useTranslations();
  const marks = Object.fromEntries(
    Object.keys(nodes).map((name) => [name, `${MARK}${name}${MARK}`]),
  );
  // Every odd part of the split is a placeholder's name.
  return t(id, marks)
    .split(MARK)
    .map((part, i) =>
      i % 2 === 1 ? <Fragment key={i}>{nodes[part]}</Fragment> : part,
    );
}

function Code({ children }: { children: string }) {
  return (
    <code className="bg-muted rounded px-1 font-mono text-xs">{children}</code>
  );
}

function ExternalLink({ href, children }: { href: string; children: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="text-primary hover:underline"
    >
      {children}
    </a>
  );
}

/**
 * Render's help under the Registry select: where to create the token or
 * account and the permission it needs, with the token page prefilled with the
 * credential's name where the registry supports it
 * (docs/render-artifacts/registry-credential-dialog.md, w4/m670). The menu
 * paths name the registries' own English pages, so they stay in English.
 */
export function RegistryHelp({
  registry,
  name,
}: {
  registry: RenderRegistry;
  name: string;
}) {
  const { t } = useTranslations();
  let id: keyof typeof en;
  let nodes: Record<string, ReactNode>;
  switch (registry) {
    case "GITHUB":
      id = "registryCredentials.helpGithub";
      nodes = {
        permission: <Code>read:packages</Code>,
        link: (
          <ExternalLink
            href={`https://github.com/settings/tokens/new?${new URLSearchParams({ description: name, scopes: "read:packages" })}`}
          >
            GitHub
          </ExternalLink>
        ),
        path: (
          <span className="font-medium">Settings &gt; Developer Settings</span>
        ),
      };
      break;
    case "GITLAB":
      id = "registryCredentials.helpGitlab";
      nodes = {
        permission: <Code>read registry</Code>,
        link: (
          <ExternalLink
            href={`https://gitlab.com/-/profile/personal_access_tokens?${new URLSearchParams({ name, scopes: "read_registry" })}`}
          >
            GitLab
          </ExternalLink>
        ),
        path: (
          <span className="font-medium">Edit Profile &gt; Access Tokens</span>
        ),
      };
      break;
    case "DOCKER":
      id = "registryCredentials.helpDocker";
      nodes = {
        permission: <Code>read</Code>,
        link: (
          <ExternalLink href="https://hub.docker.com/settings/security?generateToken=true">
            Docker Hub
          </ExternalLink>
        ),
        path: (
          <span className="font-medium">Account settings &gt; Security</span>
        ),
      };
      break;
    case "GOOGLE_ARTIFACT":
      id = "registryCredentials.helpGoogle";
      nodes = {
        permission: <Code>roles/artifactregistry.reader</Code>,
        link: (
          <ExternalLink href="https://console.cloud.google.com/projectselector2/iam-admin/serviceaccounts/create">
            Google Cloud
          </ExternalLink>
        ),
      };
      break;
    case "AWS_ECR":
      id = "registryCredentials.helpEcr";
      nodes = {
        command: <Code>aws ecr get-login-password</Code>,
        link: (
          <ExternalLink href="https://docs.aws.amazon.com/AmazonECR/latest/userguide/registry_auth.html#registry-auth-credential-helper">
            {t("registryCredentials.helpEcrLink")}
          </ExternalLink>
        ),
      };
      break;
  }
  return (
    <p className="text-muted-foreground text-sm">
      <MessageWithNodes id={id} nodes={nodes} />
    </p>
  );
}
