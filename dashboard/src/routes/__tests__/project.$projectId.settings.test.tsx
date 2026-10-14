import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ProjectSettingsPage } from "../project.$projectId.settings";
import type { ProjectView } from "@/features/projects/hooks/use-projects";
import { ProjectSettingsPageSkeleton } from "@/common/components/route-skeletons";
import i18n from "@/i18n/init";

const projectsState: {
  projects: ProjectView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = {
  projects: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => undefined),
};
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => projectsState,
}));

const rename = vi.fn();
vi.mock("@/features/projects/hooks/use-rename-project", () => ({
  useRenameProject: () => ({ rename, busy: false }),
}));

const remove = vi.fn();
vi.mock("@/features/projects/hooks/use-delete-project", () => ({
  useDeleteProject: () => ({ remove, deleting: null }),
}));

function renderSettingsPage(projectId = "prj-1") {
  const rootRoute = createRootRoute();
  const projectRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId",
    loader: ({ params }) => {
      const project = projectsState.projects.find(
        (candidate) => candidate.id === params.projectId,
      );
      return project
        ? ({ state: "ready", resource: project } as const)
        : ({ state: "not-found" } as const);
    },
  });
  const settingsRoute = createRoute({
    getParentRoute: () => projectRoute,
    path: "settings",
    component: ProjectSettingsPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      projectRoute.addChildren([settingsRoute]),
    ]),
    history: createMemoryHistory({
      initialEntries: [`/project/${projectId}/settings`],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  projectsState.projects = [];
  projectsState.loading = false;
  projectsState.error = undefined;
  rename.mockReset();
  remove.mockReset();
});

describe("ProjectSettingsPage", () => {
  it("renders the current project name", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    ];

    renderSettingsPage();

    expect(
      await screen.findByRole("heading", { name: "Project settings" }),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("storefront")).toBeInTheDocument();
  });

  it("renders nothing for an unknown project id — the layout's redirect home is in flight (w9/m55)", async () => {
    renderSettingsPage("prj-missing");

    await waitFor(() =>
      expect(screen.queryByText("Project Name")).not.toBeInTheDocument(),
    );
    expect(screen.queryByText("Project not found.")).not.toBeInTheDocument();
  });

  it("renames the project through the inline Edit control", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    ];
    rename.mockResolvedValue(true);
    const user = userEvent.setup();

    renderSettingsPage();

    // The read-only state is named too, by the card heading it is now
    // associated with — a control identified by its purpose in BOTH states
    // rather than by whatever value it holds (w4/134).
    expect(
      await screen.findByRole("textbox", { name: "Project Name" }),
    ).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Edit" }));

    const input = screen.getByRole("textbox", { name: "Project Name" });
    expect(input).toBeEnabled();
    await user.clear(input);
    await user.type(input, "new-name");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(rename).toHaveBeenCalledWith("prj-1", "new-name");
  });

  it("deletes the project through the danger-zone confirm dialog", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    ];
    remove.mockResolvedValue(true);
    const user = userEvent.setup();

    renderSettingsPage();

    await screen.findByDisplayValue("storefront");
    await user.click(screen.getByRole("button", { name: "Delete Project" }));

    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Delete Project" }),
    );

    expect(remove).toHaveBeenCalledWith("prj-1", "storefront");
  });
});

/**
 * The geometry-bearing anatomy of the read-mode page: heading type, each card's
 * border, header slots with their (wrapping) static copy, the content row's
 * layout, its h-9 field slot and the exact button box. jsdom cannot measure
 * pixels, so this pins the structure that decides them (w4/294).
 */
/** Classes a reservation adds to position its bar, not to size the box. */
function stripReserve(classes: string) {
  return classes
    .split(" ")
    .filter((name) => !["relative", "w-fit", "invisible"].includes(name))
    .join(" ");
}

/** A ready button's classes, or a reserved slot's invisible button box. */
function buttonBox(element: Element): string | undefined {
  if (element.matches("button")) return element.className;
  const reserved = element.querySelector(":scope > span.invisible");
  return reserved ? stripReserve(reserved.className) : undefined;
}

function settingsAnatomy(root: HTMLElement) {
  const container = root.querySelector<HTMLElement>(".max-w-2xl")!;
  const heading = container.firstElementChild!;
  const headingText =
    heading.tagName === "H1" ? heading : heading.firstElementChild!;
  return {
    container: container.className,
    heading: {
      classes: stripReserve(headingText.className),
      text: headingText.textContent,
    },
    cards: [
      ...container.querySelectorAll<HTMLElement>('[data-slot="card"]'),
    ].map((card) => {
      const header = card.querySelector('[data-slot="card-header"]')!;
      const content = card.querySelector<HTMLElement>(
        '[data-slot="card-content"]',
      )!;
      const row = content.firstElementChild!;
      const action = buttonBox(row);
      return {
        card: card.className,
        header: [...header.children].map((slot) => [
          slot.getAttribute("data-slot"),
          stripReserve(slot.className),
          slot.textContent,
        ]),
        content: content.className,
        row: action
          ? { button: action }
          : {
              layout: row.className,
              field: row.firstElementChild!.classList.contains("h-9"),
              button: buttonBox(row.lastElementChild!),
            },
      };
    }),
  };
}

describe("ProjectSettingsPageSkeleton (w4/294)", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it.each(["en", "zh"])(
    "previews the read-mode page's heading, cards, copy and action slots (%s)",
    async (language) => {
      await i18n.changeLanguage(language);
      projectsState.projects = [
        {
          id: "prj-1",
          name: "storefront",
          ownerId: "tea-1",
          serviceIds: [],
          databaseIds: [],
          keyValueIds: [],
        },
      ];
      const ready = renderSettingsPage();
      await screen.findByDisplayValue("storefront");
      const pending = render(<ProjectSettingsPageSkeleton />);

      expect(settingsAnatomy(pending.container)).toEqual(
        settingsAnatomy(ready.container),
      );
      // Inert and nameless: no controls, no unresolved project name.
      const frame = pending.container.querySelector("[data-skeleton-frame]")!;
      expect(frame).toHaveAttribute("aria-hidden", "true");
      expect(frame.querySelector("button, input")).toBeNull();
      expect(frame.textContent).not.toContain("storefront");
    },
  );
});
