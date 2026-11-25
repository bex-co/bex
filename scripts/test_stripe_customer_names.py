"""Offline contract tests for scripts/stripe-customer-names.py (w6/m628 t004).

Every Stripe and database call goes through a fake; nothing here runs the
stripe CLI, psql or kubectl.
"""

import contextlib
import importlib.util
import io
import json
from pathlib import Path
from unittest import TestCase, main, mock


SCRIPT = Path(__file__).with_name("stripe-customer-names.py")
SPEC = importlib.util.spec_from_file_location("stripe_customer_names", SCRIPT)
NAMES = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(NAMES)

# Names hold the characters psql rows or a shell would split on.
ACME = "Acme | Co\tLtd = 1"
LABELS = {"tea-acme": ACME, "tea-beta": "Beta's Workspace", "tea-signup": "tea-signup"}


def customer(cid, workspace="tea-acme", name=None, livemode=False, **meta):
    """A Customer as Stripe lists it; tagged with workspace unless it is ""."""
    metadata = dict(meta)
    if workspace:
        metadata["bex_workspace"] = workspace
    return {"id": cid, "object": "customer", "livemode": livemode,
            "name": workspace if name is None else name, "metadata": metadata}


class FakeStripe:
    """Serves the Customer list in pages and records every call."""

    def __init__(self, customers, page_size=100, fail=()):
        self.customers = {c["id"]: dict(c) for c in customers}
        self.order = [c["id"] for c in customers]
        self.page_size = page_size
        self.fail = set(fail)
        self.calls = []

    def __call__(self, *args):
        self.calls.append(args)
        method, path, *data = args
        if (method, path) == ("get", "/v1/customers"):
            params = dict(data[i + 1].split("=", 1) for i in range(0, len(data), 2))
            start = self.order.index(params["starting_after"]) + 1 if "starting_after" in params else 0
            ids = self.order[start:start + self.page_size]
            return {"object": "list", "data": [self.customers[i] for i in ids],
                    "has_more": start + self.page_size < len(self.order)}
        cid = path.rsplit("/", 1)[1]
        if method == "get":
            return self.customers[cid]
        if cid in self.fail:
            raise RuntimeError(f"stripe post {path}\nInvalid name: {data[1].split('=', 1)[1]}")
        self.customers[cid]["name"] = data[1].split("=", 1)[1]
        return self.customers[cid]

    def writes(self):
        return [c for c in self.calls if c[0] != "get"]


def run(argv, fake, labels=LABELS):
    """main() with fakes -> (exit code, stdout, stderr)."""
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        code = NAMES.main(argv, stripe_fn=fake, read_labels_fn=lambda: labels)
    return code, out.getvalue(), err.getvalue()


