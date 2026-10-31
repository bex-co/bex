"""Exercise the retained upstream finder with the actual default-only patch."""
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import unittest
from unittest import mock


HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("prepare_node14_tools", HERE / "prepare-node14-tools.py")
tools = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tools)


class Node14ToolsTests(unittest.TestCase):
    def test_real_finder_retains_explicit_priority_and_uses_private_default(self):
        node = shutil.which("node")
        if not node:
            self.skipTest("node is required to execute the retained upstream JavaScript finder")
        # Execute upstream's actual findPython/runChecks, not a model of it.
        program = r"""
const assert = require('assert');
const vm = require('vm');
const input = JSON.parse(require('fs').readFileSync(0, 'utf8'));
function select(source, env, config, accepted) {
  const sandbox = {module: {exports: {}}};
  vm.runInNewContext(source, sandbox);
  const attempted = [];
  let selected;
  sandbox.module.exports.findPython.call({env, configPython: config, win: false,
    addLog() {}, log: {silly() {}}, fail() { throw Error('no Python'); },
    checkCommand(command, next) {
      attempted.push(command);
      if (accepted.includes(command)) selected = command; else next(Error('unavailable'));
    }});
  return {selected, attempted};
}
const privatePython = input.private;
assert.equal(select(input.original, {}, null, ['python3']).selected, 'python3');
assert.equal(select(input.patched, {}, null, [privatePython, 'python3']).selected, privatePython);
assert.equal(select(input.patched, {PYTHON: '/user/env'}, '/user/npm', ['/user/npm', '/user/env', privatePython]).selected, '/user/npm');
assert.equal(select(input.patched, {PYTHON: '/user/env'}, null, ['/user/env', privatePython]).selected, '/user/env');
assert.equal(select(input.patched, {PYTHON: '/user/python313'}, null, ['/user/python313', privatePython]).selected, '/user/python313');
assert.equal(select(input.patched, {NODE_GYP_FORCE_PYTHON: '/user/force', PYTHON: '/user/env'}, '/user/npm', ['/user/force', '/user/npm', '/user/env', privatePython]).selected, '/user/force');
assert.deepEqual(select(input.patched, {PYTHON: '/invalid/env'}, '/invalid/npm', [privatePython]).attempted, ['/invalid/npm', '/invalid/env', privatePython]);
assert.equal(select(input.patched, {}, null, ['python3']).selected, 'python3');
assert.throws(() => select(input.patched, {NODE_GYP_FORCE_PYTHON: '/invalid/force'}, null, [privatePython]), /no Python/);
console.log('9 actual upstream finder controls passed');
"""
        for name in ("node14", "node16"):
            upstream = (HERE / ("testdata/" + name + "-find-python.js")).read_text()
            patched = tools.patch_finder(upstream)
            with self.subTest(finder=name):
                result = subprocess.run([node, "-e", program], input=json.dumps({"original": upstream,
                    "patched": patched, "private": tools.PYTHON}), capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("9 actual upstream finder controls passed", result.stdout)

    def test_patch_is_idempotent_and_refuses_changed_priority(self):
        for name in ("node14", "node16"):
            upstream = (HERE / ("testdata/" + name + "-find-python.js")).read_text()
            patched = tools.patch_finder(upstream)
            self.assertEqual(tools.patch_finder(patched), patched)
            # A previously prepared Node14 tree is upgraded without duplicating
            # its private check, including the new first-default placement.
            previous = upstream.replace(tools.ANCHOR, tools.PREVIOUS_PRIVATE_CHECK + tools.ANCHOR)
            self.assertEqual(tools.patch_finder(previous), patched)
            for source in (upstream.replace(tools.ANCHOR, "changed"),
                           upstream.replace(tools.ANCHOR3, "changed"), upstream + upstream,
                           upstream.replace("arg: this.env.PYTHON", "arg: other"),
                           upstream.replace("arg: this.configPython", "arg: other"),
                           upstream.replace("if (this.env.NODE_GYP_FORCE_PYTHON)", "if (other)")):
                with self.subTest(finder=name, source=source[-60:]), self.assertRaises(ValueError):
                    tools.patch_finder(source)

    def test_relocated_interpreter_must_have_exact_version_prefix_and_executable(self):
        completed = mock.Mock(stdout=json.dumps(["3.9.25", tools.PREFIX, tools.PYTHON]))
        finder = mock.Mock()
        finder.read_text.return_value = (HERE / "testdata/node14-find-python.js").read_text()
        with mock.patch.object(tools.subprocess, "run", return_value=completed) as run, mock.patch.object(tools, "FINDER", finder):
            tools.prepare("3.9.25")
            self.assertEqual(run.call_args.args[0][:3], [tools.PYTHON, "-I", "-c"])
            self.assertNotIn("env", run.call_args.kwargs)
            self.assertIn("distutils.version", run.call_args.args[0][3])
            finder.write_text.assert_called_once()
            for identity in (["3.9.24", tools.PREFIX, tools.PYTHON], ["3.9.25", "/usr/local", tools.PYTHON],
                             ["3.9.25", tools.PREFIX, "/usr/bin/python3"]):
                completed.stdout = json.dumps(identity)
                with self.subTest(identity=identity), self.assertRaises(ValueError):
                    tools.prepare("3.9.25")
            with self.assertRaises(ValueError):
                tools.prepare("3.11.0")

    def test_historical_embedded_helper_is_the_same_implementation(self):
        embedded = HERE.parents[1] / "lego/operator/internal/build/nativenodepython.py"
        self.assertEqual(embedded.read_bytes(), (HERE / "prepare-node14-tools.py").read_bytes())

    def test_common_factory_carries_the_helper(self):
        name = "native-common/prepare-node14-tools.py"
        self.assertIn("COPY " + name + " /usr/libexec/bex-native/prepare-node14-tools.py", (HERE / "Dockerfile").read_text())
        self.assertIn("!" + name, (HERE / "Dockerfile.dockerignore").read_text())


if __name__ == "__main__":
    unittest.main()
