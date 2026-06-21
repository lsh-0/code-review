import { assertEquals } from "@std/assert";
import {
  advanceState,
  diffContextSize,
  effectiveBoundary,
  expandDown,
  type ExpandState,
  expandStep,
  expandUp,
  gapExhausted,
  hunkReachedEOF,
  stepRange,
} from "./expand.ts";
import { type DiffLine, LineAdded, LineContext, LineRemoved } from "./types.ts";

function ctx(newLineNo: number): DiffLine {
  return {
    Type: LineContext,
    Content: "",
    OldLineNo: newLineNo,
    NewLineNo: newLineNo,
  };
}

function added(newLineNo: number): DiffLine {
  return { Type: LineAdded, Content: "", OldLineNo: 0, NewLineNo: newLineNo };
}

Deno.test("effectiveBoundary: unlinked uses fixed boundary", () => {
  const given: ExpandState = { frontier: 50, boundary: 10 };
  const expected = 10;
  const actual = effectiveBoundary(given);
  assertEquals(actual, expected);
});

Deno.test("effectiveBoundary: linked uses the sibling's frontier", () => {
  const sibling: ExpandState = { frontier: 30, boundary: 0 };
  const given: ExpandState = { frontier: 50, boundary: 0, sibling };
  const expected = 30;
  const actual = effectiveBoundary(given);
  assertEquals(actual, expected);
});

Deno.test("stepRange up: clamps the start to the boundary", () => {
  // frontier 50, boundary 45: a full step would start at 31, clamp to 45.
  const given: ExpandState = { frontier: 50, boundary: 45 };
  const expected = { startNew: 45, endNew: 50 };
  const actual = stepRange(expandUp, given);
  assertEquals(actual, expected);
});

Deno.test("stepRange up: full step when the gap is large", () => {
  const given: ExpandState = { frontier: 100, boundary: 1 };
  const expected = { startNew: 100 - expandStep + 1, endNew: 100 };
  const actual = stepRange(expandUp, given);
  assertEquals(actual, expected);
});

Deno.test("stepRange down: clamps the end to a known boundary", () => {
  const given: ExpandState = { frontier: 10, boundary: 15 };
  const expected = { startNew: 10, endNew: 15 };
  const actual = stepRange(expandDown, given);
  assertEquals(actual, expected);
});

Deno.test("stepRange down: unbounded gap takes a full step", () => {
  const given: ExpandState = { frontier: 10, boundary: 0 };
  const expected = { startNew: 10, endNew: 10 + expandStep - 1 };
  const actual = stepRange(expandDown, given);
  assertEquals(actual, expected);
});

Deno.test("stepRange down: clamps to the sibling frontier when linked", () => {
  // two converging affordances: the downward one stops at the upward's frontier.
  const sibling: ExpandState = { frontier: 14, boundary: 0 };
  const given: ExpandState = { frontier: 10, boundary: 0, sibling };
  const expected = { startNew: 10, endNew: 14 };
  const actual = stepRange(expandDown, given);
  assertEquals(actual, expected);
});

Deno.test("advanceState up: frontier moves down by revealed count", () => {
  const given: ExpandState = { frontier: 50, boundary: 10 };
  const expected = 45;
  const actual = advanceState(expandUp, given, 5, 0).frontier;
  assertEquals(actual, expected);
});

Deno.test("advanceState down: learns the boundary from total on first fetch", () => {
  const given: ExpandState = { frontier: 10, boundary: 0 };
  const actual = advanceState(expandDown, given, 5, 200);
  assertEquals(actual.frontier, 15);
  assertEquals(actual.boundary, 200);
});

Deno.test("advanceState down: keeps a known boundary", () => {
  const given: ExpandState = { frontier: 10, boundary: 42 };
  const actual = advanceState(expandDown, given, 5, 200);
  assertEquals(actual.boundary, 42);
});

Deno.test("advanceState does not mutate its input", () => {
  const given: ExpandState = { frontier: 50, boundary: 10 };
  advanceState(expandUp, given, 5, 0);
  assertEquals(given.frontier, 50);
});

Deno.test("gapExhausted up: true once the frontier crosses below the boundary", () => {
  assertEquals(gapExhausted(expandUp, { frontier: 9, boundary: 10 }), true);
  assertEquals(gapExhausted(expandUp, { frontier: 10, boundary: 10 }), false);
});

Deno.test("gapExhausted down: true once the frontier passes the boundary", () => {
  assertEquals(gapExhausted(expandDown, { frontier: 16, boundary: 15 }), true);
  assertEquals(gapExhausted(expandDown, { frontier: 15, boundary: 15 }), false);
});

Deno.test("gapExhausted down: never exhausted while the boundary is unknown", () => {
  const given: ExpandState = { frontier: 999, boundary: 0 };
  assertEquals(gapExhausted(expandDown, given), false);
});

Deno.test("gapExhausted: linked gap closes when the two frontiers cross", () => {
  // downward frontier 16 has passed the upward sibling's frontier 15.
  const sibling: ExpandState = { frontier: 15, boundary: 0 };
  const given: ExpandState = { frontier: 16, boundary: 0, sibling };
  assertEquals(gapExhausted(expandDown, given), true);
});

Deno.test("hunkReachedEOF: short trailing context run means EOF", () => {
  // fewer than diffContextSize trailing context lines: file ended.
  const given = [ctx(1), added(2), ctx(3)];
  assertEquals(given.length >= diffContextSize, true);
  const actual = hunkReachedEOF(given);
  assertEquals(actual, true);
});

Deno.test("hunkReachedEOF: full trailing context run means more below", () => {
  const given = [added(1), ctx(2), ctx(3), ctx(4)];
  const actual = hunkReachedEOF(given);
  assertEquals(actual, false);
});

Deno.test("hunkReachedEOF: a hunk ending in a removed line makes no EOF claim", () => {
  const given: DiffLine[] = [
    ctx(1),
    { Type: LineRemoved, Content: "", OldLineNo: 2, NewLineNo: 0 },
  ];
  const actual = hunkReachedEOF(given);
  assertEquals(actual, false);
});

Deno.test("hunkReachedEOF: empty hunk makes no EOF claim", () => {
  assertEquals(hunkReachedEOF([]), false);
});
