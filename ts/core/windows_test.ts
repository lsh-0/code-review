import { assert, assertEquals } from "@std/assert";
import {
  defaultWindows,
  diffContextSize,
  growDown,
  growUp,
  growWindow,
  hunkReachedEOF,
  hunkSpan,
  missingRanges,
  normaliseWindows,
  type Window,
  windowMergeThreshold,
  windowPlan,
  windowsContain,
} from "./windows.ts";
import {
  type DiffHunk,
  type DiffLine,
  LineAdded,
  LineContext,
  LineRemoved,
  type LineType,
} from "./types.ts";

function line(type: LineType, newNo: number, oldNo = newNo): DiffLine {
  return {
    Type: type,
    Content: `l${newNo}`,
    OldLineNo: oldNo,
    NewLineNo: newNo,
  };
}

// a hunk spanning new lines [start, start+count-1]; old side offset by
// `oldShift` (old = new + oldShift). Lines are plain context unless stated —
// the window arithmetic only reads the header fields.
function hunkAt(start: number, count: number, oldShift = 0): DiffHunk {
  return {
    OldStart: start + oldShift,
    OldLines: count,
    NewStart: start,
    NewLines: count,
    Lines: Array.from(
      { length: count },
      (_, i) => line(LineContext, start + i, start + i + oldShift),
    ),
  };
}

// --- defaults ---

Deno.test("defaultWindows: hunk-only file yields one window per hunk", () => {
  const hunks = [hunkAt(5, 4), hunkAt(30, 6)];
  const actual = defaultWindows(hunks, [], 100);
  assertEquals(actual, [{ start: 5, end: 8 }, { start: 30, end: 35 }]);
});

Deno.test("defaultWindows: a comment mid-gap becomes its own island", () => {
  const hunks = [hunkAt(1, 4), hunkAt(50, 4)];
  const actual = defaultWindows(hunks, [20], 100);
  assertEquals(actual, [
    { start: 1, end: 4 },
    { start: 20 - diffContextSize, end: 20 + diffContextSize },
    { start: 50, end: 53 },
  ]);
});

Deno.test("defaultWindows: an island near a hunk merges into it at creation", () => {
  // comment at line 9; island [6..12] sits within the merge threshold of the
  // hunk window [1..4] (gap of 1 hidden line), so they join.
  const hunks = [hunkAt(1, 4)];
  const actual = defaultWindows(hunks, [9], 100);
  assertEquals(actual, [{ start: 1, end: 12 }]);
});

Deno.test("defaultWindows: a comment above the first hunk gets a clamped island", () => {
  const hunks = [hunkAt(50, 4)];
  const actual = defaultWindows(hunks, [2], 100);
  assertEquals(actual[0], { start: 1, end: 2 + diffContextSize });
});

Deno.test("defaultWindows: a comment below the last hunk gets an island clamped to EOF", () => {
  const hunks = [hunkAt(1, 4)];
  const actual = defaultWindows(hunks, [99], 100);
  assertEquals(actual[actual.length - 1], { start: 96, end: 100 });
});

Deno.test("defaultWindows: several comments in one gap merge into one island", () => {
  const hunks = [hunkAt(1, 4), hunkAt(60, 4)];
  const actual = defaultWindows(hunks, [20, 24, 28], 100);
  assertEquals(actual, [
    { start: 1, end: 4 },
    { start: 17, end: 31 },
    { start: 60, end: 63 },
  ]);
});

Deno.test("defaultWindows: a comment inside a hunk window adds nothing", () => {
  const hunks = [hunkAt(5, 6)];
  const actual = defaultWindows(hunks, [7], 100);
  assertEquals(actual, [{ start: 5, end: 10 }]);
});

Deno.test("defaultWindows: line-0 comments (top-level) produce no window", () => {
  const actual = defaultWindows([hunkAt(5, 4)], [0], 100);
  assertEquals(actual, [{ start: 5, end: 8 }]);
});