class PlanTest(TestCase):
    def plan(self, customers, labels=LABELS, live=False):
        return NAMES.plan(customers, labels, live)

    def test_customer_without_bex_workspace_is_never_planned(self):
        # The shared account's other products: a name that looks like a bex id
        # is no licence to touch a Customer without the metadata tag.
        renames, counts = self.plan([
            customer("cus_1", workspace="", name="tea-acme"),
            customer("cus_2", workspace="", name="tea-acme", other_product="tea-acme"),
            {"id": "cus_3", "name": "tea-acme", "livemode": False, "metadata": {"bex_workspace": ""}},
            {"id": "cus_4", "name": "tea-acme", "livemode": False},
        ])
        self.assertEqual([], renames)
        self.assertEqual(4, counts["not_bex"])
        self.assertEqual(0, counts["seen"])

    def test_retired_and_deleted_customers_are_skipped(self):
        renames, counts = self.plan([
            customer("cus_1", bex_deleted_at="2026-11-01T00:00:00Z"),
            dict(customer("cus_2"), deleted=True),
        ])
        self.assertEqual([], renames)
        self.assertEqual(2, counts["retired"])

    def test_workspace_gone_is_counted_and_skipped(self):
        renames, counts = self.plan([customer("cus_1", workspace="tea-gone")])
        self.assertEqual([], renames)
        self.assertEqual(1, counts["workspace_gone"])

    def test_any_other_name_is_left_alone(self):
        renames, counts = self.plan([
            customer("cus_1", name=ACME),                 # already backfilled
            customer("cus_2", name="Member Corp GmbH"),   # edited in the portal
            customer("cus_3", name=""),                   # no name at all
            customer("cus_4", bex_company_name="tea-acme"),  # a member's company name
        ])
        self.assertEqual([], renames)
        self.assertEqual(4, counts["already_named"])

    def test_label_equal_to_id_is_skipped(self):
        renames, counts = self.plan([customer("cus_1", workspace="tea-signup"),
                                     customer("cus_2", workspace="tea-blank")],
                                    labels={**LABELS, "tea-blank": ""})
        self.assertEqual([], renames)
        self.assertEqual(2, counts["label_is_id"])

    def test_only_customers_of_this_mode_are_considered(self):
        live = customer("cus_live", livemode=True)
        test = customer("cus_test", livemode=False)
        renames, counts = self.plan([live, test], live=True)
        self.assertEqual([NAMES.Rename("cus_live", "tea-acme", ACME)], renames)
        self.assertEqual(1, counts["other_mode"])
        renames, _ = self.plan([live, test], live=False)
        self.assertEqual(["cus_test"], [r.customer_id for r in renames])

    def test_counts_partition_every_listed_customer(self):
        renames, counts = self.plan([
            customer("cus_1"), customer("cus_2", workspace="tea-beta"),
            customer("cus_3", workspace=""), customer("cus_4", name="Other"),
            customer("cus_5", workspace="tea-gone"), customer("cus_6", bex_deleted_at="x"),
        ])
        self.assertEqual([NAMES.Rename("cus_1", "tea-acme", ACME),
                          NAMES.Rename("cus_2", "tea-beta", "Beta's Workspace")], renames)
        self.assertEqual(6, counts["listed"])
        self.assertEqual(5, counts["seen"])
        self.assertEqual(counts["seen"], sum(counts[k] for k in NAMES.SEEN))


