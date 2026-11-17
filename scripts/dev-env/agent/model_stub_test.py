#!/usr/bin/env python3
"""Tests for the dev-N model stub (model-stub.py). Run: python3 -B model_stub_test.py"""

import http.client
import http.server
import importlib.util
import json
import pathlib
import threading
import time
import unittest

_spec = importlib.util.spec_from_file_location(
    "model_stub", pathlib.Path(__file__).with_name("model-stub.py")
)
stub = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(stub)


def user(text):
    return {"role": "user", "content": text}


class HoldDirective(unittest.TestCase):
    def test_no_directive_is_not_held(self):
        self.assertEqual(stub.hold_seconds({"messages": [user("say hi")]}), 0)
        self.assertEqual(stub.hold_seconds({}), 0)
        self.assertEqual(stub.hold_seconds({"messages": ["not a message"]}), 0)

    def test_string_and_block_content(self):
        self.assertEqual(stub.hold_seconds({"messages": [user("stub-hold:45 please")]}), 45)
        blocks = [
            {"type": "text", "text": "context"},
            {"type": "image", "source": {}},
            {"type": "text", "text": "wait stub-hold:7"},
        ]
        self.assertEqual(stub.hold_seconds({"messages": [user(blocks)]}), 7)

    def test_only_the_newest_user_message_counts(self):
        messages = [
            user("stub-hold:60"),
            {"role": "assistant", "content": "tick 1"},
            user("a follow-up with no directive"),
        ]
        self.assertEqual(stub.hold_seconds({"messages": messages}), 0)
        messages.append({"role": "assistant", "content": "stub-hold:99"})
        self.assertEqual(stub.hold_seconds({"messages": messages}), 0)

    def test_a_directive_in_the_replayed_conversation_does_not_count(self):
        # The driver's priming of a fresh process (continuity.ts renderPreamble).
        primed = (
            "<session-context>\nYou are continuing an existing session in a fresh process. "
            "Earlier conversation (possibly truncated; oldest first):\n\n"
            "[turn 1] user:\ncount slowly. stub-hold:300\n\n"
            "[turn 1] assistant:\ntick 1\ntick 2\n"
            "</session-context>\n\nContinue from that context. "
            "The user's next message follows.\n\n"
        )
        self.assertEqual(stub.hold_seconds({"messages": [user(primed + "third queued")]}), 0)
        self.assertEqual(
            stub.hold_seconds({"messages": [user(primed + "again. stub-hold:12")]}), 12
        )

    def test_the_hold_is_capped(self):
        self.assertEqual(
            stub.hold_seconds({"messages": [user("stub-hold:999999")]}),
            stub.MAX_HOLD_SECONDS,
        )


def bash_result(output):
    return {"role": "user", "content": [
        {"type": "tool_result", "tool_use_id": "toolu_devstub1", "content": output}]}


BASH = [{"name": "Read"}, {"name": "Bash"}]


class RunDirective(unittest.TestCase):
    def test_the_rest_of_the_line_is_the_command(self):
        body = {"tools": BASH, "messages": [user("stub-run:echo kept > kept.txt \nstub-hold:300")]}
        self.assertEqual(stub.run_command(body), "echo kept > kept.txt")
        self.assertEqual(stub.hold_seconds(body), 300)

    def test_no_directive_or_no_shell_tool_runs_nothing(self):
        self.assertEqual(stub.run_command({"tools": BASH, "messages": [user("say hi")]}), "")
        self.assertEqual(stub.run_command({"messages": [user("stub-run:id")]}), "")
        self.assertEqual(
            stub.run_command({"tools": [{"name": "Read"}], "messages": [user("stub-run:id")]}), "")

    def test_a_command_runs_once_and_its_output_is_the_reply(self):
        messages = [user("stub-run:cat kept.txt\nstub-hold:5")]
        self.assertIsNone(stub.tool_output({"tools": BASH, "messages": messages}))
        messages += [{"role": "assistant", "content": [{"type": "tool_use"}]}, bash_result("kept\n")]
        body = {"tools": BASH, "messages": messages}
        self.assertEqual(stub.run_command(body), "")
        self.assertEqual(stub.tool_output(body), "kept\n")
        # The hold still comes from the prompt, not from the tool's result.
        self.assertEqual(stub.hold_seconds(body), 5)
        blocks = bash_result([{"type": "text", "text": "a"}, {"type": "text", "text": "b"}])
        self.assertEqual(stub.tool_output({"messages": [user("x"), blocks]}), "a\nb")

    def test_a_command_in_the_replayed_conversation_does_not_run(self):
        primed = ("<session-context>\n[turn 1] user:\nstub-run:rm -rf kept.txt\n"
                  "</session-context>\n\nThe user's next message follows.\n\n")
        self.assertEqual(stub.run_command({"tools": BASH, "messages": [user(primed + "hi")]}), "")
        self.assertEqual(
            stub.run_command({"tools": BASH, "messages": [user(primed + "stub-run:ls")]}), "ls")


