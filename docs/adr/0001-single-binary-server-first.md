# 0001: Keep a Single-Binary Server-First Architecture

## Status

Accepted

## Context

Bookshelf is intended to be easy to self-host, easy to reason about, and easy to
deploy on a private host or small cluster. The project needs a backend that can
later support additional clients, but it should avoid unnecessary operational
complexity in the early releases.

## Decision

Bookshelf will keep a server-first architecture built around one Go binary that:

- serves the REST API
- serves the embedded frontend
- runs background scans
- owns persistence and migrations

The frontend remains a client of that server rather than a separately deployed app.

## Consequences

- deployment is simpler for self-hosters
- API and frontend development stay closely aligned
- future clients can still be added against the same API
- build and release pipelines must account for embedded frontend assets
