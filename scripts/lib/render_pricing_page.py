#!/usr/bin/env python3
"""Read Render's pricing pages into the fixtures' form (w4/m690).

    render_pricing_page.py <pricing.md> <platform-features-by-plan.md> <out dir>

Render serves a markdown copy of render.com/pricing at /pricing.md and of
render.com/docs/platform-features-by-plan at the same path plus .md. This
writes one tab-separated file per captured table into <out dir>, named and
shaped exactly as its fixture in lego/backend/internal/pricing/testdata/
without the fixture's # header:

    render-plan-prices.tsv        Pricing Tiers: tier, price, description
    render-compute-prices.tsv     Services & Workers: compute, price, RAM, plan ID
    render-postgres-prices.tsv    Postgres: compute, price, RAM, connections, plan ID
    render-keyvalue-prices.tsv    Key Value: CPU, price, RAM, storage, connections, plan ID
    render-cron-prices.tsv        Cron Jobs: compute, price per minute, RAM, plan ID
    render-workflow-prices.tsv    Workflows: compute, price, RAM, plan ID
    render-sandbox-prices.tsv     Sandboxes: compute, CPU price, RAM, memory price, plan ID
    render-pricing-faq.tsv        the FAQ: question, answer
    render-plan-table.tsv         the plan comparison table: section, row, 4 plans
    render-features-by-plan.tsv   features by plan: category, feature, lowest plan

Price-table cells are kept as displayed. The comparison table and the
features page are normalized to the form their fixtures were first captured
in, from the page's embedded table data (w7/m429): a tick is "yes", a row's
"- " sub-row marker and its footnote (a trailing parenthetical sentence) are
dropped, and a value's parenthetical footnote follows it without the
parentheses. Markdown links keep their text.

It fails closed, naming what it could not read: a section or table that is
gone, a table whose columns changed, a table with no rows. A page it cannot
read must never pass as one whose rows all vanished.
"""

from __future__ import annotations

import os
import re
import sys

PLANS = ("Hobby", "Pro", "Scale", "Enterprise")

# (fixture, heading path to the table, the columns Render shows). Key Value's
# table has no header of its own: Render labels it "Column 1" … "Column 6".
PRICE_TABLES = (
    ("render-plan-prices.tsv", ("## Pricing Tiers",), ("Tier", "Price", "Description")),
    ("render-compute-prices.tsv", ("## Compute", "### Services & Workers"), ("Compute", "Price", "RAM", "Plan ID")),
    ("render-postgres-prices.tsv", ("## Compute", "### Postgres"), ("Compute", "Price", "RAM", "Connections", "Plan ID")),
    ("render-keyvalue-prices.tsv", ("## Compute", "### Key Value"), tuple(f"Column {n}" for n in range(1, 7))),
    ("render-cron-prices.tsv", ("## Compute", "### Cron Jobs"), ("Compute", "Price", "RAM", "Plan ID")),
    ("render-workflow-prices.tsv", ("## Compute", "### Workflows"), ("Compute", "Price", "RAM", "Plan ID")),
    ("render-sandbox-prices.tsv", ("## Compute", "### Sandboxes"), ("Compute", "Price", "RAM", "Connections", "Plan ID")),
)

LINK = re.compile(r"\[([^\]]*)\]\((?:<[^>]*>|[^)\s]*)\)")
PLAN_LINE = re.compile(r"^\*(Hobby|Pro|Scale|Enterprise)\* workspaces and higher\b")


class Unreadable(Exception):
    pass


def text(cell: str) -> str:
    """A cell or line as displayed: links keep their text, no emphasis or
    code ticks, whitespace collapsed."""
    cell = LINK.sub(r"\1", cell)
    cell = re.sub(r"[*`]", "", cell)
    return " ".join(cell.split())


def section(lines: list[str], path: tuple[str, ...], where: str) -> list[str]:
    """The lines under the heading path, each heading inside the previous
    one's section, up to the next heading of the same or a higher level."""
    for heading in path:
        level = len(heading) - len(heading.lstrip("#"))
        start = next((i for i, line in enumerate(lines) if line.rstrip() == heading), None)
        if start is None:
            raise Unreadable(f"{where} has no {' > '.join(path)} section")
        end = len(lines)
        for i in range(start + 1, len(lines)):
            hashes = len(lines[i]) - len(lines[i].lstrip("#"))
            if 0 < hashes <= level and lines[i][hashes : hashes + 1] == " ":
                end = i
                break
        lines = lines[start + 1 : end]
    return lines


def row_cells(line: str) -> list[str] | None:
    """A pipe-table line's cells; None for a separator row."""
    cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
    return None if all(re.fullmatch(r":?-+:?", cell) for cell in cells) else cells


def tables(lines: list[str]) -> list[list[list[str]]]:
    """Every pipe table in lines: its header row, then its body rows."""
    found, current = [], None
    for line in lines:
        if not line.strip().startswith("|"):
            current = None
            continue
        cells = row_cells(line)
        if cells is None:
            continue
        if current is None:
            current = []
            found.append(current)
        current.append(cells)
    return found


def one_table(lines: list[str], columns: tuple[str, ...], where: str) -> list[list[str]]:
    found = tables(lines)
    if len(found) != 1:
        raise Unreadable(f"{where} has {len(found)} tables, want 1")
    header, *rows = found[0]
    if tuple(header) != columns:
        raise Unreadable(f"{where}'s columns are now {header}, were {list(columns)}: re-capture its fixture")
    if not rows:
        raise Unreadable(f"{where} has no rows")
    for row in rows:
        if len(row) != len(columns):
            raise Unreadable(f"{where} has a row of {len(row)} cells, want {len(columns)}: {row}")
    return [[text(cell) for cell in row] for row in rows]


