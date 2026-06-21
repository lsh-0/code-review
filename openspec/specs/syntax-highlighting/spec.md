# syntax-highlighting Specification

## Purpose
Make diff code readable at a glance by syntax-highlighting the source shown in added, removed, and
context lines according to the file's language. Highlighting is delegated to a third-party library
invoked at the render boundary; the project's responsibility is limited to selecting the language and
keeping rendering intact whether highlighting is applied, absent, or fails. The library is bundled with
only a curated set of languages to keep the artefact small.

## Requirements

### Requirement: Diff code is syntax-highlighted

The system SHALL syntax-highlight the source code shown in diff lines, so that code is readable at a
glance. Highlighting SHALL be applied to the content of added, removed, and context lines according to
the file's language.

#### Scenario: A code line is highlighted by language

- **WHEN** a diff line of a recognised language is rendered
- **THEN** its content is displayed with syntax highlighting for that language

#### Scenario: An unrecognised or unsupported language is shown plainly

- **WHEN** a diff line belongs to a file whose language is not in the curated set
- **THEN** its content is displayed without highlighting and without error

### Requirement: Highlighting is supplied by a third-party library at the render boundary

Highlighting SHALL be delegated to a third-party library, invoked from the render layer. The library's
highlighting behaviour is out of scope for this project's tests — the system SHALL NOT include tests
that assert the library's highlighted output. The system's own responsibility is limited to selecting
the file's language and not breaking rendering when highlighting is applied or absent.

#### Scenario: The library performs the highlighting

- **WHEN** a diff line of a recognised language is rendered
- **THEN** the third-party highlighting library produces the highlighted form
- **AND** the project does not test the library's output, only that the correct language was selected
  and the line still renders

#### Scenario: Highlighting failure does not break the diff

- **WHEN** highlighting is unavailable or errors for a line
- **THEN** the line still renders with its plain content
- **AND** the surrounding diff, line numbers, and comment threads are unaffected

### Requirement: Only reviewed languages are bundled

The highlighting library SHALL be included with only a curated set of languages registered, rather than
its full language set, to keep the bundled artefact small.

#### Scenario: The bundle omits unused languages

- **WHEN** the frontend is bundled
- **THEN** only the curated languages' highlighting definitions are included
- **AND** the artefact does not carry the library's full language set
