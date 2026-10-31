"""Independent output-oracle controls; these do not execute Erlang or a NIF."""
import base64
import hashlib
import unittest

from check import check_result


class OutputOracle(unittest.TestCase):
    def test_actual_semantic_fields_required(self):
        text = "bex café λ 雪".encode()
        good = base64.b64encode(text).decode() + "|42|" + hashlib.sha256(text).hexdigest()
        check_result(good)
        for bad in (good.replace("|42|", "|41|"), good[:-1] + "X",
                    base64.b64encode(b"bex cafe").decode() + "|42|" + hashlib.sha256(text).hexdigest(),
                    good + "|extra", "not-a-result"):
            with self.subTest(result=bad), self.assertRaises((AssertionError, ValueError)):
                check_result(bad)


if __name__ == "__main__":
    unittest.main()
