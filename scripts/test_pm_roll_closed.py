import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "pm-roll-closed.py"
LINKS = HERE / "docs-links-validate.sh"
sys.path.insert(0, str(HERE))
import pm_gates  # noqa: E402

README = """# w9 — project workstream (worker9)

**Worker:** worker9.

## Milestones

- [ ] **m7** — [Open work](m7/README.md) (3 tasks)
- [x] **m5** — [Shipped](done/m5/README.md) — **DONE** (see [w8/012](../w8/done/012.md) and `](done/not-a-link)`)
- [ ] **m6** — [Parked](blocked/m6/README.md) — **BLOCKED (pin must carry `abcdef1`)**

- [x] **m4** — [Loose](done/m4/README.md) — **DONE**

  - follow-up: [evidence summary](done/m4/notes.md), [ADR](../../docs/adr.md#a-heading), <https://render.com/docs>

- [x ] **m3** — [Typo box](done/m3/README.md) — **DONE**

## Inbox

- [ ] **012** — [Open note](012.md)
- [x] **011** — [Closed note](done/011.md), cross-referencing [the parked m6](blocked/m6/README.md) and [a section](#inbox)

Prose about notes that closed long ago.
- [x] **010** — [Closed note](<done/010.md>)
More prose right after it.

- **009 transferred** — [to w8](../w8/done/009.md)

```text
- [x] not a line: a code sample
```
"""

TARGETS = [
    ".pm/w9/m7/README.md",
    ".pm/w9/done/m5/README.md",
    ".pm/w9/done/m4/README.md",
    ".pm/w9/done/m4/notes.md",
    ".pm/w9/done/m3/README.md",
    ".pm/w9/blocked/m6/README.md",
    ".pm/w9/012.md",
    ".pm/w9/done/011.md",
    ".pm/w9/done/010.md",
    ".pm/w8/done/012.md",
    ".pm/w8/done/009.md",
    ".pm/AGENTS.md",
]


class Fixture:
    """A throwaway repository: the README above and every file its links name, with origin/main at its first commit."""

    def __init__(self, readme: str = README):
        self._dir = tempfile.TemporaryDirectory()
        self.root = Path(self._dir.name)
        self.write(".pm/w9/README.md", readme)
        self.write("docs/adr.md", "# ADR\n\n## A heading\n")
        for rel in TARGETS:
            self.write(rel, "# target\n")
        self.git("init", "-q", "-b", "main")
        self.git("config", "user.email", "t@example.com")
        self.git("config", "user.name", "t")
        self.git("config", "commit.gpgsign", "false")
        self.git("config", "core.hooksPath", os.devnull)
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "base")
        self.git("update-ref", "refs/remotes/origin/main", "HEAD")

    def close(self):
        self._dir.cleanup()

    def git(self, *args: str) -> str:
        return subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True, text=True).stdout

    def write(self, rel: str, text: str):
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def read(self, rel: str) -> str:
        return (self.root / rel).read_text(encoding="utf-8")

    def roll(self, *extra: str, workstream: str = "w9") -> tuple[int, str]:
        cmd = [sys.executable, str(SCRIPT), workstream, "--root", str(self.root), "--date", "2026-11-24", *extra]
        done = subprocess.run(cmd, capture_output=True, text=True)
        return done.returncode, done.stdout + done.stderr

    def links(self) -> tuple[int, str]:
        env = {**os.environ, "DOCS_LINKS_ROOT": str(self.root)}
        done = subprocess.run(["bash", str(LINKS)], capture_output=True, text=True, env=env)
        return done.returncode, done.stdout + done.stderr


