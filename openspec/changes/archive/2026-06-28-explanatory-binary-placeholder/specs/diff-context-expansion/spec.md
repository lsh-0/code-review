## MODIFIED Requirements

### Requirement: Binary files are listed but not rendered

A file whose diff carries the `Binary files ... differ` marker SHALL be flagged as binary during diff parsing. A binary file SHALL still appear in the file navigation, but selecting it SHALL show an explanatory placeholder that states the file cannot be diffed — the text "binary file, cannot diff" — instead of a diff body, and SHALL NOT render hunks, expansion affordances, or fetch the file's blob content. The placeholder SHALL explain why there is no diff body rather than merely labelling the file as binary, so the empty body reads as expected rather than broken.

#### Scenario: Diff marks a file as binary

- **WHEN** a changed file's diff section contains `Binary files ... differ` and no hunks
- **THEN** that file is flagged as binary in the parsed diff

#### Scenario: Binary file appears in navigation

- **WHEN** the file list is rendered and one changed file is binary
- **THEN** the binary file is listed alongside text files

#### Scenario: Selecting a binary file

- **WHEN** the reviewer selects a binary file
- **THEN** an explanatory placeholder reading "binary file, cannot diff" is shown in place of a diff body, with no hunks and no expansion affordances, and no blob content is fetched for it
