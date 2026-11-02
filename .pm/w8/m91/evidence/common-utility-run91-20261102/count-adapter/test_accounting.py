"""Pure actual-accounting controls; never imports or runs the full auditor."""
import ast
import copy
import json
from pathlib import Path
import unittest

HERE = Path(__file__).resolve().parent
ACTUAL = Path("/private/tmp/bex-w8-run91-terminal-intake-bound-20261102-bymjzmq5/intake/amd64/extracted")
TREE = ast.parse((HERE / "audit-bootstrap-accounting-v2.py").read_bytes())
OLD = ast.parse((HERE / "before-audit.py").read_bytes())
NS = {}
selected = [n for n in TREE.body if isinstance(n, ast.FunctionDef)
            and n.name in ("check", "bootstrap_manifest_rows")]
exec(compile(ast.Module(body=selected, type_ignores=[]), "<reviewed-helper-only>", "exec"), NS)

def count_checks(tree):
    return [n for n in ast.walk(tree) if isinstance(n, ast.Expr)
            and isinstance(n.value, ast.Call) and isinstance(n.value.func, ast.Name)
            and n.value.func.id == "check" and len(n.value.args) == 2
            and isinstance(n.value.args[1], ast.Constant)
            and n.value.args[1].value in ("manifest path duplicates/count", "extractor incomplete")]

def validate(rows, bootstrap, extraction):
    retained = {row["path"]: row for row in rows}
    env = dict(NS, rows=rows, retained=retained, extraction=extraction)
    env["expected_rows"] = NS["bootstrap_manifest_rows"](retained, bootstrap)
    exec(compile(ast.Module(body=count_checks(TREE), type_ignores=[]), "<actual-auditor-count-checks>", "exec"), env)
    return env["expected_rows"]

class Accounting(unittest.TestCase):
    def setUp(self):
        self.rows = json.loads((ACTUAL / "artifact/MANIFEST.json").read_bytes())["files"]
        self.bootstrap = json.loads((ACTUAL / "artifact/bootstrap.json").read_bytes())
        self.extraction = json.loads((ACTUAL / "extraction.json").read_bytes())

    def test_actual_old_red_new_pass(self):
        self.assertEqual(len(self.rows), 357)
        self.assertEqual(self.extraction["completed_members"], 358)
        old = dict(NS, rows=self.rows, retained={r["path"]:r for r in self.rows},
                   extraction=self.extraction)
        with self.assertRaisesRegex(AssertionError, "manifest path duplicates/count"):
            exec(compile(ast.Module(body=count_checks(OLD), type_ignores=[]), "<unchanged-old-predicate>", "exec"), old)
        self.assertEqual(validate(self.rows, self.bootstrap, self.extraction), 357)

    def test_one_and_three_exact_source_shapes(self):
        rows = [x for x in self.rows if not x["path"].startswith("bootstrap-2.")]
        b = copy.deepcopy(self.bootstrap)
        b["attempts"] = b["attempts"][:1]
        e = dict(self.extraction, completed_members=356)
        self.assertEqual(validate(rows, b, e), 355)
        rows = copy.deepcopy(self.rows)
        b = copy.deepcopy(self.bootstrap)
        a = copy.deepcopy(b["attempts"][-1]); a["number"] = 3
        for stream in ("stdout", "stderr"):
            a[stream]["path"] = "bootstrap-3." + stream + ".log"
            rows.append(dict(a[stream]))
        b["attempts"].append(a)
        self.assertEqual(validate(rows, b, dict(self.extraction, completed_members=360)), 359)

    def test_missing_extra_or_relocated_bootstrap_file(self):
        for action in ("missing", "extra", "relocated"):
            with self.subTest(action=action):
                rows = copy.deepcopy(self.rows)
                if action == "missing":
                    rows = [x for x in rows if x["path"] != "bootstrap-2.stderr.log"]
                elif action == "extra":
                    rows.append(dict(rows[0], path="bootstrap-99.stderr.log"))
                else:
                    next(x for x in rows if x["path"] == "bootstrap-2.stderr.log")["path"] = "elsewhere.log"
                with self.assertRaises(AssertionError):
                    validate(rows, self.bootstrap, self.extraction)

    def test_receipt_number_path_size_hash_or_bound_mutations(self):
        for action in ("number", "bool-number", "path", "size", "bool-size", "hash", "empty", "too-many", "maximum"):
            with self.subTest(action=action):
                b = copy.deepcopy(self.bootstrap)
                a = b["attempts"][-1]
                if action == "number": a["number"] = 3
                elif action == "bool-number": b["attempts"][0]["number"] = True
                elif action == "path": a["stdout"]["path"] = "somewhere.log"
                elif action == "size": a["stdout"]["bytes"] += 1
                elif action == "bool-size": b["attempts"][0]["stdout"]["bytes"] = False
                elif action == "hash": a["stdout"]["sha256"] = "0" * 64
                elif action == "empty": b["attempts"] = []
                elif action == "too-many": b["attempts"] = b["attempts"] * 2
                elif action == "maximum": b["maximum_attempts"] = 4
                with self.assertRaises(AssertionError):
                    validate(self.rows, b, self.extraction)

    def test_no_compensated_payload_drop_duplicate_or_false_completion(self):
        nonboot = next(x for x in self.rows if not x["path"].startswith("bootstrap-"))
        for action in ("drop-original", "duplicate", "wrong-members", "not-complete"):
            with self.subTest(action=action):
                rows = copy.deepcopy(self.rows); e = dict(self.extraction)
                if action == "drop-original": rows.remove(nonboot)
                elif action == "duplicate": rows.append(dict(rows[0]))
                elif action == "wrong-members": e["completed_members"] = 356
                else: e["complete"] = False
                with self.assertRaises(AssertionError):
                    validate(rows, self.bootstrap, e)

if __name__ == "__main__":
    unittest.main(verbosity=2)
