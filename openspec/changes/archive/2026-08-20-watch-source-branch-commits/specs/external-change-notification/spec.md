## ADDED Requirements

### Requirement: Detect new commits on the source branch

The system SHALL watch the review's source branch and detect when its HEAD commit changes while the
GUI is open, so a commit made underneath the reviewer — by an agent, or by the reviewer themselves
in another terminal — is reported rather than left silent. Detection SHALL be by periodically
re-reading the branch's commit SHA, in the same manner as the working-tree status poll, and SHALL NOT
depend on the review state file being written.

The first successful read SHALL establish the baseline and SHALL NOT be reported as a change. A
subsequent read differing from the baseline SHALL be reported once, and SHALL become the new baseline
so the same commit is not reported again. A failed read SHALL be skipped, leaving the previous
baseline in place, so a transient git error does not raise a false notification.

Detection SHALL NOT alter what is displayed: the diff, file list, and marked-file set on screen remain
as they were until the reviewer chooses to refresh.

#### Scenario: A new commit on the source branch is detected

- **WHEN** a commit is made on the review's source branch while the GUI is open
- **THEN** the GUI is notified that the review's commits have changed
- **AND** the displayed diff is unchanged until the reviewer acts

#### Scenario: The branch's initial commit is not reported as a change

- **WHEN** the commit watch starts and reads the source branch's HEAD for the first time
- **THEN** no change is reported

#### Scenario: A steady branch reports nothing

- **WHEN** the source branch's HEAD is unchanged across successive reads
- **THEN** no change is reported

#### Scenario: The same commit is reported only once

- **WHEN** a new commit has been detected and reported, and no further commit is made
- **THEN** that commit is not reported again on subsequent reads

#### Scenario: A failed read does not report a change

- **WHEN** reading the source branch's commit SHA fails
- **THEN** no change is reported
- **AND** the previously established baseline is retained

#### Scenario: A commit is detected on a clean working tree

- **WHEN** a commit is made on the source branch and the working tree is clean both before and after
- **THEN** the new commit is still detected

### Requirement: Commit changes and state-file changes are detected independently

The system SHALL treat the source-branch commit watch and the review-state-file watch as separate
detections, each with its own trigger. A new commit SHALL raise a notification even when the state
file is untouched, and an external state-file write SHALL raise a notification even when no commit has
been made. Neither watch SHALL suppress the other.

#### Scenario: A commit with no state-file write still notifies

- **WHEN** a commit is made on the source branch and the review state file is not modified
- **THEN** the GUI is notified that the review has changed

#### Scenario: A state-file write with no commit still notifies

- **WHEN** the review state file is modified externally and no commit is made
- **THEN** the GUI is notified that the review has changed

### Requirement: A watch suspends while its alert is raised

The system SHALL stop polling for a given kind of change once it has reported one, and SHALL resume
only when the resulting alert is cleared. A watch SHALL NOT re-report a change the reviewer has already
been told about and has not yet acted on.

Suspension SHALL be per watch, not global: one watch being suspended SHALL NOT prevent another from
detecting and reporting its own kind of change.

For the refresh banner, the alert SHALL be considered cleared when the reviewer either refreshes or
dismisses it, since both remove the banner. On resuming, the watch SHALL re-establish its baseline from
the current state rather than comparing against the baseline held before the alert, so that the change
already reported is not reported again.

#### Scenario: A reported change suspends further polling

- **WHEN** a change has been detected and the refresh banner is raised
- **THEN** that watch performs no further reads until the banner is cleared

#### Scenario: Refreshing resumes the watch

- **WHEN** the reviewer refreshes from the banner
- **THEN** the watch resumes
- **AND** its baseline is re-established from the current state
- **AND** the change just refreshed away is not reported again

#### Scenario: Dismissing resumes the watch

- **WHEN** the reviewer dismisses the banner without refreshing
- **THEN** the watch resumes
- **AND** the dismissed change is not reported again
- **AND** a subsequent, different change is reported

#### Scenario: A suspended watch does not blind the others

- **WHEN** the commit watch is suspended because a new commit has been reported
- **AND** the review state file is then modified externally
- **THEN** the state-file change is still reported

#### Scenario: Changes arriving while suspended are absorbed on resume

- **WHEN** further commits are made while the commit watch is suspended
- **AND** the reviewer then clears the banner
- **THEN** the watch re-baselines to the latest commit
- **AND** no alert is raised for the commits that arrived while suspended

## MODIFIED Requirements

### Requirement: External changes surface a reviewer-controlled refresh banner

The system SHALL, on an external change — an external write to the state file, or a new commit on the
source branch — display a dismissable banner at the top of the window reading that changes to the
review have been made, with a control to refresh. The banner's message SHALL identify which kind of
change it is reporting: new commits, external review-state edits, or both when both have occurred
since the banner was last cleared. The view SHALL NOT change until the reviewer activates the refresh
control; the GUI SHALL never reload the view underneath the reviewer.

Refreshing from the banner SHALL both reload the review state and recompute the diff, so a new
commit's files, re-anchored comments, and re-evaluated file marks are all picked up by the one
control.

#### Scenario: Banner appears without altering the view

- **WHEN** an external change is detected while the reviewer is reading a file
- **THEN** a top-of-window banner appears offering to refresh
- **AND** the file currently displayed is unchanged until the reviewer acts

#### Scenario: A new commit raises the banner

- **WHEN** a new commit is detected on the source branch
- **THEN** the same top-of-window refresh banner appears
- **AND** its message reports that new code was committed

#### Scenario: An external state-file write raises the banner

- **WHEN** an external write to the state file is detected
- **THEN** the refresh banner appears
- **AND** its message reports that the review's comments have changed

#### Scenario: Both kinds of change report both

- **WHEN** a new commit and an external state-file write are both detected before the reviewer acts
- **THEN** the banner's message reports both

#### Scenario: Refreshing from the banner reloads state

- **WHEN** the reviewer activates the banner's refresh control
- **THEN** the review state is reloaded and the visible surface re-rendered
- **AND** the diff is recomputed so newly committed changes appear
- **AND** the banner is dismissed

#### Scenario: Refreshing clears the recorded change sources

- **WHEN** the reviewer refreshes from the banner and a single new change is then detected
- **THEN** the banner's message reports only that new change, not the ones already refreshed away

#### Scenario: Dismissing the banner leaves the view stale by choice

- **WHEN** the reviewer dismisses the banner without refreshing
- **THEN** the banner is removed
- **AND** the view continues to show the previously loaded state
