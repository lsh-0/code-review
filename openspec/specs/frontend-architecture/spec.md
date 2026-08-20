# frontend-architecture Specification

## Purpose
Keep the frontend tractable by separating concerns into three layers: a pure core that computes
decisions without touching the DOM or the Wails bridge, a typed RPC client that is the sole point of
contact with the backend's bound `App` methods, and a render layer that is the only code permitted to
build DOM nodes. The split makes the core unit-testable without a browser, the render layer verifiable
against a real DOM, and every backend call typed rather than reflected. The frontend's view state is
the single source of truth for what is on screen — the DOM is disposable output, rebuilt wholesale from
state on every change rather than patched in place — and the core never re-implements semantics (such
as comment placement) that the backend already derives.

## Requirements

### Requirement: Frontend is structured as a pure core, an RPC client, and a render layer

The frontend SHALL be organised into three layers: a pure core that performs computation with no
reference to the DOM or to the Wails bridge; a typed RPC client that is the sole point of contact with
the backend's bound `App` methods; and a render layer that is the only code permitted to touch the DOM.
Non-trivial computation SHALL live in the pure core; the render layer SHALL read state and call DOM
APIs but SHALL NOT itself compute decisions (which lines are visible, which windows exist, which order
or label to use).

The pure core SHALL NOT re-implement semantics the backend owns. Comment placement and outdatedness
arrive as backend-derived fields over the bridge and SHALL be consumed as data; the core computes
view decisions (window arithmetic, threading layout, grouping) over that data.

#### Scenario: A rendering decision is computed by a pure function

- **WHEN** the render layer needs a non-trivial decision, such as a file's window set or what status a
  file's comments aggregate to
- **THEN** that decision is produced by a pure function in the core, or arrives pre-derived from the
  backend
- **AND** the render layer only consumes the result to build DOM nodes

#### Scenario: The core does not reference the DOM or the bridge

- **WHEN** the pure core is compiled and tested
- **THEN** it can run without a DOM and without the Wails bridge being present

#### Scenario: Comment semantics are not re-derived in the frontend

- **WHEN** the frontend needs a comment's line, outdatedness, or untethered context
- **THEN** it reads fields the backend derived
- **AND** no frontend code inspects anchor histories

### Requirement: The view model is the single source of truth for what is on screen

Everything the diff pane displays SHALL be derivable from the frontend's state: the diff files, the
per-file window ranges, the fetched file lines, and the comment caches. No view state SHALL exist
only in the rendered DOM — revealed lines, growth extents, and comment placements SHALL all live in
state, so that discarding and re-rendering a region reproduces the same view. Asynchronous results
(fetched lines, mutation results) SHALL be applied to state, never spliced directly into the
rendered tree; whether they become visible is decided by rendering from state.

#### Scenario: Re-rendering reproduces the view

- **WHEN** a file region is discarded and re-rendered from state with no state change in between
- **THEN** the rendered tree shows the same windows, revealed lines, and comment threads as before

#### Scenario: An async result lands in state, not the DOM

- **WHEN** a line-range fetch completes after the reviewer has navigated to a different file
- **THEN** the fetched lines are recorded in that file's state and nothing is inserted into the
  currently rendered tree
- **AND** the lines appear when that file is next rendered

### Requirement: View changes re-render regions rather than patching the DOM

A change to a file's view — a comment mutation, window growth, or refresh — SHALL be applied by
re-rendering that file's region from state (its section in the combined view; the diff pane in the
single-file view). The render layer SHALL NOT locate and surgically modify existing DOM nodes to
reflect state changes. The pane's scroll position SHALL be preserved across a region re-render.
Features that annotate rendered output they do not own (the in-page find highlights) SHALL re-apply
their annotations after a region render.

#### Scenario: A comment mutation re-renders the region

- **WHEN** the reviewer adds, edits, resolves, or deletes a comment in a file
- **THEN** that file's region is re-rendered from the updated state, preserving the pane's scroll
  position and the file's window state

#### Scenario: Find highlights survive a region re-render

- **WHEN** the in-page find has highlighted matches and a comment mutation re-renders a region
- **THEN** the find highlights are re-applied to the freshly rendered content

### Requirement: The pure core is unit-tested without a browser

The pure-core logic SHALL be covered by unit tests that run without a DOM and without any third-party
test dependency. These tests SHALL pin requirements — the diff/comment transforms, the
context-expansion state machine, comment threading and authorship, end-of-file detection, and status
derivation — rather than re-asserting implementation structure.

#### Scenario: Core tests run with no DOM dependency

- **WHEN** the pure-core unit tests are executed
- **THEN** they pass without a DOM shim or browser
- **AND** they require no third-party library to run

#### Scenario: Expansion state machine is pinned by tests

- **WHEN** the context-expansion logic computes the next range, whether a gap is exhausted, or where
  two converging affordances meet
- **THEN** a unit test asserts the computed result against a known expected value

### Requirement: The render layer is verified against a real DOM

The render layer SHALL be covered by integration tests that build markup into an actual DOM and assert
on the resulting tree (nodes, classes, attributes, ordering), rather than asserting on returned markup
strings. These tests MAY depend on a single sandboxed DOM shim.

#### Scenario: Rendering a diff produces the expected DOM tree

- **WHEN** a render function is given known diff and comment state and run against the DOM shim
- **THEN** the test queries the produced tree and asserts the expected structure
- **AND** the assertion is on DOM nodes, not on a markup string

### Requirement: Frontend-backend communication goes through a typed client

All calls from the frontend to the backend SHALL pass through the typed RPC client, which wraps the
Wails-generated bindings. There SHALL NOT be a reflection-based or untyped bridge helper at individual
call sites.

#### Scenario: A backend call uses a typed client method

- **WHEN** the frontend fetches a file's diff or mutates a comment
- **THEN** it calls a typed method on the RPC client whose parameters and result type are declared
- **AND** no per-call-site reflection or `interface{}`-style decoding is involved