Deno.test("defaultWindows: an island beyond the end of the file drops when the total is known", () => {
  // comment at line 200 of a 100-line file: nothing to show, no window. The
  // comment renders untethered instead.
  const actual = defaultWindows([hunkAt(1, 4)], [200], 100);
  assertEquals(actual, [{ start: 1, end: 4 }]);
});

// --- growth ---

Deno.test("growWindow: a partial step extends the edge", () => {
  const windows: Window[] = [{ start: 40, end: 50 }];
  const actual = growWindow(windows, 0, growUp, 10, 100);
  assertEquals(actual, [{ start: 30, end: 50 }]);
});

Deno.test("growWindow: growth clamps at the top of the file", () => {
  const windows: Window[] = [{ start: 5, end: 10 }];
  const actual = growWindow(windows, 0, growUp, 20, 100);
  assertEquals(actual, [{ start: 1, end: 10 }]);
});

Deno.test("growWindow: growth clamps at the end of the file", () => {
  const windows: Window[] = [{ start: 90, end: 95 }];
  const actual = growWindow(windows, 0, growDown, 20, 100);
  assertEquals(actual, [{ start: 90, end: 100 }]);
});

Deno.test("growWindow: converging growth merges two windows", () => {
  const windows: Window[] = [{ start: 1, end: 10 }, { start: 25, end: 30 }];
  const actual = growWindow(windows, 0, growDown, 20, 100);
  assertEquals(actual, [{ start: 1, end: 30 }]);
});

Deno.test("growWindow: growth leaving a sub-threshold sliver merges too", () => {
  // growing to end 22 leaves a 2-line gap (23, 24) below the threshold: merged.
  const windows: Window[] = [{ start: 1, end: 10 }, { start: 25, end: 30 }];
  const actual = growWindow(windows, 0, growDown, 12, 100);
  assertEquals(actual, [{ start: 1, end: 30 }]);
});

Deno.test("growWindow: growth stopping a full threshold short stays separate", () => {
  const windows: Window[] = [{ start: 1, end: 10 }, { start: 25, end: 30 }];
  const actual = growWindow(windows, 0, growDown, 11, 100);
  assertEquals(actual, [{ start: 1, end: 21 }, { start: 25, end: 30 }]);
});

Deno.test("growWindow: unknown total (0) applies no upper clamp", () => {
  const windows: Window[] = [{ start: 1, end: 10 }];
  const actual = growWindow(windows, 0, growDown, 20, 0);
  assertEquals(actual, [{ start: 1, end: 30 }]);
});

// --- normalisation ---

Deno.test("normaliseWindows: sorts, merges overlaps, drops empties", () => {
  const given: Window[] = [
    { start: 40, end: 45 },
    { start: 10, end: 20 },
    { start: 18, end: 25 },
    { start: 8, end: 3 },
  ];
  const actual = normaliseWindows(given, 100);
  assertEquals(actual, [{ start: 10, end: 25 }, { start: 40, end: 45 }]);
});

Deno.test("normaliseWindows: keeps gaps of at least the merge threshold", () => {
  const given: Window[] = [
    { start: 1, end: 10 },
    { start: 10 + windowMergeThreshold + 1, end: 20 },
  ];
  assertEquals(normaliseWindows(given, 100), given);
});

Deno.test("windowsContain: inclusive bounds", () => {
  const windows: Window[] = [{ start: 5, end: 8 }];
  assert(windowsContain(windows, 5));
  assert(windowsContain(windows, 8));
  assert(!windowsContain(windows, 4));
  assert(!windowsContain(windows, 9));
});

// --- render plan ---

Deno.test("windowPlan: a hunk-only window is a single hunk segment", () => {
  const hunks = [hunkAt(5, 4)];
  const actual = windowPlan({ start: 5, end: 8 }, hunks);
  assertEquals(actual, [{ kind: "hunk", hunkIndex: 0 }]);
});

