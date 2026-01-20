# ADR-001: Temporal Workflow Engine

## Status

Accepted

## Problem

Payment processing involves long-running operations that may span hours or days. Authorization holds wait for capture. Retry workflows need intelligent scheduling. Multi-step flows can fail at any stage. We need exactly-once semantics despite distributed system challenges.

**The core challenge**: How do we orchestrate complex, long-running payment workflows that survive failures, support durable timers, and integrate external events like webhooks?

| Requirement | Why It Matters |
|-------------|----------------|
| Survive process crashes | Worker restarts shouldn't lose payment state |
| Durable timers | Sleep for days (dunning retries, capture deadlines) |
| Exactly-once semantics | Never charge a customer twice |
| External event integration | Receive webhooks, cancellation requests |
| Visibility | Operations team needs to see workflow state |

## Solutions Considered

### Solution A: Database State Machine + Polling

Store payment state in database. Background jobs poll for payments needing action.

| Pros | Cons |
|------|------|
| Simple, no new infrastructure | Polling overhead and latency |
| Team already knows SQL | Manual retry logic required |
| Easy to query current state | Complex failure recovery code |
| | No native support for long sleeps |
| | Difficult to coordinate multi-step flows |

### Solution B: Message Queues (RabbitMQ/SQS)

Use message queues to trigger payment actions. Each step publishes message for next step.

| Pros | Cons |
|------|------|
| Decoupled processing | No built-in state persistence |
| Reliable message delivery | Complex multi-step coordination |
| Existing team knowledge | Manual correlation of related messages |
| | Difficult to query "current state" |
| | No native timer support |

### Solution C: AWS Step Functions

Use AWS managed state machine service for workflow orchestration.

| Pros | Cons |
|------|------|
| Fully managed, no infrastructure | Vendor lock-in to AWS |
| Built-in state management | Limited flexibility in workflow logic |
| Visual workflow designer | Cost increases with scale |
| | Learning project should demonstrate infrastructure understanding |

### Solution D: Temporal Workflow Engine

Use Temporal for durable workflow orchestration. Workflows define the flow, activities perform side effects.

| Pros | Cons |
|------|------|
| Durable execution survives crashes | Team needs to learn Temporal concepts |
| Native support for long timers | Requires running Temporal server |
| Signals for external events | Determinism constraints on workflow code |
| Built-in activity retries | Debugging replay can be confusing |
| Query support for current state | |
| Battle-tested at Stripe, Uber | |

## Chosen Solution

**Solution D: Temporal Workflow Engine**

### Key Capabilities Used

| Capability | Use Case |
|------------|----------|
| Durable Execution | Workflow survives worker restarts, picks up where it left off |
| Signals | Receive provider webhooks, method updates, cancellation requests |
| Queries | Expose current payment state for API status endpoints |
| Durable Timers | Sleep for calculated retry times (hours/days) |
| Activity Retries | Automatic retry of PSP calls with exponential backoff |
| Child Workflows | Spawn recovery workflows from main payment workflow |

### Workflow Structure

| Step | Type | Purpose |
|------|------|---------|
| 1. Validate Intent | Activity | Verify payment data and customer |
| 2. Select Provider | Activity | Choose PSP based on routing rules |
| 3. Request Authorization | Activity | Call provider API for auth |
| 3a. On Soft Decline | Child Workflow | Spawn recovery workflow for retries |
| 4. Wait for Capture | Signal/Timer | Wait for capture request or auto-capture timer |
| 5. Process Capture | Activity | Call provider API for capture |
| 6. Record Ledger | Activity | Create double-entry ledger records |
| 7. Write Outbox | Activity | Record event for CDC publishing |

### Configuration

| Setting | Value | Rationale |
|---------|-------|-----------|
| Task Queue | `payment-processing` | Single queue for all payment workflows |
| Workflow Timeout | 30 days | Maximum dunning window |
| Activity Initial Interval | 1 second | Quick first retry |
| Activity Backoff Coefficient | 2.0 | Exponential backoff |
| Activity Max Attempts (PSP) | 3 | Limited retries for external calls |
| Activity Max Attempts (DB) | 5 | More retries for internal operations |

## Why This Solution

| Reason | Explanation |
|--------|-------------|
| **Exactly-once semantics** | Activity retries combined with idempotency keys ensure payments are never duplicated, even after crashes. |
| **Long-running support** | Workflows can sleep for days waiting for capture or scheduling retries. No polling or cron jobs needed. |
| **Signal handling** | Webhooks from providers can be delivered as signals to the running workflow, enabling clean event-driven flows. |
| **Visibility** | Temporal UI shows workflow state, history, and pending activities. Operations can see exactly where each payment is. |
| **Failure handling** | Temporal manages state persistence and recovery automatically. No custom failure recovery code needed. |
| **Production proven** | Stripe, Uber, and Netflix use Temporal for similar payment and transaction workflows. |

### Trade-off Acceptance

| Trade-off | Mitigation |
|-----------|------------|
| Learning curve | Use official Temporal Go SDK tutorials; start with simple workflows |
| Infrastructure dependency | Temporal server is well-documented; can use Temporal Cloud for managed option |
| Determinism constraints | Strict separation: workflows are deterministic, activities handle side effects |
| Debugging complexity | Leverage Temporal's testing framework; learn replay debugging |

## References

- [Temporal Documentation](https://docs.temporal.io/)
- [Temporal at Stripe](https://temporal.io/case-studies/stripe)
- [Workflow Engine Comparison](https://docs.temporal.io/evaluate/why-temporal)
