#!/usr/bin/env python3
"""Read-only m188 BLOCKED migration/link plan. Writes NEW /tmp receipts, never the board."""
import argparse
import datetime
import hashlib
import json
import posixpath
from pathlib import Path
import re
import subprocess
import urllib.parse

OLD = ".pm/w1/m188"
NEW = ".pm/w1/blocked/m188"
TASK = re.compile(r"t002\.md\Z")
INLINE = re.compile(r"\]\(\s*(?P<destination><[^>\n]+>|[^\s)]+)")
REFERENCE = re.compile(r"^ {0,3}\[[^]\n]+\]:\s*(?P<destination><[^>\n]+>|[^\s]+)")


def sha(data):
    return hashlib.sha256(data).hexdigest()


def below(path, directory):
    return path == directory or path.startswith(directory + "/")


def future(path):
    if not below(path, OLD):
        return path
    relative = posixpath.relpath(path, OLD)
    if relative == ".":
        return NEW
    if TASK.fullmatch(relative):
        relative = "done/" + relative
    return NEW + "/" + relative


def virtual_exists(repo, path):
    if below(path, OLD):
        return False
    if below(path, NEW):
        relative = posixpath.relpath(path, NEW)
        if relative.startswith("done/") and TASK.fullmatch(relative[5:]) and (repo / OLD / relative[5:]).exists():
            return True
        return (repo / OLD / relative).exists()
    return (repo / path).exists()


