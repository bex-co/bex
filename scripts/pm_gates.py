# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""pm_gates.py — the checkable gates on a parked item's workstream line.

A parked item's gate lives in its workstream README line, inside
`**BLOCKED (…)**`. Two kinds of gate clear without anyone touching the board,
so each has one form a script can read (w5/m502):

- a release gate: ``pin must carry `<sha>`[, `<sha>`…]``: the newest
  `chore(deploy): pin platform images to <sha>` commit on origin/main pins a
  commit that contains every named one;
- a date gate: `date gate: YYYY-MM-DD` or `date gate: YYYY-MM-DDTHH:MMZ`:
  that instant (a bare date is 00:00Z) has passed.

A gate may carry both; it clears when both hold. Gates that need a person
(a decision, a credential, hardware) stay free text.

`scripts/pm-board-validate.py` refuses a BLOCKED summary that names a
production pin, deploy or roll, or a date gate, without its form, and
`scripts/loopx-guard.py board` lists the parked items whose forms hold under
`blocked.ready`.
"""

from __future__ import annotations

import dataclasses
import datetime
import re

# ``pin must carry `abc1234`, `def5678` and `0123abc` ``.
PIN_FORM = re.compile(
    r"pin must carry (`[0-9a-f]{7,40}`(?:(?:,\s*(?:and\s+)?|\s+and\s+)`[0-9a-f]{7,40}`)*)",
    re.IGNORECASE,
)
SHA = re.compile(r"`([0-9a-fA-F]{7,40})`")
DATE_FORM = re.compile(r"date gate: (\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2})(?::(\d{2}))?Z)?", re.IGNORECASE)
# Words that say the item waits for production to run some commit. Past-tense
# records ("production has carried it since pin `…`") do not match.
RELEASE_MENTION = re.compile(
    r"release gate"
    r"|production(?:'s|’s)? (?:pin|deploy|roll|release)"
    r"|\bpins? (?:must )?(?:carry|carries|carrying|contain|contains|containing)\b"
    r"|\bpin whose producer"
    r"|pin platform images"
    r"|\b(?:deploy|roll)s? (?:that |which )?(?:carry|carries|carrying|contain|contains|containing|pins)\b",
    re.IGNORECASE,
)
DATE_MENTION = re.compile(r"date gate|on or after \d{4}-\d{2}-\d{2}", re.IGNORECASE)
# The production pin, as deploy.yml writes it back to main.
PIN_SUBJECT = re.compile(r"chore\(deploy\): pin platform images to ([0-9a-f]{7,40})\b")
PIN_GREP = r"^chore\(deploy\): pin platform images to "
# A parked item's line links to it first: `](blocked/m489/README.md)`, `](blocked/075.md)`.
PARKED_LINK = re.compile(r"\]\((?:\./)?blocked/([^/)#\s]+)")
OPEN_PAREN, CLOSE_PAREN = "(（", ")）"


@dataclasses.dataclass(frozen=True)
class Gate:
    """The checkable part of a gate: commits the pin must carry, and the instant it may clear."""

    commits: tuple[str, ...] = ()
    after: float | None = None  # seconds since the epoch


def gate_text(line: str) -> str | None:
    """The BLOCKED summary on LINE, or None when the line has none.

    `**BLOCKED (…)**` gives the parenthesis, nested ones included; a colon
    after `BLOCKED` or after that parenthesis (`**BLOCKED（2026-10-29）：** …`)
    adds the rest of the line.
    """
    at = line.find("BLOCKED")
    if at < 0:
        return None
    rest = line[at + len("BLOCKED") :].lstrip("* ")
    text = ""
    if rest[:1] and rest[0] in OPEN_PAREN:
        depth = 0
        for index, char in enumerate(rest):
            if char in OPEN_PAREN:
                depth += 1
            elif char in CLOSE_PAREN:
                depth -= 1
                if depth == 0:
                    text, rest = rest[1:index], rest[index + 1 :].lstrip("* ")
                    break
        else:
            return rest[1:]  # unbalanced: the rest of the line is the gate
    if rest[:1] in (":", "："):
        text = f"{text} {rest[1:]}".strip()
    return text


def date_of(found: re.Match) -> float | None:
    year, month, day, hour, minute, second = (int(part) if part else 0 for part in found.groups())
    try:
        moment = datetime.datetime(year, month, day, hour, minute, second, tzinfo=datetime.timezone.utc)
    except ValueError:
        return None
    return moment.timestamp()


def parse(text: str) -> Gate:
    """The checkable part of gate TEXT; an empty Gate when it has none."""
    commits: list[str] = []
    for form in PIN_FORM.finditer(text):
        for sha in SHA.findall(form.group(1)):
            if sha.lower() not in commits:
                commits.append(sha.lower())
    dates = [date_of(found) for found in DATE_FORM.finditer(text)]
    valid = [date for date in dates if date is not None]
    return Gate(tuple(commits), max(valid) if valid else None)


def problems(text: str) -> list[str]:
    """Why gate TEXT is not in a checkable form where it needs one."""
    found = []
    if RELEASE_MENTION.search(text) and not PIN_FORM.search(text):
        found.append("waits for a production pin, deploy or roll but names no commit as ``pin must carry `<sha>` ``")
    if DATE_MENTION.search(text) and not DATE_FORM.search(text):
        found.append("names a date gate but not as `date gate: YYYY-MM-DD` (optionally `THH:MMZ`)")
    for date in DATE_FORM.finditer(text):
        if date_of(date) is None:
            found.append(f"`{date.group(0)}` is not a real date")
    return found


def parked_lines(readme: str) -> dict[str, str]:
    """Each parked item's line in a workstream README, by its name under `blocked/`.

    An item's line is the first unchecked list line whose first `blocked/`
    link names it; a later link on another item's line is a cross-reference.
    """
    lines: dict[str, str] = {}
    for line in readme.splitlines():
        if not line.lstrip().startswith("- [ ]"):
            continue
        found = PARKED_LINK.search(line)
        if found:
            lines.setdefault(found.group(1), line.strip())
    return lines
