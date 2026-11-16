# Job text selection

Job titles, description bodies, and description summaries use the existing
`select-none` utility. This applies to list rows, the job page and its sticky
title, the tracking drawer, publication previews, and the locations/copies page.
The shared description component carries the same behavior into its other hosts.

The restriction applies to the displayed text, leaving surrounding controls and
editable fields usable. Text remains rendered as HTML and available to assistive
technology. This discourages ordinary browser text selection; it does not provide
confidentiality or prevent extraction through screenshots, OCR, or browser tools.

This frontend change needs no database migration, API change, or reindex.
