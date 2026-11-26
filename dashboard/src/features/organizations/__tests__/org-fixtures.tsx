import type { ReactNode } from "react";
import { render } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { GraphQLError } from "graphql";
import { Observable } from "rxjs";
import { vi } from "vitest";
import { OrganizationDocument } from "@/graphql/definitions";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import type {
  Organization,
  OrganizationRole,
  OrganizationWorkspace,
} from "@/features/organizations/types";

export function orgWorkspace(
  overrides: Partial<OrganizationWorkspace> = {},
): OrganizationWorkspace {
  return {
    __typename: "OrganizationWorkspace",
    id: "tea-hq",
    name: "acme-hq",
    slug: "acme-hq",
    plan: "scale",
    privacy: "invite_only",
    primary: false,
    viewerIsMember: true,
    avatarDownload: null,
    ...overrides,
  };
}

/** Acme as each role sees it: the primary, a Public and an Invite-only one. */
export function organization(
  role: OrganizationRole,
  workspaces?: OrganizationWorkspace[],
): Organization {
  return {
    __typename: "Organization",
    id: "org-1",
    name: "Acme",
    plan: "scale",
    primaryWorkspaceId: "tea-hq",
    viewerRole: role,
    createdAt: "2026-11-01T00:00:00Z",
    requireTwoFactor: false,
    requireGoogleLogin: false,
    workspaces: workspaces ?? [
      orgWorkspace({ primary: true, viewerIsMember: role === "OWNER" }),
      orgWorkspace({
        id: "tea-web",
        name: "acme-web",
        slug: "acme-web",
        privacy: "public",
        viewerIsMember: false,
      }),
      orgWorkspace({
        id: "tea-ml",
        name: "acme-ml",
        slug: "acme-ml",
        privacy: "invite_only",
        viewerIsMember: false,
      }),
    ],
  };
}

/**
 * The org pages inside a memory router at `/org/org-1/<section>`, so their
 * navigation links and the skeletons' `useParams` resolve as in the app.
 */
