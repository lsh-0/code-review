# frontend-toolchain Specification

## Purpose
Build the frontend with a Deno toolchain that type-checks, tests, and bundles the TypeScript to a single
JavaScript artefact, replacing GopherJS and removing the need for `node_modules`. Deno is strictly a
build-time tool: the shipped application embeds only the bundled JavaScript and acquires no new runtime
dependency, process, or network capability from the toolchain change.

## Requirements

### Requirement: The frontend is built with a Deno toolchain

The frontend SHALL be type-checked, tested, and bundled to JavaScript using Deno. The build SHALL NOT
depend on GopherJS, and SHALL NOT require a `node_modules` directory.

#### Scenario: Building the frontend uses Deno only

- **WHEN** the project's build is run
- **THEN** the TypeScript frontend is type-checked, tested, and bundled by Deno
- **AND** GopherJS is neither installed nor invoked
- **AND** no `node_modules` directory is produced or required

#### Scenario: Setup no longer provisions GopherJS

- **WHEN** the project's setup step is run
- **THEN** it does not install GopherJS or pin a GopherJS-compatible Go version for the frontend

### Requirement: Deno is a build-time tool, not a runtime dependency

The shipped application SHALL embed only the bundled JavaScript artefact. Deno SHALL NOT be present in,
required by, or invoked by the running application, and the application SHALL NOT acquire any new
runtime permission or network capability as a result of the toolchain change.

#### Scenario: The running app has no Deno dependency

- **WHEN** the built binary is run on a machine without Deno installed
- **THEN** the application starts and serves its embedded frontend
- **AND** no Deno process is started and no Deno permission is requested at runtime

### Requirement: The build produces a single embedded frontend artefact

The Deno build SHALL emit the JavaScript the WebKit webview loads as the embedded asset that the Go
binary serves, replacing the GopherJS-compiled `review.js`. The compiled GopherJS artefact SHALL be
removed.

#### Scenario: The bundled JS becomes the served asset

- **WHEN** the frontend is bundled
- **THEN** the resulting JavaScript is the asset embedded and served by the backend
- **AND** the previous GopherJS-compiled `review.js` is no longer present
