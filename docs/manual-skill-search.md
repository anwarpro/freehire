# Manual skill search

The Skills button above the job list opens the existing Skills pane directly.
The pane now provides a job-text keyword input for any skill, including Mongoose
and terms outside the taught curriculum or current dictionary. This input edits
the main search query, which remains visible in the header and removable in the
filter summary. Country, source, seniority, and other filters remain selected.

Tagged skills retain their existing include/exclude and Any/All controls. The
keyword field is explicitly separate: it searches titles/descriptions and does not claim a
new structured skill was recognized. An empty dictionary search also offers to
use its term as the job-text query. Keyword changes replace title-only field scoping
left by a previous search completion with title/description search. A company name
alone is not a keyword match. Clearing the keyword clears that field scope.

Changes stay in the modal until Show results; Cancel leaves the live query alone.
The existing search URL carries keywords through pagination, reload, saved
searches, and alerts. No curriculum edits, schema migration, index-setting change,
or reindex is required. Plain profile skill pickers do not expose this search field.
