## Context

A binary file cannot be diffed line-by-line, so its diff body is empty. The
`diff-context-expansion` spec already requires binary files to be listed but not
rendered, showing "a plain placeholder (for example 'binary file')". That bare
label does not explain why the body is empty, so an empty pane still reads as a
possible bug.

The implementation already on `master` renders the explanatory text "binary
file, cannot diff":
- `backend/diff_parser.go` detects git's `Binary files ... differ` marker and
  sets `DiffFile.Binary`.
- `ts/render/hunks.ts` renders the `.binary-placeholder` element with that text.
- `ts/render/render_dom_test.ts` asserts the exact string.

This change records the decision in the spec. There is no new runtime capability;
the contract is being tightened to match what the code already does.

## Goals / Non-Goals

**Goals:**
- Tighten the existing "Binary files are listed but not rendered" requirement so
  the placeholder must explain that the file cannot be diffed, not merely label
  it as binary.
- Keep the spec and the implementation in agreement.

**Non-Goals:**
- No change to binary detection, file-list navigation, or the suppression of
  hunks, expansion affordances, and blob fetches — all already specified and
  implemented.
- No attempt to render any preview of binary content (images, size, hex). The
  placeholder is text only.
- No new capability and no breaking change.

## Decisions

**Modify the existing requirement rather than add a new capability.** The
"binaries are not diffed" decision already lives in `diff-context-expansion`. A
second capability covering the same behaviour would fragment the contract.
Alternative considered: a standalone `binary-placeholder` capability — rejected
because it would duplicate detection and suppression wording already owned by
`diff-context-expansion`.

**Specify the exact placeholder string, "binary file, cannot diff".** The prior
wording allowed any plain label, which is what let an unhelpful bare "binary
file" satisfy the spec. Pinning the explanatory phrasing makes the requirement
testable (`render_dom_test.ts` asserts it) and prevents regression to a bare
label. Alternative considered: require only that the placeholder "explains the
file cannot be diffed" without fixing the wording — rejected as harder to test
and easy to under-deliver against.

## Risks / Trade-offs

- [Spec pins an exact UI string, which can drift if the copy is later reworded] →
  The string is asserted by `render_dom_test.ts`, so a reword breaks the test and
  forces a matching spec update. The requirement and the test move together.
- [Recording a decision the code already satisfies risks looking like a no-op] →
  Accepted deliberately: the value is the durable contract, so a future change
  cannot silently revert the placeholder to a bare, unexplained label.
