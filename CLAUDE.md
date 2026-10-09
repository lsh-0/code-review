this project is a local-first code-review GUI intended to create a stable, familiar and comfortable interface for the review of changes between the current branch and the main/master/default branch - the sort of code review that would be done on a Github or Bitbucket Pull Request (PR).

the goals are:
* speed. the program should be fast to start, the UI responsiveness should be immediate. Speed of reviewing code should only be limited by ourselves.
* utility/practicality. the program should prioritise getting work done over looks and following trends.
* portability. the program should be small and self contained.
* automation. Opt-in AI capabilities are available to address review feedback but are _not_ tightly coupled to any one AI agent.

## private and sensitive data

this repository is public. never commit private or sensitive data to it: recordings of a private repository's API, organisation names, people's names, account identifiers, UUIDs, pull request titles, branch names, ticket keys or review comment text from private work. credentials and tokens are never committed at all.

test fixtures are either recordings of a public source, with that source named in the fixture's README, or hand-built synthetic data. before copying fixtures from another project, check where each one came from. when a test needs the structure of private data, build a synthetic payload with the same structure.
