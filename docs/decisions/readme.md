# Architectural Decision Records (ADRs)

This directory contains Architectural Decision Records documenting significant technical decisions made during the design and development of the payment processing service.

## What is an ADR?

An Architectural Decision Record captures an important architectural decision along with its context and consequences. ADRs help future developers understand why certain decisions were made.

## ADR Template

Each ADR follows this structure:

```markdown
# ADR-XXX: Title

## Status
[Proposed | Accepted | Deprecated | Superseded]

## Context
What is the issue that we're seeing that is motivating this decision?

## Decision
What is the change that we're proposing and/or doing?

## Consequences
What becomes easier or more difficult because of this change?
```

## Decision Log

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [001](001-temporal-workflow-engine.md) | Temporal Workflow Engine | Accepted | Dec 2025 |
| [002](002-multi-provider-adapters.md) | Multi-Provider Adapter Pattern | Accepted | Jan 2026 |
| [003](003-transactional-outbox.md) | Transactional Outbox Pattern | Accepted | Jan 2026 |
| [004](004-double-entry-bookkeeping.md) | Double-Entry Bookkeeping | Accepted | Jan 2026 |

## Pending Decisions

Decisions still under consideration:

- Database sharding strategy for high-volume scaling
- Multi-currency support and exchange rate handling
- Smart routing between payment providers

## How to Contribute

1. Copy the template to a new file with the next ADR number
2. Fill in the sections
3. Set status to "Proposed"
4. Submit for review
5. Update status to "Accepted" when approved
