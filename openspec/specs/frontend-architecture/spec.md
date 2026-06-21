# frontend-architecture Specification

## Purpose
Keep the frontend tractable by separating concerns into three layers: a pure core that computes
decisions without touching the DOM or the Wails bridge, a typed RPC client that is the sole point of
contact with the backend's bound `App` methods, and a render layer that is the only code permitted to
build DOM nodes. The split makes the core unit-testable without a browser, the render layer verifiable
against a real DOM, and every backend call typed rather than reflected.

## Requirements

### Requirement: Frontend is structured as a pure core, an RPC client, and a render layer

The frontend SHALL be organised into three layers: a pure core that performs computation with no
reference to the DOM or to the Wails bridge; a typed RPC client that is the sole point of contact with
the backend's bound `App` methods; and a render layer that is the only code permitted to touch the DOM.
Non-trivial computation SHALL live in the pure core; the render layer SHALL read state and call DOM
APIs but SHALL NOT itself compute decisions (which lines are visible, which status applies, which order
or label to use).

#### Scenario: A rendering decision is computed by a pure function

- **WHEN** the render layer needs a non-trivial decision, such as which lines of a hunk are visible in
  the overview window or what status a file's comments aggregate to
- **THEN** that decision is produced by a pure function in the core
- **AND** the render layer only consumes the result to build DOM nodes

#### Scenario: The core does not reference the DOM or the bridge

- **WHEN** the pure core is compiled and tested
- **THEN** it can run without a DOM and without the Wails bridge being present

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
