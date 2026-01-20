# ADR-001: Temporal Workflow Engine

## Status

Accepted

## Context

Payment processing involves long-running operations that may span hours or days:

- Authorization holds that wait for capture
- Retry workflows with intelligent scheduling (align with paydays)
- Multi-step flows with potential failures at any stage
- Need for exactly-once semantics despite distributed system challenges

Traditional approaches have significant limitations:

1. **State machines in database**: Requires polling, complex failure recovery, manual retry logic
2. **Message queues**: No built-in state persistence, complex multi-step coordination
3. **Cron jobs**: Imprecise timing, no guaranteed delivery, stateless

We need a solution that:
- Survives process crashes and restarts
- Provides durable timers (sleep for days if needed)
- Enables exactly-once semantics
- Supports signals for external events (webhooks)
- Offers visibility into running workflows

## Decision

We will use **Temporal** as the workflow orchestration engine for payment processing.

### Key Capabilities Used

| Capability | Use Case |
|------------|----------|
| **Durable Execution** | Workflow survives worker restarts, picks up where it left off |
| **Signals** | Receive provider webhooks, method updates, cancellation requests |
| **Queries** | Expose current payment state for API status endpoints |
| **Durable Timers** | Sleep for calculated retry times (hours/days) |
| **Activity Retries** | Automatic retry of PSP calls with exponential backoff |
| **Child Workflows** | Spawn recovery workflows from main payment workflow |

### Workflow Design

```
PaymentWorkflow
├── ValidateIntent (activity)
├── SelectProvider (activity)
├── RequestAuthorization (provider-specific activity)
│   └── On soft decline: spawn RecoveryWorkflow
├── WaitForCapture (signal or timer)
├── ProcessCapture (provider-specific activity)
├── RecordLedger (activity)
└── WriteOutbox (activity)
```

### Configuration

| Setting | Value | Rationale |
|---------|-------|-----------|
| Task Queue | `payment-processing` | Single queue for all payment workflows |
| Workflow Timeout | 30 days | Maximum dunning window |
| Activity Initial Interval | 1 second | Quick first retry |
| Activity Backoff Coefficient | 2.0 | Exponential backoff |
| Activity Max Attempts | 3 (PSP) / 5 (DB) | Limited PSP retries, more for internal |

## Consequences

### Positive

- **Simplified failure handling**: Temporal manages state persistence and recovery
- **Exactly-once semantics**: Through activity retry + idempotency keys
- **Long-running support**: Native support for workflows spanning days
- **Visibility**: Temporal UI shows workflow state, history, pending activities
- **Signal handling**: Clean integration point for webhooks
- **Battle-tested**: Used by Stripe, Uber, Netflix for similar use cases

### Negative

- **Learning curve**: Team needs to understand Temporal concepts (determinism, activities vs workflows)
- **Infrastructure dependency**: Requires running Temporal server cluster
- **Debugging complexity**: Workflow replay can be confusing initially
- **Determinism constraints**: Cannot use non-deterministic operations in workflows

### Mitigations

- Use official Temporal Go SDK tutorials and documentation
- Start with simple workflows and add complexity incrementally
- Leverage Temporal's testing framework for workflow testing
- Strict separation between workflow code (deterministic) and activity code (side effects)

## Alternatives Considered

### 1. Database State Machine + Polling

- **Pros**: Simple, no new infrastructure
- **Cons**: Polling overhead, manual retry logic, no native long sleep support
- **Rejected**: Too much custom code for failure handling

### 2. AWS Step Functions

- **Pros**: Managed service, no infrastructure
- **Cons**: Vendor lock-in, limited flexibility, cost at scale
- **Rejected**: Learning project should demonstrate infrastructure understanding

### 3. Apache Airflow

- **Pros**: Mature, well-known
- **Cons**: Designed for batch processing, not real-time workflows
- **Rejected**: Wrong tool for payment latency requirements

## References

- [Temporal Documentation](https://docs.temporal.io/)
- [Temporal at Stripe](https://temporal.io/case-studies/stripe)
- [Workflow Engine Comparison](https://docs.temporal.io/evaluate/why-temporal)