def trailing_parenthetical(value: str) -> tuple[str, str] | None:
    """(head, inner) when value ends in a balanced "(inner)" group."""
    if not value.endswith(")"):
        return None
    depth = 0
    for i in range(len(value) - 1, -1, -1):
        depth += {")": 1, "(": -1}.get(value[i], 0)
        if depth == 0:
            return value[:i].rstrip(), value[i + 1 : -1]
    return None


def comparison_title(cell: str) -> str:
    title = text(cell)
    if title.startswith("- "):
        title = title[2:]
    split = trailing_parenthetical(title)
    # A footnote is a sentence; "Key-value (Redis-compatible)" is the name.
    if split and " " in split[1]:
        title = split[0]
    return title


def comparison_value(cell: str) -> str:
    value = text(cell)
    if value == "✓":
        return "yes"
    split = trailing_parenthetical(value)
    if split and split[0]:
        value = f"{split[0]} {split[1]}"
    return value


def plan_table(lines: list[str], where: str) -> list[list[str]]:
    features = section(lines, ("## Workspace features",), where)
    rows, sections = [], 0
    for i, line in enumerate(features):
        if not line.startswith("### "):
            continue
        name = line[4:].strip()
        body = section(features[i:], (line.rstrip(),), f"{where} > Workspace features")
        sections += 1
        for row in one_table(body, ("Feature",) + PLANS, f"{where} > Workspace features > {name}"):
            rows.append([name, comparison_title(row[0])] + [comparison_value(cell) for cell in row[1:]])
    if sections == 0:
        raise Unreadable(f"{where} > Workspace features has no sections")
    return rows


def faq(lines: list[str], where: str) -> list[list[str]]:
    body = section(lines, ("## Frequently Asked Questions",), where)
    rows: list[list[str]] = []
    for line in body:
        if line.startswith("### "):
            rows.append([text(line[4:]), ""])
        elif rows and line.strip():
            rows[-1][1] = f"{rows[-1][1]} {text(line)}".strip()
    if not rows:
        raise Unreadable(f"{where} has no FAQ questions")
    for question, answer in rows:
        if not answer:
            raise Unreadable(f"{where} FAQ question {question!r} has no answer")
    return rows


def features_by_plan(lines: list[str], where: str) -> list[list[str]]:
    """Each feature with the lowest plan that includes it. A plan line
    ("*Pro* workspaces and higher gain access to:") opens a block of
    features: a table's rows, a list's top-level items, or ##### headings
    (whose own lists describe them)."""
    body = section(lines, ("## Feature categories",), where)
    rows, category, plan, block = [], None, None, None
    for line in body:
        stripped = line.strip()
        if line.startswith("### "):
            category, plan, block = text(line[4:]), None, None
            continue
        match = PLAN_LINE.match(stripped)
        if match:
            plan, block = match.group(1).lower(), "open"
            continue
        if not stripped or plan is None or category is None:
            continue
        if block == "open":
            if stripped.startswith("|"):
                block = "table"
            elif stripped.startswith("- "):
                block = "list"
            elif stripped.startswith("#####") or re.fullmatch(r"-{3,}", stripped):
                block = "headings"
            else:
                raise Unreadable(f"{where} > {category}: unrecognised feature block after the {plan} line: {stripped!r}")
        if block == "table" and stripped.startswith("|"):
            cells = row_cells(stripped)
            if cells is not None and cells[0] != "Feature":
                rows.append([category, text(cells[0]), plan])
        elif block == "list" and line.startswith("- "):
            rows.append([category, text(line[2:]), plan])
        elif block == "headings" and stripped.startswith("##### "):
            rows.append([category, text(stripped[6:]), plan])
    if not rows:
        raise Unreadable(f"{where} has no features")
    return rows


def write(out: str, name: str, rows: list[list[str]]) -> None:
    for row in rows:
        for cell in row:
            if "\t" in cell or "\n" in cell:
                raise Unreadable(f"{name}: a cell holds a tab or newline: {cell!r}")
    with open(os.path.join(out, name), "w", encoding="utf-8") as handle:
        for row in rows:
            handle.write("\t".join(row) + "\n")


def main(argv: list[str]) -> int:
    if len(argv) != 4:
        print("usage: render_pricing_page.py <pricing.md> <platform-features-by-plan.md> <out dir>", file=sys.stderr)
        return 2
    pricing_path, features_path, out = argv[1:4]
    try:
        with open(pricing_path, encoding="utf-8") as handle:
            pricing = handle.read().splitlines()
        with open(features_path, encoding="utf-8") as handle:
            features = handle.read().splitlines()
        if not pricing or not pricing[0].startswith("# Render Pricing"):
            raise Unreadable("render.com/pricing.md does not start with its '# Render Pricing' title")
        if not features or not features[0].startswith("# Platform Features by Plan"):
            raise Unreadable("platform-features-by-plan.md does not start with its '# Platform Features by Plan' title")
        outputs = {}
        for name, path, columns in PRICE_TABLES:
            where = "render.com/pricing.md > " + " > ".join(h.lstrip("# ") for h in path)
            outputs[name] = one_table(section(pricing, path, "render.com/pricing.md"), columns, where)
        outputs["render-pricing-faq.tsv"] = faq(pricing, "render.com/pricing.md")
        outputs["render-plan-table.tsv"] = plan_table(pricing, "render.com/pricing.md")
        outputs["render-features-by-plan.tsv"] = features_by_plan(features, "platform-features-by-plan.md")
    except Unreadable as err:
        print(f"::error title=Render pricing page unreadable::{err}", file=sys.stderr)
        return 1
    os.makedirs(out, exist_ok=True)
    for name, rows in outputs.items():
        write(out, name, rows)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