def links(text):
    fence = None
    for line_number, line in enumerate(text.splitlines(), 1):
        marker = re.match(r"^ {0,3}(`{3,}|~{3,})", line)
        if marker:
            token = marker[1]
            if fence is None:
                fence = token
            elif token[0] == fence[0] and len(token) >= len(fence):
                fence = None
            continue
        if fence:
            continue
        masked = re.sub(r"(`+).*?\1", lambda match: " " * len(match[0]), line)
        for pattern in (INLINE, REFERENCE):
            for match in pattern.finditer(masked):
                yield line_number, match["destination"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    repo = args.repo.resolve()
    output = args.output.resolve()
    protected_output = output.with_name(output.stem + ".protected.json")
    if not output.is_relative_to(Path("/tmp").resolve()) or output.exists() or protected_output.exists():
        parser.error("Both receipt paths must be NEW files below /tmp")
    if not (repo / OLD).is_dir() or (repo / NEW).exists():
        parser.error("Expected current active milestone only and absent blocked destination; never recreate or overwrite a moved milestone")
    names = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "*.md"], cwd=repo).decode().split("\0")
    documents = {name for name in names if name and (repo / name).is_file()}
    documents.update(path.relative_to(repo).as_posix() for path in (repo / OLD).rglob("*.md") if path.is_file())
    def tree_snapshot():
        return {path.relative_to(repo).as_posix(): {"sha256": sha(path.read_bytes()), "bytes": path.stat().st_size} for path in sorted((repo / OLD).rglob("*")) if path.is_file()}
    milestone_snapshot = tree_snapshot()
    changes, stable, preexisting, protected_links, invalid_future, move_literal_missing = [], [], [], [], [], []
    inputs = {}
    for name in sorted(documents):
        raw = (repo / name).read_bytes()
        if not below(name, OLD) and b"m188" not in raw:
            continue
        text = raw.decode("utf-8")
        protected = "evidence" in Path(name).parts
        inputs[name] = sha(raw)
        for line, destination in links(text):
            angle = destination.startswith("<")
            value = destination[1:-1] if angle else destination
            parsed = urllib.parse.urlsplit(value)
            if parsed.scheme or parsed.netloc or not parsed.path or parsed.path.startswith("/"):
                continue
            target = posixpath.normpath(posixpath.join(posixpath.dirname(name), urllib.parse.unquote(parsed.path)))
            if target.startswith("../"):
                continue
            if not below(name, OLD) and not below(target, OLD):
                continue
            inputs[name] = sha(raw)
            next_source, wanted = future(name), future(target)
            literal = posixpath.normpath(posixpath.join(posixpath.dirname(next_source), urllib.parse.unquote(parsed.path)))
            exists = (repo / target).exists()
            new_path = posixpath.relpath(wanted, posixpath.dirname(next_source))
            if parsed.path.endswith("/"):
                new_path += "/"
            new_value = urllib.parse.quote(new_path, safe="/._-~")
            new_value += ("?" + parsed.query if parsed.query else "") + ("#" + parsed.fragment if parsed.fragment else "")
            if angle:
                new_value = "<" + new_value + ">"
            record = {"source": name, "line": line, "destination": destination, "future_source": next_source, "current_target": target, "future_target": wanted, "current_target_exists": exists, "virtual_target_exists": virtual_exists(repo, wanted)}
            if not exists:
                preexisting.append(record)
            if exists and not virtual_exists(repo, literal):
                move_literal_missing.append(dict(record, virtual_literal_target=literal, requires_reviewed_link_replacement_or_reader_locator=True))
            if protected:
                if literal != wanted or not exists:
                    protected_links.append(dict(record, preserve_bytes=True, virtual_literal_target_exists=virtual_exists(repo, literal), locator_target=wanted))
                continue
            if literal != wanted:
                changes.append(dict(record, replace_destination_with=new_value, preexisting_broken=not exists))
            else:
                stable.append(record)
            if not record["virtual_target_exists"]:
                invalid_future.append(record)
    protected_files = {name: dict(value, future_path=future(name)) for name, value in milestone_snapshot.items() if below(name, OLD + "/evidence")}
    # Root may complete/move t002 before the final plan. All already-done paths
    # remain stable; the other six pending tasks never acquire done destinations.
    tasks = []
    for number in range(1, 9):
        filename = f"t{number:03}.md"
        open_path, done_path = repo / OLD / filename, repo / OLD / "done" / filename
        present = [path for path in (open_path, done_path) if path.exists()]
        status = re.search(r"(?m)^status:\s*(\S+)", present[0].read_text()) if len(present) == 1 else None
        current = present[0].relative_to(repo).as_posix() if len(present) == 1 else None
        only_t002_move = number == 2 and open_path.exists() and not done_path.exists()
        tasks.append({"logical_id": "w1/m188/" + filename[:-3], "current_files": [path.relative_to(repo).as_posix() for path in present], "current_status": status[1] if status else None, "future_file": future(current) if current else None, "move_into_task_done_allowed_only_for_t002": only_t002_move, "completion_requires_root_actual_task_record_not_this_plan": only_t002_move, "already_done_preserved": done_path.exists() and not open_path.exists(), "pending_task_must_not_be_newly_completed": number >= 3, "unexpected_existing_done_pending_task_requires_review": number >= 3 and (done_path.exists() or (status and status[1] == "done")), "duplicate_or_missing_requires_resolution": len(present) != 1})
    for name, digest in inputs.items():
        if not (repo / name).is_file() or sha((repo / name).read_bytes()) != digest:
            parser.error("Markdown inputs changed while planning; rerun against the final tree")
    if tree_snapshot() != milestone_snapshot:
        parser.error("Milestone files changed while planning; wait for owned archive writes to finish, then rerun")
    protected_output.write_text(json.dumps(protected_files, sort_keys=True, indent=2) + "\n")
    report = {
        "schema": "bex.m188-blocked-migration-dryrun/v1",
        "at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "source_head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
        "helper_sha256": sha(Path(__file__).read_bytes()),
        "dry_run_only": True,
        "milestone_DoD_not_complete": True,
        "permitted_new_done_task_only": "w1/m188/t002",
        "pending_tasks": [f"w1/m188/t{number:03}" for number in range(3, 9)],
        "root_blocked_move_only_after_fresh_gate_and_final_human_archive_work": {"from": OLD, "to": NEW, "leave_old_stub_or_symlink": False, "keep_workstream_checkbox_unchecked": True},
        "task_plan": tasks,
        "human_markdown_link_changes": changes,
        "unchanged_human_links": len(stable),
        "preexisting_broken_links_separate": preexisting,
        "move_caused_literal_missing_before_reviewed_replacements": move_literal_missing,
        "move_caused_human_missing_after_planned_replacements": [record for record in invalid_future if record["current_target_exists"]],
        "preexisting_human_missing_still_missing_after_move": [record for record in invalid_future if not record["current_target_exists"]],
        "virtual_human_targets_missing_after_planned_replacements": invalid_future,
        "immutable_markdown_locator_advisories": protected_links,
        "source_markdown_input_hashes": inputs,
        "milestone_all_files_input_snapshot": milestone_snapshot,
        "input_consistency": "All selected Markdown and full milestone paths/hash/bytes were rechecked before writing this plan. A later edit/addition/removal invalidates this snapshot; rerun, never apply a stale receipt.",
        "immutable_evidence": {"all_existing_files_including_Markdown_preserve_bytes": True, "count": len(protected_files), "manifest": str(protected_output), "manifest_sha256": sha(protected_output.read_bytes()), "policy": "Never rewrite JSON, proofs, copied scripts/primary source, raw argv/absolute paths, custody/MANIFEST or Markdown below evidence. Only reviewed human Markdown destinations outside evidence may change."},
        "manual_blocked_migration_actions": [
            "Finish remaining implementable publication/handoff/archive work, and freshly verify the actual w8/m91/t007 qualified-common upstream gate still remains. Record owner, exact missing evidence and release condition; this is not a slow-CI or repeated-user-decision gate.",
            "Through pm record actual t002 publication acceptance and move only t002 to done if still open. Preserve t001 and all already-done files. Do not mark or move t003..t008 done: common integration, fixtures/rollout/parity and closeout DoDs remain open.",
            "After all actual human Markdown/artifact edits are ready, rerun THIS helper into new /tmp outputs. Check every duplicate/missing task, input hash/path snapshot and reviewed link replacement before mutating; this example grants no task completion or board-write authority.",
            "Through pm set the milestone Status to blocked and retain pending/open task states with the qualified-common dependency. Keep the workstream checkbox unchecked and name completed t001/t002 plus exact BLOCKED dependency/owner/release condition.",
            "Apply only reviewed human Markdown destination replacements, preserving dated historical claims. Move the whole milestone to blocked/m188 with no old stub/symlink; do not move it to milestone done/.",
            "Verify all protected evidence raw/stored bytes at the new mapped paths and resolve reviewed human links. Keep preexisting missing links separate from move-caused failures; never repair protected Markdown or raw proof path strings.",
            "Add the root-owned human locator note for old .pm/w1/m188 prefix to .pm/w1/blocked/m188 using expected original path plus SHA/bytes; resolve stored gzip separately from its decompressed original. Sibling MANIFEST-relative payload paths remain valid after a later blocked/done relocation.",
            "Run repository Markdown formatting with immutable evidence exclusions. Surface the genuine common integration dependency and /unblock w1 explanation while the root loop continues the remaining queue."
        ],
        "limitations": [
            "Focused standard inline/reference Markdown destinations only; fenced commands and inline code/raw argv intentionally ignored. Plain IDs/raw paths, HTML hrefs, absolute filesystem links and anchors are not rewritten/anchor-validated.",
            "Existing broken links, move-caused literal misses before replacement, and unresolved targets after planned replacements are distinct fields; no preexisting error is silently labeled as migration damage.",
            "Protected Markdown keeps historical relative links byte-exact; reader locator advisories identify the intended mapped target instead of changing signed/custody text.",
            "Root must regenerate after t002 is actually moved and all final new files/archives are present. This read-only helper never writes board/source, moves files, commits, calls CI, asserts common compatibility or closes any unmet DoD."
        ]
    }
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"plan": str(output), "plan_sha256": sha(output.read_bytes()), "helper_sha256": report["helper_sha256"], "human_link_changes": len(changes), "existing_broken": len(preexisting), "move_caused_human_missing_after_replacements": len(report["move_caused_human_missing_after_planned_replacements"]), "preexisting_human_missing_after_move": len(report["preexisting_human_missing_still_missing_after_move"]), "immutable_markdown_advisories": len(protected_links), "protected_files": len(protected_files)}, indent=2))


if __name__ == "__main__":
    main()
