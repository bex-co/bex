import { beforeEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { MemberRow } from "@/features/team/components/member-row";
import { TeamMembersSkeleton } from "@/features/team/components/team-panel-skeleton";

// The root route's session (Kratos whoami), as UserNav reads it.
let session: unknown = null;
vi.mock("@/common/hooks/use-root-context", () => ({
  useRootContext: () => ({ session }),
}));

beforeEach(() => {
  session = null;
});

// The fixture of w7/094's production walk: 36 characters, which wrap over two
// lines at 390 where the placeholder fits on one.
const EMAIL = "qa-20261119-w7094-member@example.com";
const IDENTITY_ID = "9b2e4c1a-7d3f-4e8b-a6c5-1f0e2d3c4b5a";

/** The member cell's email and id line, in the frame or on the ready row. */
function ownCell(row: Element) {
  const cell = row.querySelector("td")!;
  return {
    // The first span after the avatar (w1/m648).
    email: cell.querySelector(":scope > div > span:not([data-slot])")!,
    subject: cell.querySelector(":scope > div.font-mono")!,
  };
}

function cellText(row: Element): string {
  const cell = row.querySelector("td")!.cloneNode(true) as Element;
  cell.querySelectorAll('[data-slot="monogram"]').forEach((m) => m.remove());
  return cell.textContent ?? "";
}

/** A class list without the colour, the one thing the frame changes. */
function layoutClasses(element: Element): string[] {
  return [...element.classList].filter(
    (name) => name !== "text-transparent" && name !== "text-muted-foreground",
  );
}

function renderFrame(canManage: boolean) {
  const { container, unmount } = render(
    <TeamMembersSkeleton canManage={canManage} />,
  );
  const row = container.querySelector("[data-skeleton-region=member-row]")!;
  return { row, unmount };
}

function renderReadyOwnRow(canManage: boolean) {
  const { container, unmount } = render(
    <table>
      <tbody>
        <MemberRow
          member={{
            subject: IDENTITY_ID,
            userId: "usr-self",
            email: EMAIL,
            role: canManage ? "ADMIN" : "DEVELOPER",
            createdAt: null,
            mfaEnabled: false,
            identityResolved: true,
            isSelf: true,
          }}
          canManage={canManage}
          changing={false}
          removing={false}
          onChangeRole={vi.fn()}
          onRemove={vi.fn()}
        />
      </tbody>
    </table>,
  );
  return { row: container.querySelector("tr")!, unmount };
}

describe("TeamMembersSkeleton's own row (w7/095)", () => {
  // At 390 the ready row wraps a long email over two lines and drops "You" to
  // a third; a frame that lays out a short placeholder stays on one, so Team
  // grows by 44 px when the page lands. The frame lays out the caller's own
  // text, transparent, in MemberRow's classes.
  it.each([
    ["an Admin", true],
    ["a member", false],
  ])(
    "lays out the session's email and identity id as %s's ready row does",
    (_who, canManage) => {
      session = { identity: { id: IDENTITY_ID, traits: { email: EMAIL } } };
      const frame = renderFrame(canManage);
      const pending = ownCell(frame.row);
      const ready = renderReadyOwnRow(canManage);
      const loaded = ownCell(ready.row);

      expect(pending.email).toHaveTextContent(EMAIL);
      expect(pending.email).toHaveClass("text-transparent");
      expect(layoutClasses(pending.email)).toEqual(layoutClasses(loaded.email));
      expect(pending.subject).toHaveTextContent(IDENTITY_ID);
      expect(pending.subject).toHaveClass("text-transparent");
      expect(layoutClasses(pending.subject)).toEqual(
        layoutClasses(loaded.subject),
      );
      // The same words, the avatar's monogram letter aside (w1/m648).
      expect(cellText(frame.row)).toBe(cellText(ready.row));
      expect(frame.row.querySelector("td .rounded-full")).not.toBeNull();
      frame.unmount();
      ready.unmount();
    },
  );

  it("keeps the placeholder text without a session", () => {
    const frame = renderFrame(true);
    const pending = ownCell(frame.row);
    expect(pending.email).toHaveTextContent("member@example.com");
    expect(pending.subject).toHaveTextContent(
      "00000000-0000-0000-0000-000000000000",
    );
    frame.unmount();
  });
});