class Streaming(unittest.TestCase):
    def setUp(self):
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), stub.Handler)
        self.server.daemon_threads = True
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.tick = stub.TICK_SECONDS
        stub.TICK_SECONDS = 0.05
        self.addCleanup(lambda: setattr(stub, "TICK_SECONDS", self.tick))

    def post(self, body, path="/v1/messages"):
        conn = http.client.HTTPConnection("127.0.0.1", self.server.server_address[1], timeout=10)
        self.addCleanup(conn.close)
        conn.request(
            "POST",
            path,
            json.dumps(body),
            {"Content-Type": "application/json", "x-api-key": "k"},
        )
        return conn.getresponse()

    def text_of(self, response):
        """Read the stream to its end (message_stop) and return the reply text."""
        text = ""
        while True:
            line = response.fp.readline()
            if not line:
                self.fail("the stream ended before message_stop")
            if not line.startswith(b"data: "):
                continue
            event = json.loads(line[len(b"data: "):])
            if event["type"] == "content_block_delta":
                text += event["delta"]["text"]
            if event["type"] == "message_stop":
                return text

    def events_of(self, response):
        events = []
        for line in response.fp:
            if line.startswith(b"data: "):
                events.append(json.loads(line[len(b"data: "):]))
        return events

    def test_a_run_directive_is_answered_with_one_shell_tool_call(self):
        response = self.post(
            {"stream": True, "tools": BASH, "messages": [user("stub-run:echo kept > kept.txt")]})
        events = self.events_of(response)
        start = next(e for e in events if e["type"] == "content_block_start")
        self.assertEqual(start["content_block"]["type"], "tool_use")
        self.assertEqual(start["content_block"]["name"], "Bash")
        arguments = "".join(
            e["delta"]["partial_json"] for e in events if e["type"] == "content_block_delta")
        self.assertEqual(json.loads(arguments)["command"], "echo kept > kept.txt")
        stop = next(e for e in events if e["type"] == "message_delta")
        self.assertEqual(stop["delta"]["stop_reason"], "tool_use")
        self.assertEqual(events[-1]["type"], "message_stop")

    def test_after_the_command_the_reply_is_its_output_then_the_hold(self):
        messages = [
            user("stub-run:cat kept.txt\nstub-hold:2"),
            {"role": "assistant", "content": [{"type": "tool_use"}]},
            bash_result("kept\n"),
        ]
        response = self.post({"stream": True, "tools": BASH, "messages": messages})
        self.assertEqual(self.text_of(response), "ran:\nkept\ntick 1\ntick 2\ndone\n")
        again = self.post({"stream": True, "tools": BASH, "messages": [
            user("stub-run:cat kept.txt"),
            {"role": "assistant", "content": [{"type": "tool_use"}]},
            bash_result("kept\n"),
        ]})
        self.assertEqual(self.text_of(again), "ran:\nkept\n")

    def test_a_plain_turn_gets_the_fixed_reply_at_once(self):
        started = time.monotonic()
        response = self.post({"stream": True, "messages": [user("say hi")]})
        self.assertEqual(self.text_of(response), stub.REPLY + "\n")
        self.assertLess(time.monotonic() - started, 1)

    def test_the_stream_body_ends_after_message_stop(self):
        # An engine reads the body to its end, not to message_stop: a stream
        # left open keeps the turn running for good.
        response = self.post({"stream": True, "messages": [user("say hi")]})
        self.assertEqual(self.text_of(response), stub.REPLY + "\n")
        response.fp.raw._sock.settimeout(2)
        self.assertEqual(response.fp.read().strip(), b"")

    def test_a_held_turn_ticks_once_per_interval_then_finishes(self):
        started = time.monotonic()
        response = self.post({"stream": True, "messages": [user("stub-hold:4")]})
        self.assertEqual(self.text_of(response), "tick 1\ntick 2\ntick 3\ntick 4\ndone\n")
        self.assertGreaterEqual(time.monotonic() - started, 4 * 0.05)

    def test_the_first_tick_arrives_before_the_hold_ends(self):
        stub.TICK_SECONDS = 5
        response = self.post({"stream": True, "messages": [user("stub-hold:3")]})
        started = time.monotonic()
        while True:
            line = response.fp.readline()
            if b"tick 1" in line:
                break
            self.assertTrue(line, "the stream ended without a tick")
        self.assertLess(time.monotonic() - started, 2)

    def test_a_client_that_leaves_mid_hold_does_not_break_the_stub(self):
        response = self.post({"stream": True, "messages": [user("stub-hold:200")]})
        response.fp.readline()
        response.close()
        time.sleep(0.3)
        again = self.post({"stream": True, "messages": [user("say hi")]})
        self.assertEqual(self.text_of(again), stub.REPLY + "\n")

    def test_a_non_streaming_request_is_never_held(self):
        started = time.monotonic()
        stub.TICK_SECONDS = 5
        response = self.post({"messages": [user("stub-hold:30")]})
        self.assertEqual(json.loads(response.read())["content"][0]["text"], stub.REPLY)
        self.assertLess(time.monotonic() - started, 2)


if __name__ == "__main__":
    unittest.main()
