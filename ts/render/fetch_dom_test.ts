// DOM tests for the fetch control: the button is unavailable while a fetch is in
// flight, a success reloads and reports, and a failure reports without reloading.
// Run with --allow-read --allow-net (deno-dom WASM init).

import { assert, assertEquals } from "@std/assert";
import { setupDom } from "./dom_test_setup.ts";
import { failureMessage, type FetchDeps, triggerFetch } from "./fetch.ts";
import type { ImportResult } from "../client.ts";

function controls(): { button: HTMLButtonElement; input: HTMLInputElement } {
  const doc = setupDom();
  const button = doc.createElement("button") as unknown as HTMLButtonElement;
  const input = doc.createElement("input") as unknown as HTMLInputElement;
  // deno-dom does not implement `value` on an input, so it starts empty here as it
  // does in a browser.
  input.value = "";
  return { button, input };
}

// deps recording what the flow did. `fetch` resolves or rejects as `outcome` says.
function recordingDeps(outcome: () => Promise<ImportResult>) {
  const calls = {
    fetched: [] as string[],
    reloads: 0,
    reports: [] as string[][],
    flashes: 0,
    disabledDuringFetch: [] as boolean[],
  };
  let button: HTMLButtonElement | null = null;
  const deps: FetchDeps = {
    fetch: (url) => {
      calls.fetched.push(url);
      calls.disabledDuringFetch.push(button?.disabled ?? false);
      return outcome();
    },
    reload: () => {
      calls.reloads++;
      return Promise.resolve();
    },
    report: (lines) => calls.reports.push(lines),
    flash: () => calls.flashes++,
  };
  return { deps, calls, bind: (b: HTMLButtonElement) => (button = b) };
}

const success: ImportResult = {
  cancelled: false,
  source: "bb:5405",
  lines: [
    "imported 3 comments from bb:5405 (1 placed, 0 untethered, 1 replies)",
  ],
};

Deno.test("fetch with a blank input asks for the branch's pull request", async () => {
  const { button, input } = controls();
  input.value = "   ";
  const { deps, calls, bind } = recordingDeps(() => Promise.resolve(success));
  bind(button);

  await triggerFetch(button, input, deps);

  assertEquals(calls.fetched, [""]);
});

Deno.test("fetch passes a pasted URL trimmed, then clears it", async () => {
  const { button, input } = controls();
  input.value = " https://bitbucket.org/a/b/pull-requests/1 ";
  const { deps, calls, bind } = recordingDeps(() => Promise.resolve(success));
  bind(button);

  await triggerFetch(button, input, deps);

  assertEquals(calls.fetched, ["https://bitbucket.org/a/b/pull-requests/1"]);
  assertEquals(input.value, "");
});

Deno.test("the button is disabled while the fetch is in flight", async () => {
  const { button, input } = controls();
  const { deps, calls, bind } = recordingDeps(() => Promise.resolve(success));
  bind(button);

  await triggerFetch(button, input, deps);

  assertEquals(calls.disabledDuringFetch, [true]);
  assertEquals(button.disabled, false);
});

Deno.test("a second click during a fetch starts nothing", async () => {
  const { button, input } = controls();
  let settle: (result: ImportResult) => void = () => {};
  const pending = new Promise<ImportResult>((resolve) => (settle = resolve));
  const { deps, calls, bind } = recordingDeps(() => pending);
  bind(button);

  const first = triggerFetch(button, input, deps);
  await triggerFetch(button, input, deps);
  settle(success);
  await first;

  assertEquals(calls.fetched.length, 1);
});

Deno.test("a successful fetch reloads, flashes and reports", async () => {
  const { button, input } = controls();
  const { deps, calls, bind } = recordingDeps(() => Promise.resolve(success));
  bind(button);

  await triggerFetch(button, input, deps);

  assertEquals(calls.reloads, 1);
  assertEquals(calls.flashes, 1);
  assertEquals(calls.reports, [success.lines]);
});

Deno.test("a failed fetch reports the message and does not reload", async () => {
  const { button, input } = controls();
  input.value = "https://bitbucket.org/a/b/pull-requests/1";
  const { deps, calls, bind } = recordingDeps(() =>
    Promise.reject(
      "no such pull request, or it is not visible to this credential.",
    )
  );
  bind(button);

  await triggerFetch(button, input, deps);

  assertEquals(calls.reloads, 0);
  assertEquals(calls.flashes, 0);
  assertEquals(calls.reports, [[
    "Fetch failed: no such pull request, or it is not visible to this credential.",
  ]]);
  assertEquals(input.value, "https://bitbucket.org/a/b/pull-requests/1");
  assert(!button.disabled);
});

Deno.test("failureMessage reads an Error and a string rejection alike", () => {
  assertEquals(
    failureMessage(new Error("bridge not ready")),
    "bridge not ready",
  );
  assertEquals(failureMessage("not visible"), "not visible");
});
