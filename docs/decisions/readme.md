# Architectural Decision Records (ADRs)

This directory contains Architectural Decision Records documenting significant technical decisions made during the design and development of the payment processing service.

## Decision Summary

| ADR | Decision | Summary |
|-----|----------|---------|
| [001](001-temporal-workflow-engine.md) | Temporal Workflow Engine | Use Temporal for durable workflow orchestration instead of database state machines or message queues. Provides exactly-once semantics, durable timers, and signal handling for long-running payment flows. |
| [002](002-multi-provider-adapters.md) | Multi-Provider Adapter Pattern | Normalize all provider webhooks to canonical events at the edge. Core system only uses canonical types, making it provider-agnostic and easy to extend. |
| [003](003-transactional-outbox.md) | Transactional Outbox Pattern | Write events to an outbox table in the same transaction as business data. Use CDC (Debezium) to publish to Kafka. Solves the dual-write problem without distributed transactions. |
| [004](004-double-entry-bookkeeping.md) | Double-Entry Bookkeeping | Track all money movement with balanced debit/credit entries. Provides mathematical proof of correctness, audit trails, and self-auditing through clearing account monitoring. |

## Pending Decisions

Decisions still under consideration are documented in [pending-decisions.md](pending-decisions.md). These require input before implementation can proceed.

## ADR Format

Each ADR follows this structure:

| Section | Purpose |
|---------|---------|
| **Status** | Proposed, Accepted, Deprecated, or Superseded |
| **Problem** | What issue or unknown are we addressing? |
| **Solutions Considered** | List of options with pros/cons |
| **Chosen Solution** | Which option was selected and its details |
| **Why This Solution** | Reasoning for the choice |

## How to Contribute

1. For new decisions, add to [pending-decisions.md](pending-decisions.md) first
2. Once a decision is made, create a new ADR file with the next number
3. Update this readme with the decision summary
4. Remove from pending decisions