export function renderAtOrgRoute(
  section: "workspaces" | "members" | "settings",
  element: ReactNode,
) {
  const rootRoute = createRootRoute();
  const page = createRoute({
    getParentRoute: () => rootRoute,
    path: `/org/$orgId/${section}`,
    component: () => element,
  });
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <p>overview</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([page, home]),
    history: createMemoryHistory({
      initialEntries: [`/org/org-1/${section}`],
    }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

/**
 * Where the client already knows the caller's org from (w4/706): the
 * caller's `organizations` list (the shell's fetch), or only the org's own
 * `organization(id)` entry, which the previous org tab read.
 */
export const KNOWN_FROM = ["org list", "org entry"] as const;

/**
 * An org route's pending frame, with `org` cached only where `knownFrom`
 * says: a real client whose cache holds the org entry or nothing, and the
 * (globally mocked) list holding the org or nothing.
 */
export function renderPendingAtOrgRoute(
  section: "workspaces" | "members" | "settings",
  element: ReactNode,
  org: Organization,
  knownFrom: (typeof KNOWN_FROM)[number],
) {
  const client = new ApolloClient({
    link: ApolloLink.empty(),
    cache: new InMemoryCache(),
  });
  if (knownFrom === "org entry") {
    client.writeQuery({
      query: OrganizationDocument,
      variables: { id: org.id },
      data: { organization: org },
    });
  }
  vi.mocked(useOrganizations).mockReturnValue({
    organizations: knownFrom === "org list" ? [org] : [],
  });
  return renderAtOrgRoute(
    section,
    <ApolloProvider client={client}>{element}</ApolloProvider>,
  );
}

/** A button's visible label: its text without the screen-reader suffix. */
function visibleLabel(element: Element): string {
  const copy = element.cloneNode(true) as Element;
  copy.querySelectorAll(".sr-only").forEach((node) => node.remove());
  return copy.textContent?.trim() ?? "";
}

/**
 * Ready buttons, or the reserved boxes a skeleton draws in their place. A
 * login requirement's switch is read with its row (`loginRequirements`).
 */
function actionLabels(root: Element | null | undefined): string[] {
  if (!root) return [];
  const reserved = [
    // ReservedButton's box (a ReservedBadge's carries the Badge's data-slot).
    ...root.querySelectorAll(
      "span.relative.inline-flex.shrink-0 > span.invisible:not([data-slot])",
    ),
  ];
  const buttons = [...root.querySelectorAll("button:not([role=switch])")];
  return [...reserved, ...buttons].map(visibleLabel);
}

/**
 * A login requirement row's control, as the box it takes: "switch" for the
 * switch or (pending) the switch's box; the badge's variant and text for the
 * state or (pending, w7/089) the badge's box, which a skeleton lays out with
 * the ready badge's own classes and text, invisibly.
 */
function loginRequirementControl(section: Element): string | null {
  const badge = section.querySelector("[data-slot=badge]");
  if (badge) {
    const classes = badge.className
      .split(/\s+/)
      .filter((name) => name && name !== "invisible")
      .join(" ");
    return `badge "${badge.textContent}" ${classes}`;
  }
  return section.querySelector(
    '[role=switch], [data-skeleton-region$="-control"]',
  )
    ? "switch"
    : null;
}

/**
 * The geometry-bearing structure of an org page, read the same way from the
 * ready page and from its pending frame: the frame's width, the header's
 * copy and navigation, each card's title, description and action, the table
 * columns, and each row's height class, badges and actions.
 */
export function pageStructure(root: HTMLElement) {
  const texts = (selector: string) =>
    [...root.querySelectorAll(selector)].map((node) => visibleLabel(node));
  return {
    frame: root.querySelector("[data-page-scroll] > div")?.className,
    headings: root.querySelectorAll("h1").length,
    description: root.querySelector("[data-page-header] .max-w-prose")
      ?.textContent,
    navigation: [...root.querySelectorAll("nav a")].map((link) => [
      link.textContent,
      link.getAttribute("aria-current"),
    ]),
    cards: root.querySelectorAll("[data-slot=card]").length,
    cardTitles: texts("[data-slot=card-title]"),
    cardDescriptions: texts("[data-slot=card-description]"),
    cardActions: [...root.querySelectorAll("[data-slot=card-action]")].map(
      (action) => actionLabels(action),
    ),
    columns: texts("[data-slot=table-head]"),
    rows: [
      ...root.querySelectorAll(
        "[data-slot=table-body] > [data-slot=table-row]",
      ),
    ].map((row) => ({
      height: row.className.match(/\bh-\d+\b/)?.[0] ?? null,
      badges: row.querySelectorAll("[data-slot=badge]").length,
      actions: actionLabels(row.lastElementChild),
    })),
    labels: texts("label"),
    /** Each Security setting (w7/m466): its label, its control's box (the
     *  switch's, or the state badge's; w7/089), and what it requires. */
    loginRequirements: [
      ...root.querySelectorAll("[data-login-requirement]"),
    ].map((section) => ({
      name: section.getAttribute("data-login-requirement"),
      label: section.querySelector("label")?.textContent,
      control: loginRequirementControl(section),
      description: section.querySelector(":scope > p")?.textContent,
    })),
    /** The cards' static notes (who may change a setting). */
    notes: texts("[data-slot=card-content] > p.text-xs"),
    /** Every button on the page, in order (a skeleton's reserved boxes). */
    buttons: actionLabels(root.querySelector("[data-page-scroll]")),
  };
}

type Answer = Record<string, unknown> | GraphQLError;

/**
 * A real Apollo client whose network answers each operation by name and
 * records what was sent, so a test reads a mutation's variables as bex-api
 * would receive them. `before` links run ahead of the network, as the app's
 * own (the auth error link) do.
 */
export function scriptedClient(
  answers: Record<string, (variables: Record<string, unknown>) => Answer>,
  before: ApolloLink[] = [],
) {
  const sent: { name: string; variables: Record<string, unknown> }[] = [];
  const link = new ApolloLink(
    (operation) =>
      new Observable((observer) => {
        const name = operation.operationName ?? "";
        sent.push({ name, variables: operation.variables });
        const answer = answers[name]?.(operation.variables);
        if (answer === undefined) {
          observer.error(new Error(`no answer scripted for ${name}`));
          return;
        }
        observer.next(
          answer instanceof GraphQLError
            ? { errors: [answer] }
            : { data: answer },
        );
        observer.complete();
      }),
  );
  const client = new ApolloClient({
    link: ApolloLink.from([...before, link]),
    cache: new InMemoryCache(),
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return { client, wrapper, sent };
}

/**
 * A link that holds every operation until `release()`, so a test reads the
 * frame a component draws while its read is in flight, then the ready state.
 */
export function heldLink() {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => (release = resolve));
  const link = new ApolloLink(
    (operation, forward) =>
      new Observable((observer) => {
        let subscription: { unsubscribe(): void } | undefined;
        void gate.then(() => {
          subscription = forward(operation).subscribe(observer);
        });
        return () => subscription?.unsubscribe();
      }),
  );
  return { link, release };
}

/** bex-api's coded refusal, as it arrives in a GraphQL response. */
export function refusal(code: string, params: Record<string, unknown> = {}) {
  return new GraphQLError("reworded by the server", {
    extensions: { ...params, code },
  });
}
