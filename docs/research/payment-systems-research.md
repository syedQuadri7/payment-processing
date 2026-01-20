# Production-grade payment system architecture: patterns from Stripe, Square, and Adyen

Building a payment system that won't need to be rebuilt requires adopting patterns proven at companies processing billions of transactions. The core architectural truth is this: **payments are promises about money movement, not money movement itself**—and every design decision flows from understanding that distinction. Stripe, Square/Block, Adyen, and major banks converge on remarkably similar patterns: stateless API layers with asynchronous processing backends, double-entry bookkeeping for mathematical correctness, event sourcing for auditability, and idempotency as a first-class concern. These systems separate the payment intent (what is being paid) from the payment method (how it's paid) and the order (what's being purchased), enabling support for diverse payment methods without architectural rewrites.

The most critical insight from studying these systems is that **exactly-once payment processing is achieved through at-least-once delivery combined with idempotent consumers**—true exactly-once delivery is theoretically impossible in distributed systems. This shapes everything from API design to database schema to event streaming architecture.

---

## How Stripe, Square, and Adyen architect their payment cores

Stripe's infrastructure processes **over 5 million queries per second** across petabytes of financial data using a customized MongoDB deployment called DocDB, with **5,000+ collections across 2,000+ database shards**. Their architecture separates concerns through a layered approach: database proxy servers built in Go handle routing and access control, a chunk metadata service maps data to shards, and replica sets provide high availability. Change Data Capture pipelines transport database operation logs to Kafka, then archive to S3 for durability.

Square's approach differs significantly in their choice of **Google Cloud Spanner** for their "Books" ledger service—a globally distributed SQL database that eliminates the complexity of manual sharding. Their three-person team manages approximately 20TB of ledger data, demonstrating how modern managed databases can dramatically reduce operational burden. The Books service uses three core tables: books (account metadata and cached balances), journal_entries (append-only transaction log), and book_entries (linking entries to accounts with monotonic version counters).

Adyen's architecture centers on a **stateless service-oriented design** they call PAL (Payments Acceptance Layer). Their critical insight is that all payments—whether from terminals, mobile apps, APIs, or hosted pages—should be abstracted and treated identically. Any PAL instance can shut down without impacting payment processing, new instances immediately handle full volume, and scaling is perfectly linear. For distributed idempotency, Adyen integrated **CockroachDB**, which allows payments to be processed by any machine without routing constraints while maintaining exactly-once semantics.

PayPal, while less transparent about internals, operates on Google Cloud infrastructure processing **1,000+ payments per second** during peak periods. Their architecture uses Aerospike for high-performance data storage and Apache TinkerPop with Gremlin for graph-based fraud detection queries.

---

## The payment processing pipeline: from validation to settlement

The standard order of operations follows a clear progression that separates authorization (the promise) from settlement (the money movement). First comes **validation and authentication**: card details verified, 3D Secure authentication triggered if required, fraud checks executed. Then **authorization** requests approval from the card issuer, who checks funds, account status, and fraud signals. A successful authorization places a hold on customer funds—critically, no money transfers yet. Authorization holds typically expire after **5-30 days** depending on the card network.

**Capture** follows authorization, either immediately (for digital goods) or delayed (for physical goods shipped later). This is when the merchant claims the previously authorized funds. Partial captures are supported, and platforms like Stripe and Adyen allow split instructions at capture time. Finally, **settlement** performs the actual fund transfer from issuing bank to merchant acquirer, typically in batches at end of day with **1-3 business day timelines**.

Stripe's PaymentIntent API models this as a unified state machine with states like `requires_payment_method`, `requires_confirmation`, `requires_action` (for 3DS), `processing`, `requires_capture`, and terminal states `succeeded` or `canceled`. Critically, **declined payments automatically return to `requires_payment_method`** rather than becoming a terminal "failed" state—this design acknowledges that a single payment intent may involve multiple attempts.

The separation between synchronous API responses and asynchronous processing is fundamental. The synchronous response confirms the request was received and initial validation passed, returning the current state. All subsequent state changes arrive via webhooks—payment completion, capture confirmation, settlement notification, or failure events. As Stripe's engineering team observed, **"Cards are the exception"**—most payment methods inherently require asynchronous handling through redirects, delayed finalization, or multi-step authentication.

---

## Idempotency as a first-class architectural concern

Large payment systems handle idempotency through the **idempotency key pattern** pioneered by Stripe. Clients include a unique key (typically UUID) with each request via an HTTP header. The server stores this key with the response after successful processing. Duplicate requests return the cached response immediately; in-progress duplicates return 409 Conflict. Keys expire after **24-48 hours** depending on the API version.

The implementation requires careful attention to failure scenarios. Brandur Leach (ex-Stripe) documented an "atomic phases" pattern using PostgreSQL: operations are wrapped in `SERIALIZABLE` transactions with **recovery points** stored alongside the idempotency key. Each phase handles either local database operations or external API calls (never both), enabling reliable resumption from any failure point. The schema stores request parameters, response code, response body, and the current recovery point (`started`, `ride_created`, `charge_created`, `finished`).

Airbnb built a generic idempotency framework called **Orpheus** using a three-phase request model: Pre-RPC (validation), RPC (external calls), and Post-RPC (recording results). The framework guarantees only two outcomes: definitive success or definitive failure—never ambiguous intermediate states.

---

## Solving the dual-write problem with the transactional outbox

The dual-write problem—needing to update a database AND publish an event atomically—has no solution through distributed transactions in practice. The industry-standard solution is the **transactional outbox pattern**: instead of publishing directly to Kafka, write events to an "outbox" table within the same database transaction as the business data.

```sql
BEGIN TRANSACTION;
INSERT INTO payments (id, amount, status) VALUES (...);
INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload) 
    VALUES ('Payment', payment_id, 'PaymentCreated', {...});
COMMIT;
```

A separate message relay process polls the outbox table or—preferably—uses **Change Data Capture (CDC) with Debezium** to stream changes directly from database transaction logs to Kafka. CDC adds minimal database load, captures changes in milliseconds, and maintains event ordering matching commit order. The critical caveat: the relay may publish events more than once if it crashes between publishing and marking as processed. **Consumers must therefore be idempotent**, tracking processed message IDs to handle duplicates gracefully.

For PostgreSQL-based systems, setting `REPLICA IDENTITY FULL` on payment tables ensures CDC events include previous values for all columns—essential for audit trails and debugging. Debezium configuration should include snapshot functionality for initial state capture and data masking for sensitive payment fields.

---

## Double-entry bookkeeping provides mathematical proof of correctness

Stripe's Ledger system processes **5 billion events daily** using double-entry bookkeeping, which they describe as providing "mathematical proof of correctness." Every transaction records at least two entries—a debit and corresponding credit—ensuring total debits always equal total credits. This constraint is self-auditing: imbalances immediately reveal problems.

The recommended database schema uses three tables. **Ledger accounts** store account metadata (type: ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE). **Journal entries** are the append-only transaction log with timestamps and descriptions. **Book entries** link journal entries to accounts, recording amount, direction (debit/credit), and a monotonic version counter for optimistic locking.

Stripe uses a "water flow" analogy: money flows through pipes (processes) into reservoirs (balance sheets). At steady state, terminal reservoirs are full and intermediate clearing accounts are empty. **Non-zero clearing account balances indicate unresolved issues**—this becomes a key monitoring metric. Their data quality platform tracks three metrics: clearing (did the fund flow complete?), timeliness (did data arrive on time?), and completeness (do we have all records?). They achieve **99.99% dollar volume verified within 4 days**.

Payment events translate to ledger entries through explicit mapping. A `charge.creation` event creates a pending balance in a `charge_undisbursed` account. When `charge.release` fires, funds move to the merchant's `business_balance` account—implemented as two entries: a debit (negative) to undisbursed and a credit (positive) to business balance. Airbnb extends this pattern with distinct handlers for **platform events** (reservations, check-ins), **payment events** (actual money movement), and **accounting events** (revenue recognition, receivable creation).

Balance calculation strategies vary between real-time updates (with optimistic locking via version columns) and batch computation from entries. The recommended hybrid approach stores immutable entries in an append-only log, maintains materialized balance snapshots cached for fast reads, and runs periodic reconciliation to ensure snapshot accuracy. Modern systems track multiple balance states: **settled balance** (confirmed transactions), **pending balance** (authorized but unsettled), and **available balance** (settled minus pending debits).

---

## Temporal workflows enable exactly-once payment processing

Temporal (and its predecessor Cadence, developed at Uber) powers payment workflows at **Stripe, Afterpay/Block, three of the five largest U.S. banks**, and major institutions across Europe and Asia-Pacific. Uber alone runs 12 billion workflow executions monthly across 1,000+ services. The key value proposition for payments is **durable execution with exactly-once semantics**: workflows survive process crashes, infrastructure failures, and deployments, automatically resuming from the last completed step.

The Saga pattern for distributed transactions becomes straightforward in Temporal. Each step registers a compensation action; if any step fails, compensations execute in reverse order. For a payment workflow: reserve inventory → add compensation to release inventory → charge payment → add compensation to refund → fulfill order. Failures trigger automatic rollback through the compensation chain.

Retry configuration belongs **at the activity level, not the workflow level**. Activities default to infinite retries with exponential backoff (initial interval 1 second, coefficient 2.0, maximum interval 100 seconds). Critically, **business logic failures should be marked as non-retryable**: `InsufficientFundsError`, `InvalidCardError`, and `FraudDetectedError` require business logic handling, not automated retry. The pattern is to classify errors as transient (retry with backoff) or permanent (return non-retryable error immediately).

Scaling Temporal workers in Kubernetes requires abandoning standard CPU/memory-based autoscaling. Workers spend significant time waiting on external operations, keeping CPU low while task latency climbs. **KEDA-based autoscaling using task queue backlog metrics** (`DescribeTaskQueueEnhanced` API) or schedule-to-start latency (`activity_schedule_to_start_latency`) provides accurate scaling signals. The Temporal Worker Controller enables automatic lifecycle scaling, managing different builds safely with traffic ramping and protecting pinned workflows from version changes.

---

## Kubernetes deployment patterns for payment services

Production payment services on Kubernetes require attention to connection pooling, secrets management, and graceful shutdown. **PgBouncer for PostgreSQL** is nearly universal, as Postgres uses a process-per-connection model where each connection consumes approximately 10MB even when idle. The recommended configuration uses **transaction pooling mode** (connections returned after each transaction) rather than session mode, with pool sizing calculated as `(replicasCount + 1) * poolSize` for maximum upstream connections.

Deployment options include PgBouncer as a sidecar container in each pod or as a dedicated pooler deployment. The sidecar pattern simplifies service discovery (applications connect to localhost) while dedicated pools reduce total connections to the database at the cost of additional network hops.

Secrets management for payment credentials follows the **HashiCorp Vault with Kubernetes authentication** pattern. Vault Agent sidecar injection automatically retrieves secrets and writes them to pod filesystems or environment variables. Dynamic database credentials—generated on-demand with automatic rotation and expiration—eliminate static credential risks. The External Secrets Operator provides an alternative for teams preferring declarative Kubernetes-native configuration, syncing secrets from Vault (or AWS Secrets Manager, GCP Secret Manager) to Kubernetes Secrets.

Payment-critical workloads should use **Guaranteed QoS** (requests equal limits) to prevent resource contention. Node autoscaling should be disabled for pooler pods to prevent disruptive scale-down events. All services require properly configured liveness and readiness probes—for database-connected services, probes should execute actual queries through connection pools to verify end-to-end health.

---

## Event streaming architecture with Kafka

Fintech companies use Kafka as the central nervous system for real-time payment processing. Transaction events from multiple channels (payment gateways, mobile banking, APIs) flow through central topics, with downstream microservices consuming, processing, and enriching data. Major UK banking groups process millions of transactions daily with millisecond latency using this pattern. Revolut built their entire payment platform on Kafka, enabling near-instant transfers for **15+ million customers**.

The key architectural benefit beyond throughput is **event replay capability**. Unlike traditional message queues where messages disappear after consumption, Kafka stores events for configurable retention periods. This enables debugging production issues by replaying events, reconstructing state after failures, and adding new consumers that process historical data.

**Circuit breakers require payment-specific thresholds**. Standard circuit breakers might trigger at 95% success rate—unacceptable for payments where even 0.5% failure rates during peak traffic cause significant revenue loss and customer frustration. Payment systems implement processor-specific circuit breakers: when Visa's API becomes slow, its circuit breaker opens while Mastercard processing continues unaffected.

Schema evolution uses **Avro with Schema Registry** in production deployments. Compatibility modes (BACKWARD, FORWARD, FULL, BACKWARD_TRANSITIVE) control what schema changes are permitted. For long-lived payment systems, BACKWARD_TRANSITIVE compatibility—requiring compatibility with ALL previous versions—provides the strongest guarantees. Safe changes include adding fields with default values and adding type aliases. Breaking changes like removing required fields or renaming fields without aliases should be avoided. Production deployments disable auto-registration (`auto.register.schemas=false`) and use CI/CD pipelines for controlled schema deployment.

---

## The most damaging architectural mistakes to avoid

The most insidious mistake is **handling only synchronous PSP responses**. Payment processors are distributed systems that may send both a synchronous response AND an asynchronous webhook for the same transaction. Without proper locking (`SELECT ... FOR UPDATE` on the payment record), concurrent processing of both responses causes double charges. This is not a theoretical concern—it's a common production incident pattern.

Building **card-first payment abstractions** creates technical debt that compounds over time. Stripe's engineering team acknowledged: "We built abstractions designed for the simplest payment method—cards. It's as if we were trying to build a spaceship by adding parts to a car." Payment methods like bank transfers, buy-now-pay-later, and cryptocurrency have fundamentally different lifecycles. The solution is separating Intent (what is paid), Method (how it's paid), and Order (what's purchased) from the beginning.

**Circular state machines** indicate a design flaw. When a payment can transition FAILED → PENDING → AUTHORIZED → FAILED, each cycle actually represents a new payment attempt that should be tracked as a distinct record. The correct pattern creates new attempt records with their own linear state machines, linking back to the parent payment intent.

The **distributed monolith** anti-pattern occurs when technically separate services remain logically entangled through shared DTO libraries and tight API contracts. Every change to the Payments service triggers cascading changes in Accounting and User Management. Similarly, **synchronous service chains** where checkout calls 6 services sequentially creates fragility—one network glitch in the fraud service causes complete checkout failure during Black Friday.

Race conditions cause duplicate payments or data loss. The classic pattern: two concurrent requests read balance ($100), both approve withdrawal ($80), both write new balance ($20)—resulting in $160 deducted against $80 actual balance. Solutions include pessimistic locking (`SELECT ... FOR UPDATE`), optimistic locking (version column checks), or atomic operations (`UPDATE accounts SET balance = balance - 80 WHERE balance >= 80`).

Finally, **"soft outages"** are as damaging as hard failures. A 200ms latency increase at a digital lender—where all dashboards showed green—distorted risk models and created multi-million dollar exposure. Payment systems require latency SLAs (typically <150ms for transactions) with alerts on percentile degradation, not just error rates.

---

## Conclusion: patterns that scale from startup to millions of transactions

The converging patterns from Stripe, Square, Adyen, and major banks provide a clear architectural blueprint. Start with **separated concerns**: Intent, Method, and Order as distinct entities. Implement **idempotency with atomic phases** from day one—this is not something that can be retrofitted easily. Use the **transactional outbox pattern** with CDC for reliable event publishing, avoiding the dual-write problem entirely. Build your ledger on **append-only double-entry bookkeeping** with clearing account monitoring. Deploy **Temporal for workflow orchestration** with activity-level retry configuration and KEDA-based worker scaling. Run **PgBouncer in transaction mode** for database connection management, and use **Vault with dynamic credentials** for secrets.

These patterns work at any scale because they're fundamentally sound—not because they add complexity for complexity's sake. A three-person team at Square manages 20TB of ledger data. Stripe's data quality platform achieves 99.999% automated monitoring. The key is implementing the patterns correctly from the start rather than bolting them on after production incidents force the issue.