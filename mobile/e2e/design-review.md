# Mobile design review

The mobile companion should make service health, agent progress, and the next
useful action easy to find. This pass keeps the supervision scope and existing
API and authentication contracts.

## Changes

- Environment-variable viewing and editing stay on the dashboard. Mobile service
  details omit the card and its read, refresh, reveal, and update requests; the
  native OAuth scope remains read/write only.
- Top-level pages share one compact row: a hamburger menu, the current workspace
  name in 20-point semibold, and the page action. The menu opens the drawer with
  a 44-point target; workspace selection lives inside the drawer. Long names
  truncate visually while remaining complete for screen readers. The separate
  workspace row and redundant caret are removed. The active tab identifies the
  page; Alerts has one home in the tab bar.
- The drawer uses the dashboard's bex logo, bundled locally from
  `dashboard/public/logo.png` with its original black-and-white artwork.
- Detail pages, logs, Settings, the composer, and selection sheets share a centered
  toolbar with equal action slots and 44-point controls. Resource detail headers
  stay visible while content scrolls. The composer owns its modal safe-area
  provider so the toolbar stays below the status bar.
- Resource health precedes usage. Search combines with resource-type filters;
  clearing filters restores the list. Unknown states remain explicitly unknown,
  and failed reads do not masquerade as an empty workspace.
- Sessions has a named New action, an actionable empty state, clearer repository
  rows, and retry/partial-refresh feedback.
- Alerts puts received items first, explains the empty state, and hides Enable
  when push is unavailable. It no longer offers a nonfunctional refresh gesture.
- Shared cards own their appearance while scroll containers own spacing. Card
  actions stack at narrow widths or larger text settings. Tab screens avoid
  applying the bottom safe area twice.
- Primary actions have sufficient text contrast in both themes, readable disabled
  states, and busy semantics that prevent repeat taps. Detail back buttons describe
  the actual history behavior.
- New copy is available in English and Chinese.

## Verification

Reviewed the native iOS app with the official `expo-mcp` local stdio server against
Metro on port 8081. Simulator screenshots and React Native DevTools supplemented
MCP for navigation and visual inspection. The live workspace briefly failed to
load during hot reload, then recovered; a failure is presented with a retry.
Temporary runtime inspection/fixture scripts were kept outside application source.

Visual checks included light/dark screens, Chinese copy, larger iOS text, and a
320-point constrained native Sessions layout. The session composer opened and
closed without submitting a task; a no-match resource search exposed Clear
filters and returned to the resource list. Populated agent sessions used
read-only preview data.

Automated checks cover search/type composition, project identity, unknown states,
duplicate resource projections, empty health counts, translation parity, and
primary-button contrast. Required formatting, TypeScript, lint, unit tests, Expo
configuration, and both native export builds are run with this change.

This is simulator and bundle verification, not physical-device qualification.
Production notification delivery and submitting agent tasks remain outside this
visual review. Device release gates in the other `e2e/` documents still apply.
