## ADDED Requirements

### Requirement: Default context menu in the released GUI

The released GUI SHALL present the webview's default context menu on right-click,
not only in development and debug builds. This is enabled by setting
`EnableDefaultContextMenu` to true on the Wails `options.App`, so the menu is the
webview's native menu rather than an application-drawn widget. Over a text selection
the menu SHALL offer at least a copy action.

#### Scenario: Right-clicking a diff selection offers copy

- **WHEN** a reviewer selects one or more lines in the diff and right-clicks the selection in a released build
- **THEN** a context menu appears offering at least a copy action

#### Scenario: Copying a diff selection yields the code without line numbers

- **WHEN** a reviewer copies a selection of diff lines via the context menu
- **THEN** the copied text is the line contents only, excluding the gutter line numbers (which are rendered via CSS and are not part of the selection)

#### Scenario: Production matches development

- **WHEN** the GUI runs as an installed release binary
- **THEN** a right-click context menu is available, as it already is under `wails dev`
