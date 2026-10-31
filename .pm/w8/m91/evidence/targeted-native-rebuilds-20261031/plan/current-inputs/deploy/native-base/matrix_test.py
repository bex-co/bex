import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("native_matrix", Path(__file__).with_name("matrix.py"))
native_matrix = importlib.util.module_from_spec(spec)
spec.loader.exec_module(native_matrix)


class PublishingInputs(unittest.TestCase):
    def source(self, row):
        return 'var nativeToolchains = {\n' + row + '\n}\n// nativeBun'

    def test_adopted_image_builds_from_original_pinned_input(self):
        original = 'node:14-bullseye@sha256:' + 'a' * 64
        platform = 'ghcr.io/bex-co/bex-native:node-14-bullseye@sha256:' + 'b' * 64
        source = self.source(
            '{line: "14",\nimage:    "' + platform + '",\nupstream: "' + original + '"},'
        )
        self.assertEqual(native_matrix.matrix(source), {
            "include": [{"name": "node-14-bullseye", "base": original}],
        })
        before_adoption = self.source('{line: "14", image: "' + original + '"},')
        self.assertEqual(native_matrix.matrix(source), native_matrix.matrix(before_adoption))

    def test_new_upstream_digest_requires_publication(self):
        before = self.source('{line: "14", image: "node:14-bullseye@sha256:' + 'a' * 64 + '"},')
        after = before.replace('a' * 64, 'b' * 64)
        self.assertNotEqual(native_matrix.matrix(before), native_matrix.matrix(after))

    def test_backport_lines_publish_before_unrelated_lines_without_dropping_any(self):
        names = ['elixir:1.20', 'node:14-bullseye', 'ruby:3.4-bookworm', 'python:3.10-bookworm']
        source = self.source('\n'.join(
            '{line: "x", image: "' + name + '@sha256:' + 'a' * 64 + '"},'
            for name in names
        ))
        published = [image['name'] for image in native_matrix.matrix(source)['include']]
        self.assertEqual(published[:2], ['node-14-bullseye', 'python-3.10-bookworm'])
        self.assertCountEqual(published, [name.replace(':', '-') for name in names])

    def test_refuses_unreviewed_or_recursive_inputs(self):
        for image in ('node:14-bullseye', 'ghcr.io/bex-co/bex-native:node-14@sha256:' + 'a' * 64):
            with self.subTest(image=image), self.assertRaises(ValueError):
                native_matrix.matrix(self.source('{line: "14", image: "' + image + '"},'))

    def test_multiline_rows_are_not_silently_dropped(self):
        image14 = 'node:14-bullseye@sha256:' + 'a' * 64
        image16 = 'node:16-bookworm@sha256:' + 'b' * 64
        source = self.source(
            '{line: "14", image: "' + image14 + '"},\n' +
            '{\nline: "16",\nimage: "' + image16 + '",\n},'
        )
        self.assertEqual([row['base'] for row in native_matrix.matrix(source)['include']], [image14, image16])


if __name__ == "__main__":
    unittest.main()
