/**
 * A shell command draft as it should reach bex-api (w8/m68): a nonblank
 * command keeps the exact bytes the user typed, and only a blank one
 * collapses to "".
 *
 * Whitespace can be part of a command. `printf '%s' tail\ ` ends in an
 * escaped space that Bash passes as part of the last argument, so trimming
 * it runs `printf '%s' tail\` and the service gets the wrong argument. The
 * build, start (Docker), pre-deploy, and cron commands therefore keep their
 * text verbatim, while names, paths, and schedules keep their own trimming.
 *
 * "" is each producer's existing "no command" value, so a whitespace-only
 * draft behaves exactly as an empty one did: the create input omits it, and
 * an editor's save clears the command.
 */
export function commandText(draft: string): string {
  return draft.trim() === "" ? "" : draft;
}