class RollTest(unittest.TestCase):
    def setUp(self):
        self.fx = Fixture()
        self.addCleanup(self.fx.close)

    def test_moves_closed_lines_and_keeps_everything_else(self):
        rc, out = self.fx.roll()
        self.assertEqual(rc, 0, out)
        readme = self.fx.read(".pm/w9/README.md")
        archive = self.fx.read(".pm/w9/done/closed-1.md")
        for gone in ("**m5**", "**m4**", "follow-up:", "**m3**", "**011**", "**010**"):
            self.assertNotIn(gone, readme)
            self.assertIn(gone, archive)
        for kept in ("## Milestones", "**m7**", "**m6**", "**012**", "Prose about notes", "**009 transferred**"):
            self.assertIn(kept, readme)
        # A line inside fenced code is a sample, not a closed line.
        self.assertIn("- [x] not a line: a code sample", readme)
        self.assertNotIn("code sample", archive)
        # The archive keeps each line's section and the README's order.
        self.assertLess(archive.index("## Milestones"), archive.index("**m5**"))
        self.assertLess(archive.index("**m5**"), archive.index("**m4**"))
        self.assertLess(archive.index("**m3**"), archive.index("## Inbox"))
        self.assertLess(archive.index("## Inbox"), archive.index("**011**"))
        # A loose item keeps its blank line and indented follow-up together.
        self.assertIn("**DONE**\n\n  - follow-up:", archive)

    def test_board_tools_read_the_same_open_and_parked_lines(self):
        before = self.fx.read(".pm/w9/README.md")
        self.assertEqual(self.fx.roll()[0], 0)
        after = self.fx.read(".pm/w9/README.md")
        self.assertEqual(pm_gates.parked_lines(after), pm_gates.parked_lines(before))
        open_lines = lambda text: [line for line in text.splitlines() if line.lstrip().startswith("- [ ]")]
        self.assertEqual(open_lines(after), open_lines(before))
        self.assertEqual(pm_gates.parse(pm_gates.gate_text(pm_gates.parked_lines(after)["m6"])).commits, ("abcdef1",))

    def test_links_resolve_from_done(self):
        rc, out = self.fx.links()
        self.assertEqual(rc, 0, out)
        self.assertEqual(self.fx.roll()[0], 0)
        rc, out = self.fx.links()
        self.assertEqual(rc, 0, out)
        archive = self.fx.read(".pm/w9/done/closed-1.md")
        self.assertIn("](m5/README.md)", archive)
        self.assertIn("](../../w8/done/012.md)", archive)
        self.assertIn("](../blocked/m6/README.md)", archive)
        self.assertIn("](../../../docs/adr.md#a-heading)", archive)
        self.assertIn("](<010.md>)", archive)
        self.assertIn("<https://render.com/docs>", archive)
        # An anchor-only link and a link-shaped code span are left as written.
        self.assertIn("](#inbox)", archive)
        self.assertIn("`](done/not-a-link)`", archive)

    def test_readme_indexes_each_part_and_closes_gaps(self):
        self.assertEqual(self.fx.roll()[0], 0)
        readme = self.fx.read(".pm/w9/README.md")
        self.assertIn(
            "## Closed items\n\nClosed lines leave this README for `done/closed-K.md` files once it passes 384 KiB", readme
        )
        self.assertIn(
            "- [done/closed-1.md](done/closed-1.md), rolled 2026-11-24: 5 closed lines from Milestones, Inbox", readme
        )
        self.assertNotIn("\n\n\n", readme)
        # Two open lines that sandwiched a closed one stay one list.
        self.assertIn("(3 tasks)\n- [ ] **m6**", readme)
        # Two paragraphs that sandwiched a closed line stay two paragraphs.
        self.assertIn("long ago.\n\nMore prose", readme)

    def test_splits_under_max_bytes_and_never_appends(self):
        filler = "".join(f"- [x] **{n:03d}** — [note](done/{n:03d}.md) {'x' * 300}\n" for n in range(100, 112))
        self.fx.write(".pm/w9/README.md", README.replace("## Inbox\n\n", "## Inbox\n\n" + filler))
        rc, out = self.fx.roll("--max-bytes", "1500")
        self.assertEqual(rc, 0, out)
        parts = sorted((self.fx.root / ".pm/w9/done").glob("closed-*.md"))
        self.assertGreater(len(parts), 2)
        for part in parts:
            self.assertLessEqual(part.stat().st_size, 1500, part.name)
        self.assertEqual(sum(part.read_text().count("\n- [x") + part.read_text().count("\n- [X") for part in parts), 17)
        first = {part.name: part.read_text() for part in parts}
        readme = self.fx.read(".pm/w9/README.md")
        for part in parts:
            self.assertIn(f"[done/{part.name}](done/{part.name})", readme)
        # A later roll writes the next part and adds it to the same list.
        self.fx.write(".pm/w9/README.md", readme.replace("- [ ] **012**", "- [x] **012**"))
        self.assertEqual(self.fx.roll()[0], 0)
        later = f"closed-{len(parts) + 1}.md"
        self.assertIn("**012**", self.fx.read(f".pm/w9/done/{later}"))
        self.assertEqual({part.name: part.read_text() for part in parts}, first)
        readme = self.fx.read(".pm/w9/README.md")
        self.assertEqual(readme.count("## Closed items"), 1)
        self.assertLess(readme.index(f"done/{parts[-1].name}"), readme.index(f"done/{later}"))

    def test_numbering_counts_origin_main(self):
        self.fx.write(".pm/w9/done/closed-4.md", "# taken on main\n")
        self.fx.git("add", "-A")
        self.fx.git("commit", "-q", "-m", "a roll elsewhere")
        self.fx.git("update-ref", "refs/remotes/origin/main", "HEAD")
        self.fx.git("reset", "-q", "--hard", "HEAD~1")
        self.assertEqual(self.fx.roll()[0], 0)
        self.assertTrue((self.fx.root / ".pm/w9/done/closed-5.md").exists())
        self.assertFalse((self.fx.root / ".pm/w9/done/closed-1.md").exists())

    def test_nothing_to_roll_changes_nothing(self):
        self.assertEqual(self.fx.roll()[0], 0)
        readme = self.fx.read(".pm/w9/README.md")
        rc, out = self.fx.roll()
        self.assertEqual(rc, 0, out)
        self.assertIn("no closed lines to roll", out)
        self.assertEqual(self.fx.read(".pm/w9/README.md"), readme)
        self.assertFalse((self.fx.root / ".pm/w9/done/closed-2.md").exists())

    def test_refuses_what_it_cannot_roll(self):
        self.assertEqual(self.fx.roll(workstream="../w9")[0], 2)
        self.assertEqual(self.fx.roll(workstream="w404")[0], 2)
        self.assertEqual(self.fx.roll("--max-bytes", "10")[0], 2)


if __name__ == "__main__":
    unittest.main()
