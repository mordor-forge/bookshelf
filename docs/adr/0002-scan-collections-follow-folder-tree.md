# 0002: Scan Collections Follow the Folder Tree

## Status

Accepted

## Context

Bookshelf derives part of its organization model from the on-disk library tree.
There was a temporary flattening change that reduced scan-derived collections to
top-level directories only, but that loses information that future clients and
deeper library organization may need.

## Decision

Scan-derived collections will mirror the folder hierarchy under the configured
library root.

Implications:

- nested folders become nested scan collections
- a scanned book links to its immediate parent scan collection
- scan-derived collection memberships are treated as read-only derived state
- manual collections remain the editable user-managed grouping layer

## Consequences

- the backend preserves more structure than a flattened view
- clients can still render a simplified UI if they want
- scan reconciliation logic must keep derived links consistent on rescan
- future format and multi-client work has a stronger canonical hierarchy to build on
