## ADDED Requirements

### Requirement: Pure renames are listed with an explanatory placeholder

The diff SHALL be computed with rename detection forced on (git `-M`), so a file move with no content change is reported as a rename rather than as an unrelated delete-plus-add pair, regardless of the user's `diff.renames` configuration. A file whose diff carries a `rename from <path>` marker SHALL be flagged as renamed during diff parsing, recording its pre-rename path. A renamed file SHALL appear in the file navigation. Selecting a pure rename — one flagged as renamed and carrying no hunks — SHALL show an explanatory placeholder naming the old path ("renamed from `<old>`, no changes") in place of a diff body, with no hunks rendered, so the empty body reads as expected rather than broken. A rename that also carries content edits SHALL render its hunks normally and SHALL NOT show the placeholder.

#### Scenario: Diff forces rename detection

- **WHEN** a file is moved with no content change and the repository has `diff.renames` disabled
- **THEN** the diff still reports it as a rename, carrying `rename from`/`rename to` markers rather than a separate delete and add

#### Scenario: Diff marks a file as a pure rename

- **WHEN** a changed file's diff section contains a `rename from <old>` marker and no hunks
- **THEN** that file is flagged as renamed in the parsed diff and its old path is recorded

#### Scenario: Renamed file appears in navigation

- **WHEN** the file list is rendered and one changed file is a pure rename
- **THEN** the renamed file is listed alongside other changed files

#### Scenario: Selecting a pure rename

- **WHEN** the reviewer selects a file flagged as renamed that carries no hunks
- **THEN** an explanatory placeholder reading "renamed from `<old>`, no changes" is shown in place of a diff body, with no hunks rendered

#### Scenario: Rename with edits renders normally

- **WHEN** the reviewer selects a file flagged as renamed that also carries hunks
- **THEN** its hunks render as for any other changed file and no rename placeholder is shown
