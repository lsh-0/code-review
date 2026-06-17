## ADDED Requirements

### Requirement: Detect external changes to the state file

The system SHALL watch the review's state JSON file and detect when a writer other than this GUI
instance (for example an agent or the review CLI) modifies it while the GUI is open. Multiple writes
in quick succession SHALL be coalesced so a burst of external edits produces a single notification.

#### Scenario: An external write is detected

- **WHEN** a process other than this GUI modifies the state JSON
- **THEN** the GUI is notified that the review state has changed externally

#### Scenario: A burst of external writes notifies once

- **WHEN** the state JSON is written several times within a short window
- **THEN** the GUI receives a single change notification, not one per write

### Requirement: The GUI's own writes do not trigger a notification

The system SHALL NOT raise an external-change notification for writes the GUI itself performs when
saving review state. Only writes by another process SHALL be treated as external.

#### Scenario: Saving a comment does not self-notify

- **WHEN** the reviewer adds a comment and the GUI saves the state file
- **THEN** no external-change notification is raised

### Requirement: External changes surface a reviewer-controlled refresh banner

The system SHALL, on an external change, display a dismissable banner at the top of the window reading
that changes to the review have been made, with a control to refresh. The view SHALL NOT change until
the reviewer activates that control; the GUI SHALL never reload the view underneath the reviewer.

#### Scenario: Banner appears without altering the view

- **WHEN** an external change is detected while the reviewer is reading a file
- **THEN** a top-of-window banner appears offering to refresh
- **AND** the file currently displayed is unchanged until the reviewer acts

#### Scenario: Refreshing from the banner reloads state

- **WHEN** the reviewer activates the banner's refresh control
- **THEN** the review state is reloaded and the visible surface re-rendered
- **AND** the banner is dismissed

#### Scenario: Dismissing the banner leaves the view stale by choice

- **WHEN** the reviewer dismisses the banner without refreshing
- **THEN** the banner is removed
- **AND** the view continues to show the previously loaded state