class ReadTest(TestCase):
    def test_pagination_follows_has_more_and_starting_after(self):
        fetched = []
        pages = [
            {"data": [{"id": "cus_1"}, {"id": "cus_2"}], "has_more": True},
            {"data": [{"id": "cus_3"}], "has_more": True},
            {"data": [{"id": "cus_4"}], "has_more": False},
        ]

        def fetch(after):
            fetched.append(after)
            return pages[len(fetched) - 1]

        self.assertEqual(["cus_1", "cus_2", "cus_3", "cus_4"],
                         [c["id"] for c in NAMES.list_customers(fetch)])
        self.assertEqual([None, "cus_2", "cus_3"], fetched)

    def test_has_more_without_data_is_an_error(self):
        with self.assertRaisesRegex(RuntimeError, "has_more"):
            NAMES.list_customers(lambda after: {"data": [], "has_more": True})

    def test_main_lists_pages_through_the_cli_arguments(self):
        fake = FakeStripe([customer(f"cus_{i}", name="Other") for i in range(5)], page_size=2)
        code, _, _ = run([], fake)
        self.assertEqual(0, code)
        self.assertEqual([
            ("get", "/v1/customers", "-d", "limit=100"),
            ("get", "/v1/customers", "-d", "limit=100", "-d", "starting_after=cus_1"),
            ("get", "/v1/customers", "-d", "limit=100", "-d", "starting_after=cus_3"),
        ], fake.calls)

    def test_parse_labels_keeps_separators_inside_names(self):
        # json_agg puts a newline between elements; names keep | and tabs.
        output = ('[{"id" : "tea-acme", "label" : "Acme | Co\\tLtd = 1"}, \n'
                  ' {"id" : "tea-x", "label" : "Line\\nTwo"}]\n')
        self.assertEqual({"tea-acme": ACME, "tea-x": "Line\nTwo"}, NAMES.parse_labels(output))
        self.assertEqual({}, NAMES.parse_labels("[]\n"))

    def test_parse_labels_error_never_echoes_the_output(self):
        with self.assertRaises(RuntimeError) as ctx:
            NAMES.parse_labels("Secret Corp|tea-x\n")
        self.assertNotIn("Secret Corp", str(ctx.exception))

    def test_run_sql_direct_uri(self):
        fake = mock.Mock(return_value=mock.Mock(returncode=0, stdout="[]\n", stderr=""))
        self.assertEqual("[]\n", NAMES.run_sql("SELECT 1", env={"BEX_CP_DB_URI": "postgres://x"}, run=fake))
        self.assertEqual(["psql", "postgres://x", "-v", "ON_ERROR_STOP=1", "-qAt"], fake.call_args.args[0])
        self.assertEqual("SELECT 1", fake.call_args.kwargs["input"])

    def test_run_sql_execs_into_the_cnpg_primary_by_default(self):
        fake = mock.Mock(side_effect=[mock.Mock(returncode=0, stdout="bex-db-2", stderr=""),
                                      mock.Mock(returncode=0, stdout="[]", stderr="")])
        NAMES.run_sql("SELECT 1", env={}, run=fake)
        self.assertEqual(["kubectl", "-n", "bex-system", "get", "pod",
                          "-l", "cnpg.io/cluster=bex-db,cnpg.io/instanceRole=primary",
                          "-o", "jsonpath={.items[0].metadata.name}"], fake.call_args_list[0].args[0])
        self.assertEqual(["kubectl", "-n", "bex-system", "exec", "-i", "bex-db-2", "--",
                          "psql", "-U", "postgres", "-d", "bex", "-v", "ON_ERROR_STOP=1", "-qAt"],
                         fake.call_args_list[1].args[0])

    def test_run_sql_without_a_primary_fails(self):
        fake = mock.Mock(return_value=mock.Mock(returncode=0, stdout="", stderr=""))
        with self.assertRaisesRegex(RuntimeError, "no CNPG primary for cluster db2 in namespace ns"):
            NAMES.run_sql("SELECT 1", env={"BEX_SYSTEM_NAMESPACE": "ns", "BEX_CP_DB_CLUSTER": "db2"}, run=fake)

    def test_a_failed_read_exits_2_before_any_stripe_call(self):
        fake = FakeStripe([customer("cus_1")])

        def broken():
            raise RuntimeError("control-plane query failed")

        err = io.StringIO()
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(err):
            code = NAMES.main([], stripe_fn=fake, read_labels_fn=broken)
        self.assertEqual(2, code)
        self.assertEqual([], fake.calls)
        self.assertIn("control-plane query failed", err.getvalue())


