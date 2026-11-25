import {
  memo,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import {
  GitBranch,
  Box,
  Check,
  Layers,
  Lock,
  TriangleAlert,
} from "lucide-react";
import { GithubIcon } from "@/common/components/github-icon";
import { useTranslations } from "@/common/hooks/use-translations";
import { useIsMobile } from "@/common/hooks/use-mobile";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import {
  isValidGitUrl,
  parseGitHubPage,
  type GitHubPage,
} from "@/common/lib/utils/git-url";
import { dockerImageProblem } from "@/common/lib/utils/docker-image";
import { cn } from "@/common/lib/utils/utils";
import { CREATE_INPUT_CLASS } from "@/common/components/create-form";
import { useRepos, type RepoView } from "@/features/services/hooks/use-repos";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { useOnGitConnectionsChange } from "@/features/git/hooks/use-on-git-connections-change";
import { useConnectGitAccount } from "@/features/git/hooks/use-connect-git";
import { GitCredentialsMenu } from "@/features/git/components/git-credentials-menu";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import {
  REPO_HEADING_CLASS,
  REPO_LIST_CLASS,
  RepoListSkeleton,
} from "@/features/services/components/repo-list-skeleton";
import { sameRepo } from "@/features/services/lib/repo";

/** Where a service's code comes from. */
export type SourceTab = "github" | "git" | "image" | "build-source";

// From `sm`, underline tabs (Render's source tabs): the selected one marked by
// its rule, each as wide as its label plus a share of what is left, so a label
// wraps only in a language too long for the row.
// On a phone the tabs are a grid of bordered cells, two to a row, each an icon
// and a label at least 44 px tall: four underlined tabs across 300 px wrapped
// three labels onto two lines.
const TAB_LIST_CLASS =
  "items-stretch justify-start max-sm:grid max-sm:grid-cols-2 max-sm:gap-2 max-sm:border-b-0" as const;
const TAB_TRIGGER_CLASS = cn(
  // leading-5, a whole number of pixels: a 17.5 px line put everything under
  // the tabs on a half pixel, and the list's sticky heading showed a seam.
  "min-w-0 text-sm leading-5 whitespace-normal max-sm:gap-1 max-sm:px-1.5",
  // The ring's corners: an underline tab has none of its own.
  "focus-visible:rounded-sm",
  // An unselected cell is an enabled control: the fields' border and the
  // text colour, on the dialog's own surface. A grey fill under a grey label
  // is the disabled button's look, and read as three sources switched off.
  "max-sm:mb-0 max-sm:min-h-11 max-sm:rounded-md max-sm:border max-sm:border-muted-foreground/70 max-sm:py-2 max-sm:text-foreground",
  // The selected cell: a 2 px rule in the brand colour (the border and a ring
  // inside it) over a tint, and a heavier label.
  "max-sm:data-[state=active]:border-primary max-sm:data-[state=active]:bg-primary/5 max-sm:data-[state=active]:font-semibold max-sm:data-[state=active]:shadow-[inset_0_0_0_1px_var(--primary)] dark:max-sm:data-[state=active]:bg-primary/15 max-sm:last:odd:col-span-2",
  "sm:flex-auto sm:px-3",
);

// Update Source on a phone: the list takes the height the dialog has left,
// never under three rows.
const REPO_LIST_FILL_CLASS =
  "max-sm:h-auto max-sm:min-h-32 max-sm:flex-[1_1_0]";
// `min-h-full`, not `h-full`: a note taller than the list (two wrapped
// buttons in a list at its minimum) then scrolls from its top, where centring
// a fixed height cut the top off out of reach.
const REPO_LIST_NOTE_CLASS =
  "flex min-h-full flex-col items-center justify-center gap-2 px-6 text-center text-sm text-muted-foreground";

/** An action inside the list's note. The note is muted; its actions are not:
 *  "Try again" is the only way on from a failed list. */
function RepoListAction(
  props: Pick<ComponentProps<typeof Button>, "onClick" | "children">,
) {
  return (
    <Button
      variant="outline"
      size="sm"
      className="h-11 text-foreground sm:h-8"
      {...props}
    />
  );
}

/**
 * One repository in the list. Memoized: the list holds every repository the
 * workspace's connections grant, and an arrow key, a keystroke in a field
 * beside it or the accounts menu opening re-rendered all of them to change
 * two `tabIndex` attributes.
 */
const RepoRow = memo(function RepoRow({
  repo,
  selected,
  inUse,
  tabStop,
  revealRef,
  privateLabel,
  currentLabel,
  onFocusRepo,
  onSelect,
}: {
  repo: RepoView;
  selected: boolean;
  /** The repository the service uses now, once another is chosen. */
  inUse: boolean;
  tabStop: boolean;
  revealRef: ((row: HTMLButtonElement | null) => void) | undefined;
  privateLabel: string;
  currentLabel: string;
  onFocusRepo: (id: number) => void;
  onSelect: (repo: RepoView) => void;
}) {
  return (
    <button
      ref={revealRef}
      type="button"
      data-repo-row
      aria-pressed={selected}
      tabIndex={tabStop ? 0 : -1}
      onFocus={() => onFocusRepo(repo.id)}
      onClick={() => onSelect(repo)}
      className={cn(
        // relative: the phone's screen-reader-only "Private" is absolutely
        // placed, and outside a positioned row it escaped the scrolling list.
        "relative flex min-h-11 w-full items-center gap-2 px-3 py-2.5 text-left text-sm transition-colors hover:bg-muted",
        "focus-visible:bg-muted focus-visible:ring-[3px] focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset",
        // A bar as well as a tint: on the dark surface the tint alone does
        // not show. Focus keeps the tint: the ring is what it adds.
        selected &&
          "bg-primary/5 shadow-[inset_2px_0_0_var(--primary)] hover:bg-primary/10 focus-visible:bg-primary/10 dark:bg-primary/15 dark:focus-visible:bg-primary/20",
      )}
    >
      <GithubIcon className="size-4 shrink-0 text-muted-foreground" />
      {/* A phone has no hover to read a cut-off name by, so there the name
          wraps instead. */}
      <span
        className="min-w-0 font-medium max-sm:[overflow-wrap:anywhere] sm:truncate"
        title={repo.fullName}
      >
        {repo.fullName}
      </span>
      {/* The spaces are for the row's accessible name: without them its parts
          ran together ("acme/apiPrivatemain"). Flex layout drops them on
          screen. */}{" "}
      {repo.private && (
        <>
          {/* Outlined: a filled pill is the hovered row's own grey and
              vanished on it. */}
          <Badge
            variant="outline"
            className="font-normal text-muted-foreground max-sm:sr-only"
          >
            {privateLabel}
          </Badge>
          <Lock
            className="size-3.5 shrink-0 text-muted-foreground sm:hidden"
            aria-hidden="true"
          />{" "}
        </>
      )}
      {/* The repository the service uses now keeps a tag once another is
          chosen: the way back short of Cancel. Beside the name from `sm`; on
          a phone plain text where the tick would be, so the name keeps its
          line at 360 px. */}
      {inUse && (
        <>
          <Badge
            variant="outline"
            className="font-normal text-muted-foreground max-sm:ml-auto max-sm:border-0 max-sm:px-0"
          >
            {currentLabel}
          </Badge>{" "}
        </>
      )}
      <span className="ml-auto hidden shrink-0 items-center gap-1 pl-2 font-mono text-xs text-muted-foreground sm:flex">
        <GitBranch className="size-3" aria-hidden="true" />
        {repo.defaultBranch}
      </span>
      {selected ? (
        <Check
          className="size-4 shrink-0 text-primary max-sm:ml-auto"
          aria-hidden="true"
        />
      ) : (
        // The tick's place, so the branches stay in a column. A phone gives
        // it to the tag.
        <span
          className={cn(
            "size-4 shrink-0 max-sm:ml-auto",
            inUse && "max-sm:hidden",
          )}
          aria-hidden="true"
        />
      )}
    </button>
  );
});

// Every panel holds its own focusable control (the search box, the connect
// button, a URL or image field), so a panel is no tab stop of its own
// (`tabIndex={-1}` over Radix's 0): Tab goes from the tabs straight to the
// control, which shows its own focus ring.
const TAB_CONTENT_PROPS = { className: "mt-3", tabIndex: -1 } as const;

/** The image tab's own state. Supplying it is what adds the tab. */
export type ImageSourceOption = {
  value: string;
  onChange: (image: string) => void;
  registryCredentialId: string;
  onRegistryCredentialChange: (id: string) => void;
  /** Show the `$PORT` hint — only the port-binding types ever get one. */
  showPortHint: boolean;
  /** A server refusal of this image, shown under the field (w4/406). */
  serverError?: string | null;
};

const IMAGE_PROBLEM_KEYS = {
  malformed: "services.createImageMalformed",
  tooLong: "services.createImageTooLong",
  privateHost: "services.createImagePrivateHost",
} as const;

const REPO_KEY_STEPS: Record<string, number | "first" | "last" | undefined> = {
  ArrowDown: 1,
  ArrowUp: -1,
  Home: "first",
  End: "last",
};

const CONNECT_PROMPT_CLASS =
  "flex flex-col items-center gap-3 rounded-lg border p-6 text-center";

/**
 * Source selection shared by create-service, create-Blueprint, and Update
 * Source: a connected GitHub repo, a public git URL, and — for services — a
 * prebuilt image.
 *
 * The repo search box, the repo list, and the GitHub connection state live
 * here because no caller reads them. The selection itself stays lifted: both
 * forms' submit payloads and name auto-fill depend on it.
 *
 * `image` carries the third tab's state, so its presence is what decides
 * whether that tab exists — a caller that cannot deploy an image (Blueprints)
 * simply omits it rather than passing a flag plus unused handlers. The same
 * goes for `buildSource`, the Update Source dialog's build source tab
 * (w2/m254), whose content the caller renders.
 */
export function ServiceSourcePicker({
  tab,
  onTabChange,
  selectedRepo,
  onSelectRepo,
  currentRepo = null,
  gitUrl,
  onGitUrlChange,
  onGitHubPage,
  image,
  buildSource,
  titleKey = "services.createSourceTitle",
  idPrefix = "svc",
  fillHeight = false,
}: {
  tab: SourceTab;
  onTabChange: (tab: SourceTab) => void;
  selectedRepo: RepoView | null;
  onSelectRepo: (repo: RepoView) => void;
  /**
   * The clone URL of the repository in use (Update Source). Until the user
   * picks one, its row in the list is the selected one.
   */
  currentRepo?: string | null;
  gitUrl: string;
  onGitUrlChange: (url: string) => void;
  /**
   * A pasted GitHub folder page leaves its repository in the field and hands
   * its branch and folder here when the field loses focus, and a file page is
   * refused (w4/576). Without it the field takes the URL as typed.
   */
  onGitHubPage?: (page: GitHubPage) => void;
  image?: ImageSourceOption;
  buildSource?: ReactNode;
  /** The picker's own heading; null when a create row's label names it. */
  titleKey?: string | null;
  idPrefix?: string;
  /**
   * The picker sits in a column taller than it needs (Update Source on a
   * phone): it grows to fill it, and the repository list takes the height.
   */
  fillHeight?: boolean;
}) {
  const { t } = useTranslations();
  const isMobile = useIsMobile();
  const {
    repos,
    loading: reposLoading,
    error: reposError,
    refetch: refetchRepos,
  } = useRepos();
  const {
    connection,
    loading: connectionLoading,
    refetch: refetchConnection,
  } = useGitConnection();
  // In-place connect runs in a new tab so the create form survives (w8/m31),
  // through the single connect entry and its account chooser (ADR078 §3b).
  const { connect, busy: connecting } = useConnectGitAccount({ newTab: true });
  const [repoSearch, setRepoSearch] = useState("");
  const [credentialsOpen, setCredentialsOpen] = useState(false);
  // An account connected in the other tab, or disconnected from the menu.
  // useGitConnection above already polls the set.
  useOnGitConnectionsChange(refetchRepos, { poll: false });

  const gitHubDisconnected =
    !connectionLoading && connection?.connected !== true;
  // While the connection read is out, a repository in use (Update Source) or
  // repositories already listed say the workspace is connected, so the tab
  // takes the list's shape at once. Waiting for the read alone showed a
  // connected workspace the connect prompt's skeleton, then swapped it for
  // the list. Once the read answers, it decides: repositories still cached
  // after a disconnect do not keep the list up.
  const showRepoList =
    connection?.connected === true ||
    (connectionLoading && (currentRepo !== null || repos.length > 0));
  const repoListPending =
    reposLoading || (connectionLoading && repos.length === 0);
  const imageProblem = image ? dockerImageProblem(image.value) : null;
  // The Git URL is marked and announced like the image reference (w21/052).
  // A file page names no repository to build (w4/576).
  const gitUrlFilePage =
    onGitHubPage !== undefined && parseGitHubPage(gitUrl)?.file === true;
  const gitUrlInvalid =
    gitUrl !== "" && (!isValidGitUrl(gitUrl) || gitUrlFilePage);

  // When connected, GitCredentialsMenu owns the focus refetch. When disconnected
  // the menu isn't mounted, so the picker refreshes here to catch a connection
  // made from the empty-state Connect button's new tab (w8/m31).
  useEffect(() => {
    if (!gitHubDisconnected) return;
    const onFocus = () => {
      void refetchConnection();
      void refetchRepos();
    };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [gitHubDisconnected, refetchConnection, refetchRepos]);

  const filteredRepos = useMemo(
    () =>
      repos.filter(
        (r) =>
          !repoSearch ||
          r.fullName.toLowerCase().includes(repoSearch.toLowerCase()),
      ),
    [repos, repoSearch],
  );

  // Group the filtered repos by GitHub account so a workspace connected to
  // several accounts (org + personal, ADR075) reads as sections rather than one
  // undifferentiated list. Insertion order (repos already arrive account-ordered
  // from the backend aggregate) is preserved.
  const repoGroups = useMemo(() => {
    const groups: { account: string; repos: RepoView[] }[] = [];
    const byAccount = new Map<string, RepoView[]>();
    for (const r of filteredRepos) {
      const key = r.accountLogin || "";
      let bucket = byAccount.get(key);
      if (!bucket) {
        bucket = [];
        byAccount.set(key, bucket);
        groups.push({ account: key, repos: bucket });
      }
      bucket.push(r);
    }
    return groups;
  }, [filteredRepos]);
  // Only label groups when there is more than one account to distinguish.
  const showAccountHeadings = repoGroups.length > 1;
  const currentRepoId = useMemo(
    () =>
      currentRepo === null
        ? null
        : (repos.find((r) => sameRepo(r.htmlUrl, currentRepo))?.id ?? null),
    [repos, currentRepo],
  );
  const selectedRepoId = selectedRepo ? selectedRepo.id : currentRepoId;
  // The selected row may sit below the list's fold (the repository in use,
  // among a hundred). When a list mounts with it out of view, scroll the list
  // (only the list, never the dialog or the page) to leave one row above it.
  // A row chosen in a list already placed is only brought fully into view: a
  // click on the half row at the list's foot must not move the list several
  // rows under the pointer.
  // Layout offsets, not bounding boxes: the dialog opens scaled, and boxes
  // read mid-animation came out 5% short, which left the list resting between
  // two rows whenever the repositories were already cached.
  // A row's ref runs before its list's, so a list not yet recorded here is
  // one that is mounting.
  const placedListRef = useRef<HTMLElement | null>(null);
  const markListPlaced = useCallback((list: HTMLElement | null) => {
    placedListRef.current = list;
  }, []);
  const revealSelected = useCallback((row: HTMLButtonElement | null) => {
    const list = row?.closest<HTMLElement>("[data-repo-list]");
    if (!row || !list) return;
    const firstPlacing = placedListRef.current !== list;
    const heading = row.parentElement?.querySelector<HTMLElement>(
      "[data-repo-heading]",
    );
    const headingHeight = heading?.offsetHeight ?? 0;
    const top = row.offsetTop - list.scrollTop;
    const bottom = top + row.offsetHeight;
    if (top >= headingHeight && bottom <= list.clientHeight) return;
    list.scrollTop = Math.max(
      0,
      firstPlacing
        ? row.offsetTop - headingHeight - row.offsetHeight
        : top < headingHeight
          ? row.offsetTop - headingHeight
          : row.offsetTop + row.offsetHeight - list.clientHeight,
    );
  }, []);
  // Every caller passes a new `onSelectRepo` each render; the rows are
  // memoized, so they get one that never changes and reads the latest.
  const onSelectRepoRef = useRef(onSelectRepo);
  useLayoutEffect(() => {
    onSelectRepoRef.current = onSelectRepo;
  });
  const selectRepoRow = useCallback(
    (repo: RepoView) => onSelectRepoRef.current(repo),
    [],
  );
  // The list is one tab stop, moved through with the arrow keys: a workspace
  // lists hundreds of repositories, and Tab has to reach the fields after them.
  const [focusedRepoId, setFocusedRepoId] = useState<number | null>(null);
  const repoTabStopId = useMemo(() => {
    const shown = (id: number | null) =>
      id !== null && filteredRepos.some((r) => r.id === id);
    if (shown(focusedRepoId)) return focusedRepoId;
    if (shown(selectedRepoId)) return selectedRepoId;
    return filteredRepos[0]?.id ?? null;
  }, [filteredRepos, focusedRepoId, selectedRepoId]);
  function moveRepoFocus(event: KeyboardEvent<HTMLDivElement>) {
    const step = REPO_KEY_STEPS[event.key];
    if (step === undefined) return;
    const rows = [
      ...event.currentTarget.querySelectorAll<HTMLButtonElement>(
        "button[data-repo-row]",
      ),
    ];
    const at = rows.indexOf(document.activeElement as HTMLButtonElement);
    if (at < 0) return;
    event.preventDefault();
    const to =
      step === "first"
        ? 0
        : step === "last"
          ? rows.length - 1
          : Math.min(rows.length - 1, Math.max(0, at + step));
    rows[to]?.focus();
  }

  const repoListClass = cn(REPO_LIST_CLASS, fillHeight && REPO_LIST_FILL_CLASS);
  const privateLabel = t("services.createRepoPrivateBadge");
  const currentLabel = t("services.createRepoCurrent");
  const openCredentials = (
    <RepoListAction onClick={() => setCredentialsOpen(true)}>
      {t("services.createRepoOpenCredentials")}
    </RepoListAction>
  );

  return (
    // Columns that grow, down to the repository list: with `fillHeight` the
    // list takes the height the parent has left. On a page nothing gives the
    // picker more than it needs and they are plain stacks.
    <div className={cn("flex flex-col gap-3", fillHeight && "grow")}>
      {titleKey ? <Label>{t(titleKey)}</Label> : null}
      <Tabs
        value={tab}
        onValueChange={(v) => onTabChange(v as SourceTab)}
        className="grow"
      >
        <TabsList variant="underline" className={TAB_LIST_CLASS}>
          <TabsTrigger value="github" className={TAB_TRIGGER_CLASS}>
            <GithubIcon className="size-4" />
            {t("services.createTabGitHub")}
          </TabsTrigger>
          <TabsTrigger value="git" className={TAB_TRIGGER_CLASS}>
            <GitBranch className="size-4" />
            {t("services.createTabPublicGit")}
          </TabsTrigger>
          {image && (
            <TabsTrigger value="image" className={TAB_TRIGGER_CLASS}>
              <Box className="size-4" />
              {t("services.createTabImage")}
            </TabsTrigger>
          )}
          {buildSource !== undefined && (
            <TabsTrigger value="build-source" className={TAB_TRIGGER_CLASS}>
              <Layers className="size-4" />
              {t("services.sourceTabBuildSource")}
            </TabsTrigger>
          )}
        </TabsList>

        <TabsContent
          value="github"
          {...TAB_CONTENT_PROPS}
          className={cn(TAB_CONTENT_PROPS.className, "flex flex-col gap-2")}
        >
          {showRepoList ? (
            <>
              {/* One row at every width, the search straight above what it
                  filters. Stacked on a phone, the accounts button took a row
                  from the list. Beside it there the search is narrow, so its
                  placeholder is the one word; its name stays the full one. */}
              <div className="flex flex-wrap gap-2">
                {/* Where a language's accounts button leaves the search
                    under 104 px (Russian on a 360 px phone), the button
                    takes a line of its own instead of squeezing it. */}
                <Input
                  className="h-11 min-w-26 flex-[999_1_6.5rem] sm:h-9"
                  placeholder={t(
                    isMobile
                      ? "services.createRepoSearchShort"
                      : "services.createRepoSearchPlaceholder",
                  )}
                  value={repoSearch}
                  onChange={(e) => setRepoSearch(e.target.value)}
                  aria-label={t("services.createRepoSearchPlaceholder")}
                />
                <GitCredentialsMenu
                  open={credentialsOpen}
                  onOpenChange={setCredentialsOpen}
                />
              </div>
              {repoListPending ? (
                <RepoListSkeleton
                  className={fillHeight ? REPO_LIST_FILL_CLASS : undefined}
                />
              ) : reposError && repos.length === 0 ? (
                <div data-repo-list className={repoListClass}>
                  <div role="alert" className={REPO_LIST_NOTE_CLASS}>
                    <TriangleAlert
                      className="size-5 text-destructive"
                      aria-hidden="true"
                    />
                    <p className="text-foreground">
                      {t("services.createRepoLoadError")}
                    </p>
                    <RepoListAction onClick={() => void refetchRepos()}>
                      {t("common.tryAgain")}
                    </RepoListAction>
                  </div>
                </div>
              ) : filteredRepos.length === 0 ? (
                <div data-repo-list className={repoListClass}>
                  <div className={REPO_LIST_NOTE_CLASS}>
                    <p className="font-medium text-foreground">
                      {t(
                        repoSearch
                          ? "services.createRepoNoMatch"
                          : "services.createRepoEmpty",
                      )}
                    </p>
                    <p>
                      {t(
                        repoSearch
                          ? "services.createRepoNoMatchHint"
                          : "services.createRepoEmptyHint",
                      )}
                    </p>
                    {repoSearch ? (
                      <div className="flex flex-wrap justify-center gap-2">
                        <RepoListAction onClick={() => setRepoSearch("")}>
                          {t("services.createRepoClearSearch")}
                        </RepoListAction>
                        {openCredentials}
                      </div>
                    ) : (
                      openCredentials
                    )}
                  </div>
                </div>
              ) : (
                // eslint-disable-next-line jsx-a11y-x/no-noninteractive-element-interactions -- the group only moves focus between its own buttons; each row is the control
                <div
                  ref={markListPlaced}
                  role="group"
                  aria-label={t("services.createRepoListLabel")}
                  data-repo-list
                  className={repoListClass}
                  onKeyDown={moveRepoFocus}
                >
                  {repoGroups.map((group) => (
                    <div key={group.account || "_"}>
                      {showAccountHeadings && group.account && (
                        <div data-repo-heading className={REPO_HEADING_CLASS}>
                          <span className="min-w-0 truncate">
                            {group.account}
                          </span>
                          <span className="shrink-0 tabular-nums">
                            {group.repos.length}
                          </span>
                        </div>
                      )}
                      {group.repos.map((r) => {
                        const selected = r.id === selectedRepoId;
                        return (
                          <RepoRow
                            key={r.id}
                            repo={r}
                            selected={selected}
                            inUse={!selected && r.id === currentRepoId}
                            tabStop={r.id === repoTabStopId}
                            revealRef={selected ? revealSelected : undefined}
                            privateLabel={privateLabel}
                            currentLabel={currentLabel}
                            onFocusRepo={setFocusedRepoId}
                            onSelect={selectRepoRow}
                          />
                        );
                      })}
                    </div>
                  ))}
                </div>
              )}
            </>
          ) : connectionLoading ? (
            // The connect prompt's anatomy, the state a workspace sees until
            // it connects GitHub: a bare block was ~95 px shorter, so the
            // whole form jumped when the connections read settled (w4/434).
            <div
              inert
              aria-busy="true"
              data-preview="git-connect"
              className={CONNECT_PROMPT_CLASS}
            >
              <Skeleton className="size-8 rounded-full" />
              <div className="w-full">
                <span className="flex h-6 items-center justify-center">
                  <Skeleton className="h-4 w-32" />
                </span>
                <span className="mt-1 flex h-5 items-center justify-center">
                  <Skeleton className="h-3.5 w-full max-w-md" />
                </span>
                <span className="flex h-5 items-center justify-center sm:hidden">
                  <Skeleton className="h-3.5 w-2/3" />
                </span>
              </div>
              <Skeleton className="h-11 w-40 rounded-md sm:h-9" />
            </div>
          ) : (
            <div className={CONNECT_PROMPT_CLASS}>
              <GithubIcon className="size-8 text-muted-foreground" />
              <div>
                <p className="font-medium">
                  {t("services.createGitConnectPromptTitle")}
                </p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("services.createGitConnectPromptBody")}
                </p>
              </div>
              <Button
                onClick={connect}
                disabled={connecting}
                className={CREATE_INPUT_CLASS}
              >
                <GithubIcon className="size-4" />
                {t("git.connectButton")}
              </Button>
            </div>
          )}
        </TabsContent>

        <TabsContent value="git" {...TAB_CONTENT_PROPS}>
          <div className="space-y-2">
            <Label htmlFor={`${idPrefix}-git-url`}>
              {t("services.createPublicUrlLabel")}
            </Label>
            <Input
              id={`${idPrefix}-git-url`}
              className={CREATE_INPUT_CLASS}
              value={gitUrl}
              onChange={(e) => onGitUrlChange(e.target.value)}
              onBlur={() => {
                const page = onGitHubPage && parseGitHubPage(gitUrl);
                if (!page || page.file) return;
                onGitUrlChange(page.repo);
                onGitHubPage(page);
              }}
              placeholder={t("services.createPublicUrlPlaceholder")}
              autoComplete="off"
              aria-invalid={gitUrlInvalid}
              aria-describedby={
                gitUrlInvalid ? `${idPrefix}-git-url-error` : undefined
              }
            />
            {gitUrlInvalid ? (
              <p
                id={`${idPrefix}-git-url-error`}
                role="alert"
                className="text-sm text-destructive"
              >
                {t(
                  gitUrlFilePage
                    ? "services.createPublicUrlFileError"
                    : "services.createPublicUrlError",
                )}
              </p>
            ) : null}
          </div>
        </TabsContent>

        {image && (
          <TabsContent value="image" {...TAB_CONTENT_PROPS}>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor={`${idPrefix}-image`}>
                  {t("services.createImageLabel")}
                </Label>
                <Input
                  id={`${idPrefix}-image`}
                  className={CREATE_INPUT_CLASS}
                  value={image.value}
                  onChange={(e) => image.onChange(e.target.value)}
                  placeholder={t("services.createImagePlaceholder")}
                  autoComplete="off"
                  aria-invalid={
                    imageProblem !== null || Boolean(image.serverError)
                  }
                  aria-describedby={
                    imageProblem !== null || image.serverError
                      ? `${idPrefix}-image-error`
                      : undefined
                  }
                />
                {imageProblem !== null ? (
                  <p
                    id={`${idPrefix}-image-error`}
                    role="alert"
                    className="text-sm text-destructive"
                  >
                    {t(IMAGE_PROBLEM_KEYS[imageProblem])}
                  </p>
                ) : image.serverError ? (
                  <p
                    id={`${idPrefix}-image-error`}
                    role="alert"
                    className="text-sm text-destructive"
                  >
                    {image.serverError}
                  </p>
                ) : null}
                {image.showPortHint ? (
                  <p className="text-sm text-muted-foreground">
                    {t("services.createImagePortHint")}
                  </p>
                ) : null}
              </div>
              <RegistryCredentialSelect
                id={`${idPrefix}-registry-credential-image`}
                value={image.registryCredentialId}
                onValueChange={image.onRegistryCredentialChange}
                description={t("services.createRegistryCredentialDescription")}
              />
            </div>
          </TabsContent>
        )}
        {buildSource !== undefined && (
          <TabsContent value="build-source" {...TAB_CONTENT_PROPS}>
            {buildSource}
          </TabsContent>
        )}
      </Tabs>
    </div>
  );
}
