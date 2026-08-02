export const en = {
  navigation: { status: "Status", activity: "Activity", sessions: "Sessions" },
  status: {
    title: "Status",
    body: "Resource health and latest deploys arrive in the supervision milestone.",
  },
  activity: {
    title: "Activity",
    body: "Events, deploy history, metrics, and read-only logs arrive next.",
  },
  sessions: {
    title: "Agent sessions",
    body: "Assign and supervise coding-agent work from your phone.",
    gated: "Requires ADR047 phase 1",
  },
  auth: {
    loadingTitle: "Opening bex",
    loadingBody: "Restoring your secure session.",
    signInTitle: "Sign in to bex",
    signInBody:
      "Continue in your system browser. The app never sees your password or MFA response.",
    signInAction: "Continue securely",
    signInError:
      "Sign-in did not complete. Check your connection and try again.",
    expiredTitle: "Session needs a connection",
    expiredBody:
      "Your access token expired while bex was offline. Reconnect to refresh it safely.",
    retry: "Try again",
    signOut: "Sign out",
  },
  workspace: {
    current: "Workspace",
    choose: "Choose workspace",
    switchLabel: "Switch workspace. Current workspace: %{name}",
    confirm: "Switch",
    cancel: "Cancel",
    offline:
      "Offline. Showing the current workspace without cached tenant data.",
    loadingTitle: "Opening your workspace",
    loadingBody: "Checking your bex workspace membership.",
    switchingTitle: "Switching workspace",
    switchingBody: "Clearing the previous workspace before opening the next.",
    emptyTitle: "No workspace yet",
    emptyBody: "Join or create a workspace on bex.co, then try again.",
    errorTitle: "Workspace unavailable",
    errorBody: "bex could not load your workspace. Try again.",
    offlineTitle: "Workspace needs a connection",
    offlineBody: "Reconnect to load workspace data safely.",
  },
  deepLink: {
    invalidTitle: "Invalid bex link",
    invalidBody: "This link does not contain a valid bex resource identifier.",
    serviceTitle: "Service",
    serviceBody: "Service supervision details arrive in the next milestone.",
    sessionTitle: "Agent session",
    sessionBody: "Session supervision details arrive with agent operations.",
  },
  common: {
    notFoundTitle: "This screen does not exist",
    backToStatus: "Back to Status",
    seeAll: "See all",
    notEnoughChartData: "Not enough data",
  },
};
