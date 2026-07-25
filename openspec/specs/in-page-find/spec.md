# in-page-find Specification

## Purpose
TBD - created by archiving change add-in-page-find. Update Purpose after archive.
## Requirements
### Requirement: Ctrl-f opens an in-page find bar

The application SHALL provide its own find bar, because WebKitGTK does not provide
the browser's native find dialogue. Pressing **Ctrl-f** anywhere in the GUI SHALL
open a find bar overlaying the diff area and place keyboard focus in its text
input. Pressing Ctrl-f while the bar is already open SHALL re-focus and select
the input rather than opening a second bar.

#### Scenario: Ctrl-f opens the bar and focuses the input

- **WHEN** the user presses Ctrl-f and no find bar is open
- **THEN** a find bar appears with a text input, a match counter, and previous /
  next / close controls
- **AND** the input receives keyboard focus

#### Scenario: Ctrl-f re-focuses an already-open bar

- **WHEN** a find bar is already open and the user presses Ctrl-f
- **THEN** no second bar is created
- **AND** the existing input is focused with its text selected

### Requirement: The find bar searches the whole page for case-insensitive matches

Entering a term SHALL search the whole application page — the diff text, the
embedded comment threads, the file list, and the header — for case-insensitive,
non-overlapping occurrences of the term, and SHALL highlight every occurrence in
place, mirroring a browser's native find over the HTML document. An empty or
whitespace-only term SHALL match nothing and clear any existing highlights.
Hidden content (for example the collapsed change banner) and the find bar's own
controls SHALL NOT be searched.

#### Scenario: A term highlights every matching occurrence

- **WHEN** the user types a term that occurs several times on the page
- **THEN** each occurrence is highlighted, regardless of letter case
- **AND** occurrences inside embedded comment text are highlighted as well

#### Scenario: An empty term clears highlights

- **WHEN** the user clears the find input to empty or whitespace
- **THEN** no occurrences remain highlighted
- **AND** the match counter shows nothing

#### Scenario: The file list is searched

- **WHEN** a term that appears in a file-list entry is entered
- **THEN** the file-list entry is highlighted like any other page match

#### Scenario: Hidden content is not searched

- **WHEN** a term appears only inside a hidden element (such as the collapsed
  change banner)
- **THEN** it is neither highlighted nor counted

### Requirement: A match counter reports total and active position

The find bar SHALL display the number of matches and which match is active. With
matches present it SHALL read "<active> of <total>"; with a non-empty term and no
matches it SHALL read "No matches"; with an empty term it SHALL display nothing.

#### Scenario: Counter shows active position of total

- **WHEN** a term has three matches and the second is active
- **THEN** the counter reads "2 of 3"

#### Scenario: Counter reports no matches

- **WHEN** a non-empty term matches nothing in the diff
- **THEN** the counter reads "No matches"

### Requirement: Stepping moves the active match with wraparound and scrolls it into view

The bar SHALL provide forward and backward stepping through the matches, bound to
**Enter** / **F3** (forward) and **Shift+Enter** / **Shift+F3** (backward), as
well as to the next / previous buttons. Enter and Shift+Enter act while the input
has focus; F3 and Shift+F3 act globally, matching a browser's find-again idiom.
Stepping SHALL move the active selection by one match, wrapping from the last
match to the first and from the first to the last. The newly active match SHALL
be styled distinctly from the other matches and SHALL be scrolled into view.
Stepping with no matches SHALL be a no-op. Pressing F3 while the find bar is
closed SHALL open it.

#### Scenario: Next steps forward and wraps

- **WHEN** the last match is active and the user presses Enter, F3, or the next
  button
- **THEN** the first match becomes active
- **AND** it is scrolled into view and shown as the active match

#### Scenario: Previous steps backward and wraps

- **WHEN** the first match is active and the user presses Shift+Enter, Shift+F3,
  or the previous button
- **THEN** the last match becomes active

#### Scenario: F3 opens the bar when it is closed

- **WHEN** no find bar is open and the user presses F3
- **THEN** the find bar opens with its input focused

#### Scenario: Stepping with no matches does nothing

- **WHEN** there are no matches and the user presses Enter
- **THEN** the active selection does not change and nothing scrolls

### Requirement: Closing the find bar removes highlights and restores the DOM

Pressing **Escape** or activating the close button SHALL close the find bar. On
close, every highlight SHALL be removed and the page DOM SHALL be restored to the
exact text it held before the search, and keyboard focus SHALL return to the diff
so the next keypress scrolls the page. Repeated open / close cycles SHALL NOT
leave residual or nested highlight markup, and a term that straddles where a
highlight previously sat SHALL still be found on the next search.

#### Scenario: Escape closes the bar and clears highlights

- **WHEN** the user presses Escape with the find bar open
- **THEN** the bar is removed and all highlights are gone
- **AND** the page text is identical to before the search and focus returns to
  the diff

#### Scenario: Repeated searches do not corrupt the diff text

- **WHEN** the user searches, clears, and searches again for a term that spans
  where a previous highlight ended
- **THEN** the term is still found and highlighted correctly
- **AND** no nested or leftover highlight markup remains