Deno.test("windowPlan: a grown window wraps its hunk in context runs", () => {
  // hunk at [10..13] with the old side 5 ahead; window grown to [5..20].
  const hunks = [hunkAt(10, 4, 5)];
  const actual = windowPlan({ start: 5, end: 20 }, hunks);
  assertEquals(actual, [
    { kind: "context", start: 5, end: 9, oldOffset: 5 },
    { kind: "hunk", hunkIndex: 0 },
    { kind: "context", start: 14, end: 20, oldOffset: 5 },
  ]);
});

Deno.test("windowPlan: a merged window spans two hunks with the gap as context", () => {
  // an insertion of 2 lines in the first hunk shifts old numbering by -2 after it.
  const first: DiffHunk = {
    OldStart: 5,
    OldLines: 2,
    NewStart: 5,
    NewLines: 4,
    Lines: [
      line(LineContext, 5),
      line(LineAdded, 6, 0),
      line(LineAdded, 7, 0),
      line(LineContext, 8, 6),
    ],
  };
  const second = hunkAt(20, 4, -2);
  const actual = windowPlan({ start: 5, end: 23 }, [first, second]);
  assertEquals(actual, [
    { kind: "hunk", hunkIndex: 0 },
    { kind: "context", start: 9, end: 19, oldOffset: -2 },
    { kind: "hunk", hunkIndex: 1 },
  ]);
});

Deno.test("windowPlan: an island window with no hunk is one context run", () => {
  const hunks = [hunkAt(1, 4, 0), hunkAt(60, 4, 7)];
  // island between the hunks: old numbering follows the hunk above (offset
  // after hunk 0 = 0 here, since hunkAt keeps both sides equal).
  const actual = windowPlan({ start: 20, end: 26 }, hunks);
  assertEquals(actual, [
    { kind: "context", start: 20, end: 26, oldOffset: 0 },
  ]);
});

Deno.test("windowPlan: an island above every hunk uses the first hunk's before-offset", () => {
  const hunks = [hunkAt(50, 4, 3)];
  const actual = windowPlan({ start: 10, end: 16 }, hunks);
  assertEquals(actual, [
    { kind: "context", start: 10, end: 16, oldOffset: 3 },
  ]);
});

Deno.test("missingRanges: reports only unfetched context lines, in runs", () => {
  const segments = windowPlan({ start: 5, end: 20 }, [hunkAt(10, 4)]);
  const fetched = new Set([5, 6, 15]);
  const actual = missingRanges(segments, (n) => fetched.has(n));
  assertEquals(actual, [
    { start: 7, end: 9, oldOffset: 0 },
    { start: 14, end: 14, oldOffset: 0 },
    { start: 16, end: 20, oldOffset: 0 },
  ]);
});

Deno.test("missingRanges: empty when everything is fetched or hunk-covered", () => {
  const segments = windowPlan({ start: 10, end: 13 }, [hunkAt(10, 4)]);
  assertEquals(missingRanges(segments, () => false), []);
});

// --- hunkSpan edge ---

Deno.test("hunkSpan: a no-context pure deletion still gets a one-line span", () => {
  const hunk: DiffHunk = {
    OldStart: 5,
    OldLines: 2,
    NewStart: 4,
    NewLines: 0,
    Lines: [line(LineRemoved, 0, 5), line(LineRemoved, 0, 6)],
  };
  assertEquals(hunkSpan(hunk), { start: 4, end: 4 });
});

// --- hunkReachedEOF (ported from the expand-state tests) ---

Deno.test("hunkReachedEOF: short trailing context run means EOF", () => {
  const given = [
    line(LineAdded, 1),
    line(LineContext, 2),
    line(LineContext, 3),
  ];
  assertEquals(hunkReachedEOF(given), true);
});

Deno.test("hunkReachedEOF: full trailing context run means more below", () => {
  const given = [
    line(LineAdded, 1),
    line(LineContext, 2),
    line(LineContext, 3),
    line(LineContext, 4),
  ];
  assertEquals(hunkReachedEOF(given), false);
});

Deno.test("hunkReachedEOF: a hunk ending in a removed line makes no EOF claim", () => {
  const given = [line(LineContext, 1), line(LineRemoved, 0, 2)];
  assertEquals(hunkReachedEOF(given), false);
});

