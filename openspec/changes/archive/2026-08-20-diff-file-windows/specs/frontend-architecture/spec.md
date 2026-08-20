## ADDED Requirements

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

## MODIFIED Requirements

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
