import { useState } from "react";
import { isValidDnsLabel } from "@/common/lib/utils/dns-label";
import { repoNameSlug, gitUrlSlug, imageSlug } from "@/common/lib/utils/slug";
import { useServiceNameAvailability } from "@/features/services/hooks/use-service-name-availability";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { SourceTab } from "@/features/services/components/service-source-picker";

/**
 * The create-service name field: auto-filled from the chosen source, checked
 * for availability, and swapped to a free name when the derived one is taken.
 *
 * The three steps are order-dependent — the auto-fill sets `name`, the
 * availability check reads it, and the swap reacts to that check — so they are
 * kept in one hook rather than spread across the form.
 */
export function useServiceNameDraft({
  tab,
  selectedRepo,
  gitUrl,
  image,
  onRepoDefaultBranch,
}: {
  tab: SourceTab;
  selectedRepo: RepoView | null;
  gitUrl: string;
  image: string;
  /**
   * Called with the selected repo's default branch, for the form's branch
   * field. It runs during render, so it may only set the caller's own state.
   */
  onRepoDefaultBranch: (branch: string) => void;
}) {
  const [name, setName] = useState("");
  const [nameEdited, setNameEdited] = useState(false);

  // Auto-fill name + branch from source when the user hasn't manually typed a
  // name, on the first render and whenever the source or `nameEdited` changes.
  // `nameEdited` resets to false when the name is cleared (editName uses
  // `value !== ""`), so selecting a new source after clearing re-enables fill.
  // Adjusted during render, like the swap below, so no commit shows a source
  // with the previous source's name.
  const fillInputs = { tab, selectedRepo, gitUrl, image, nameEdited };
  const [filledFor, setFilledFor] = useState<typeof fillInputs | null>(null);
  const fillInputsChanged =
    filledFor === null ||
    filledFor.tab !== tab ||
    filledFor.selectedRepo !== selectedRepo ||
    filledFor.gitUrl !== gitUrl ||
    filledFor.image !== image ||
    filledFor.nameEdited !== nameEdited;
  if (fillInputsChanged) {
    setFilledFor(fillInputs);
    if (!nameEdited) {
      if (tab === "github" && selectedRepo) {
        setName(repoNameSlug(selectedRepo.fullName));
        onRepoDefaultBranch(selectedRepo.defaultBranch);
      } else if (tab === "git" && gitUrl) {
        const slug = gitUrlSlug(gitUrl);
        if (slug) setName(slug);
      } else if (tab === "image" && image) {
        const slug = imageSlug(image);
        if (slug) setName(slug);
      }
    }
  }

  const nameValid = isValidDnsLabel(name);
  const {
    taken: nameTaken,
    suggestion: nameSuggestion,
    checking: checkingName,
  } = useServiceNameAvailability(nameValid ? name : "");

  // Same repo connected twice (or any other auto-filled slug that's already
  // taken): prefill the suggested free name instead of leaving the user to
  // notice and fix it by hand — the auto-fill above just set `name` to a taken
  // slug, this swaps it for the free alternative once the check settles. Never
  // fires once the user has actually typed (nameEdited), nor in a pass that
  // just re-filled: that pass's `name` is the one being replaced, and React
  // re-renders at once with the new one.
  // Adjusted during render — React's own "store information from previous
  // renders" pattern (state, not a ref: refs can't be read/written during
  // render either) — rather than a useEffect, so it swaps at most once per
  // taken name instead of looping.
  const [lastAutoSwappedName, setLastAutoSwappedName] = useState<string | null>(
    null,
  );
  if (
    !fillInputsChanged &&
    !nameEdited &&
    nameTaken &&
    nameSuggestion &&
    name !== nameSuggestion &&
    lastAutoSwappedName !== name
  ) {
    setLastAutoSwappedName(name);
    setName(nameSuggestion);
  }

  return {
    name,
    nameValid,
    nameTaken,
    nameSuggestion,
    checkingName,
    /** A keystroke in the name field. Clearing it re-enables auto-fill. */
    editName(value: string) {
      setName(value);
      setNameEdited(value !== "");
    },
    /** The user accepted the suggested free name; treat it as hand-typed. */
    acceptSuggestion() {
      if (!nameSuggestion) return;
      setName(nameSuggestion);
      setNameEdited(true);
    },
  };
}
