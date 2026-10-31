#!/usr/bin/env python3
"""Read-only m188 closeout link plan. Writes NEW /tmp receipts, never the board."""
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
NEW = ".pm/w1/done/m188"
TASK = re.compile(r"t00[2-8]\.md\Z")
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
        parser.error("Expected current open milestone only; never recreate a moved milestone")
    names = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "*.md"], cwd=repo).decode().split("\0")
    documents = {name for name in names if name and (repo / name).is_file()}
    documents.update(path.relative_to(repo).as_posix() for path in (repo / OLD).rglob("*.md") if path.is_file())
    changes, stable, preexisting, protected_links, invalid_future = [], [], [], [], []
    inputs = {}
    for name in sorted(documents):
        raw = (repo / name).read_bytes()
        if not below(name, OLD) and b"m188" not in raw:
            continue
        text = raw.decode("utf-8")
        protected = "evidence" in Path(name).parts
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
    protected_files = {}
    for path in sorted((repo / OLD / "evidence").rglob("*")):
        if path.is_file():
            relative = path.relative_to(repo).as_posix()
            raw = path.read_bytes()
            protected_files[relative] = {"future_path": future(relative), "bytes": len(raw), "sha256": sha(raw)}
    protected_output.write_text(json.dumps(protected_files, sort_keys=True, indent=2) + "\n")
    tasks = []
    for number in range(2, 9):
        filename = f"t{number:03}.md"
        open_path, done_path = repo / OLD / filename, repo / OLD / "done" / filename
        present = [path for path in (open_path, done_path) if path.exists()]
        status = re.search(r"(?m)^status:\s*(\S+)", present[0].read_text()) if len(present) == 1 else None
        tasks.append({"logical_id": "w1/m188/" + filename[:-3], "current_files": [path.relative_to(repo).as_posix() for path in present], "current_status": status[1] if status else None, "future_file": NEW + "/done/" + filename, "move_into_task_done_needed": open_path.exists() and not done_path.exists(), "duplicate_or_missing_requires_resolution": len(present) != 1})
    report = {"schema": "bex.m188-closeout-dryrun/v2", "at": datetime.datetime.now(datetime.timezone.utc).isoformat(), "source_head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(), "helper_sha256": sha(Path(__file__).read_bytes()), "dry_run_only": True, "DoD_not_asserted_complete": True, "root_move_only_after_actual_DoD": {"from": OLD, "to": NEW, "leave_old_stub_or_symlink": False}, "task_plan": tasks, "human_markdown_link_changes": changes, "unchanged_human_links": len(stable), "preexisting_broken_links_separate": preexisting, "virtual_human_targets_missing": invalid_future, "immutable_markdown_locator_advisories": protected_links, "source_markdown_input_hashes": inputs, "immutable_evidence": {"all_existing_files_preserve_bytes": True, "count": len(protected_files), "manifest": str(protected_output), "manifest_sha256": sha(protected_output.read_bytes()), "policy": "Do not rewrite JSON, proof, copied scripts/primary source, raw argv/absolute paths, SHA/custody/MANIFEST or Markdown beneath evidence. Only human Markdown pointing to evidence may change."}, "manual_closure_actions": ["First finish and record actual publication/adoption, six real fixtures, deployed-source/Ready rollout+census and all remaining task DoD; this plan grants no done state.", "Immediately before closing t008 rerun this helper against current tree so new live-acceptance.md and individually completed done/tasks are included.", "Through pm update remaining task frontmatter status and move only still-open task files; retain stable logical IDs/dependencies. Synchronize milestone Status/task DONE rows and workstream checked box.", "Apply reviewed human Markdown destination changes; preserve existing dated historical claims. Add concise dated actual closure pointers in done143/136 if needed.", "Move whole milestone to done/m188 without old stub/symlink. Verify protected manifest byte hashes after relocation, and human virtual targets after approved replacements.", "Append dated reader-only locator note mapping old .pm/w1/m188 prefix to .pm/w1/done/m188; use retained MANIFEST/custody source-to-archive mappings for original /tmp paths, never rewrite raw evidence.", "Run repository Markdown formatting with existing evidence exclusions; do not format immutable archived Markdown."], "limitations": ["Focused standard inline/reference Markdown destinations only; fenced command history and inline code/argv intentionally ignored. Plain IDs, raw paths, HTML hrefs, absolute filesystem links and anchors are not rewritten/anchor-validated.", "Existing broken links are reported separately; no current error is silently counted as move-caused or automatically fixed.", "Protected Markdown can retain historical relative links that no longer resolve literally after move; locator advisories preserve bytes and identify intended mapped target.", "This snapshot is stale as soon as human docs/evidence/tasks change; rerun at real closeout. No repository/board/Git/CI mutation or actual move was performed."]}
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"plan": str(output), "plan_sha256": sha(output.read_bytes()), "helper_sha256": report["helper_sha256"], "human_link_changes": len(changes), "existing_broken": len(preexisting), "virtual_human_missing": len(invalid_future), "immutable_markdown_advisories": len(protected_links), "protected_files": len(protected_files)}, indent=2))


if __name__ == "__main__":
    main()