class RunTest(TestCase):
    CUSTOMERS = [
        customer("cus_1"),
        customer("cus_2", workspace="tea-beta"),
        customer("cus_3", workspace="", name="tea-acme"),
        customer("cus_4", name="Other"),
    ]

    def test_dry_run_issues_no_writes(self):
        fake = FakeStripe(self.CUSTOMERS)
        code, out, _ = run([], fake)
        self.assertEqual(0, code)
        self.assertEqual([], fake.writes())
        self.assertIn("  cus_1 tea-acme\n", out)
        self.assertIn("  cus_2 tea-beta\n", out)
        self.assertIn("Dry run: nothing written", out)
        self.assertEqual("tea-acme", fake.customers["cus_1"]["name"])

    def test_apply_writes_exactly_one_name_per_planned_customer(self):
        fake = FakeStripe(self.CUSTOMERS)
        code, out, _ = run(["--apply"], fake)
        self.assertEqual(0, code)
        self.assertEqual([
            ("post", "/v1/customers/cus_1", "-d", f"name={ACME}"),
            ("post", "/v1/customers/cus_2", "-d", "name=Beta's Workspace"),
        ], fake.writes())
        self.assertIn("renamed 2, skipped 0 (changed since listed), failed 0", out)
        # A re-run finds nothing left to rename.
        code, _, _ = run(["--apply"], fake)
        self.assertEqual(0, code)
        self.assertEqual(2, len(fake.writes()))

    def test_apply_skips_a_customer_changed_since_listing(self):
        fake = FakeStripe([customer("cus_1")])
        real = fake.__call__

        def portal_edit_after_listing(*args):
            if args == ("get", "/v1/customers/cus_1"):
                fake.customers["cus_1"]["name"] = "Edited In Portal"
            return real(*args)

        code, out, _ = run(["--apply"], portal_edit_after_listing)
        self.assertEqual(0, code)
        self.assertEqual([], fake.writes())
        self.assertIn("skipped cus_1 tea-acme: changed since it was listed", out)

    def test_names_are_printed_only_with_show_names(self):
        for argv in ([], ["--apply"], ["--live"], ["--live", "--apply"]):
            with self.subTest(argv=argv):
                live = "--live" in argv
                fake = FakeStripe([dict(c, livemode=live) for c in self.CUSTOMERS], fail={"cus_2"})
                _, out, err = run(argv, fake)
                for name in ("Acme", "Beta"):
                    self.assertNotIn(name, out + err)
        _, out, _ = run(["--show-names"], FakeStripe(self.CUSTOMERS))
        self.assertIn(f"cus_1 tea-acme -> {json.dumps(ACME)}", out)

    def test_a_failed_write_makes_the_exit_code_non_zero(self):
        fake = FakeStripe(self.CUSTOMERS, fail={"cus_1"})
        code, out, err = run(["--apply"], fake)
        self.assertEqual(1, code)
        self.assertIn("FAILED cus_1 tea-acme: stripe post /v1/customers/cus_1", err)
        self.assertIn("Invalid name: <name>", err)
        # The other Customer is still renamed.
        self.assertEqual("Beta's Workspace", fake.customers["cus_2"]["name"])
        self.assertIn("renamed 1, skipped 0 (changed since listed), failed 1", out)

    def test_a_write_stripe_does_not_echo_counts_as_failed(self):
        fake = FakeStripe([customer("cus_1")])
        real = fake.__call__

        def ignores_the_name(*args):
            result = real(*args)
            return dict(result, name="tea-acme") if args[0] == "post" else result

        code, _, err = run(["--apply"], ignores_the_name)
        self.assertEqual(1, code)
        self.assertIn("did not return the new name", err)


class StripeHelperTest(TestCase):
    def test_live_appends_the_global_flag_and_errors_omit_request_data(self):
        result = mock.Mock(returncode=0, stdout='{"error":{"message":"denied"}}', stderr="")
        with mock.patch.object(NAMES, "STRIPE_GLOBAL_ARGS", ["--live"]), \
                mock.patch.object(NAMES.subprocess, "run", return_value=result) as run_:
            with self.assertRaises(RuntimeError) as ctx:
                NAMES.stripe("post", "/v1/customers/cus_1", "-d", f"name={ACME}")
        self.assertEqual(["stripe", "post", "/v1/customers/cus_1", "-d", f"name={ACME}", "--live"],
                         run_.call_args.args[0])
        self.assertIn("denied", str(ctx.exception))
        self.assertNotIn("Acme", str(ctx.exception))

    def test_main_selects_live_mode_only_with_live(self):
        run(["--live"], FakeStripe([]))
        self.assertEqual(["--live"], NAMES.STRIPE_GLOBAL_ARGS)
        run([], FakeStripe([]))
        self.assertEqual([], NAMES.STRIPE_GLOBAL_ARGS)


if __name__ == "__main__":
    main()