Deno.test("hunkReachedEOF: empty hunk makes no EOF claim", () => {
  assertEquals(hunkReachedEOF([]), false);
});

// --- property tests ---

// a small deterministic PRNG (LCG), so the property runs are reproducible.
function lcg(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0;
    return s / 0x100000000;
  };
}

function randomInt(rand: () => number, lo: number, hi: number): number {
  return lo + Math.floor(rand() * (hi - lo + 1));
}

// the window-list invariants every operation must preserve: sorted by start,
// non-empty, disjoint, with hidden gaps of at least the merge threshold.
function assertInvariants(windows: Window[], totalLines: number, ctx: string) {
  for (let i = 0; i < windows.length; i++) {
    const w = windows[i];
    assert(w.start >= 1, `${ctx}: window ${i} starts below 1`);
    assert(w.end >= w.start, `${ctx}: window ${i} is empty`);
    if (totalLines > 0) {
      assert(w.end <= totalLines, `${ctx}: window ${i} passes EOF`);
    }
    if (i > 0) {
      const gap = w.start - windows[i - 1].end - 1;
      assert(
        gap >= windowMergeThreshold,
        `${ctx}: gap of ${gap} below threshold between windows ${
          i - 1
        } and ${i}`,
      );
    }
  }
}

Deno.test("property: defaults and random growth preserve the invariants; growth never shrinks", () => {
  const rand = lcg(20260729);
  for (let run = 0; run < 200; run++) {
    const totalLines = randomInt(rand, 10, 400);

    // random non-overlapping hunks.
    const hunks: DiffHunk[] = [];
    let cursor = 1;
    while (cursor < totalLines - 5 && hunks.length < 5 && rand() < 0.8) {
      const start = cursor + randomInt(rand, 0, 30);
      const count = randomInt(rand, 1, 8);
      if (start + count - 1 > totalLines) {
        break;
      }
      hunks.push(hunkAt(start, count));
      cursor = start + count + randomInt(rand, 1, 20);
    }

    const commentLines = Array.from(
      { length: randomInt(rand, 0, 4) },
      () => randomInt(rand, 1, totalLines + 10),
    );

    let windows = defaultWindows(hunks, commentLines, totalLines);
    assertInvariants(windows, totalLines, `run ${run} defaults`);

    // every hunk is wholly inside exactly one window.
    for (const hunk of hunks) {
      const span = hunkSpan(hunk);
      assert(
        windows.some((w) => span.start >= w.start && span.end <= w.end),
        `run ${run}: hunk span not contained`,
      );
    }

    // random growth sequence: invariants hold and total coverage never shrinks.
    for (let g = 0; g < 10 && windows.length > 0; g++) {
      const covered = windows.reduce((n, w) => n + (w.end - w.start + 1), 0);
      const index = randomInt(rand, 0, windows.length - 1);
      const direction = rand() < 0.5 ? growUp : growDown;
      const step = randomInt(rand, 1, 40);
      windows = growWindow(windows, index, direction, step, totalLines);
      assertInvariants(windows, totalLines, `run ${run} grow ${g}`);
      const after = windows.reduce((n, w) => n + (w.end - w.start + 1), 0);
      assert(after >= covered, `run ${run} grow ${g}: coverage shrank`);
    }
  }
});

Deno.test("property: normaliseWindows is idempotent", () => {
  const rand = lcg(1337);
  for (let run = 0; run < 200; run++) {
    const totalLines = randomInt(rand, 5, 300);
    const raw: Window[] = Array.from(
      { length: randomInt(rand, 0, 8) },
      () => {
        const start = randomInt(rand, -5, totalLines + 5);
        return { start, end: start + randomInt(rand, -2, 40) };
      },
    );
    const once = normaliseWindows(raw, totalLines);
    const twice = normaliseWindows(once, totalLines);
    assertEquals(twice, once, `run ${run}: normalisation not idempotent`);
    assertInvariants(once, totalLines, `run ${run} normalised`);
  }
});
