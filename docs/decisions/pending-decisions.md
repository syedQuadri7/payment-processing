# Pending Decisions

Decisions that require input before implementation can proceed. Please review and provide direction.

---

## PD-001: Database Sharding Strategy

### Problem

As transaction volume grows, a single PostgreSQL instance will become a bottleneck. We need a strategy for horizontal scaling of the database layer.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Shard by Customer ID** | All data for a customer lives on one shard. Simple queries but potential hot spots for high-volume customers. |
| **B: Shard by Payment Intent ID** | Distributes load evenly but cross-shard queries needed for customer history. |
| **C: Citus Extension** | Use Citus for transparent sharding within PostgreSQL. Maintains SQL compatibility but adds operational complexity. |
| **D: Defer Decision** | Start with single instance, add read replicas, decide sharding strategy when actual bottlenecks observed. |

### Questions to Answer

1. What is the expected transaction volume at launch?
2. What is the projected transaction volume at 1 year? At 3 years?
3. Are there known high-volume customers that would create hot spots?
4. Is operational simplicity or maximum scalability more important initially?
5. Do we have Citus expertise on the team or budget for training?
6. What is the acceptable query latency for customer history lookups?
7. Will we need cross-customer reporting queries?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-002: Multi-Currency Support

### Problem

The system currently assumes single-currency operations. Supporting multiple currencies requires decisions about exchange rates, settlement currencies, and display formatting.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Store in Original Currency** | Keep amounts in the currency they were charged. Convert only for reporting. |
| **B: Convert to Base Currency** | Convert all amounts to a base currency (USD) at transaction time. Simpler ledger but exchange rate risk. |
| **C: Dual Storage** | Store both original currency and converted base currency amounts. More storage but maximum flexibility. |

### Questions to Answer

1. What currencies do we need to support at launch?
2. What currencies do we anticipate adding in the next 2 years?
3. Where do exchange rates come from? (Internal system, third-party API, manual entry?)
4. How often should exchange rates be updated? (Real-time, daily, weekly?)
5. Do merchants settle in their local currency or a single currency?
6. How should exchange rate gains/losses be recorded in the ledger?
7. What is our base/reporting currency?
8. Do we need to support currency conversion fees?
9. How should amounts be displayed to customers vs. merchants vs. internal reporting?
10. Are there regulatory requirements for currency handling in target markets?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-003: Smart Routing Between Providers

### Problem

With multiple payment providers, we need logic to decide which provider handles each transaction. This affects success rates, costs, and customer experience.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Static Rules** | Configure rules like "EU cards go to Adyen, US cards go to Stripe". Simple but inflexible. |
| **B: Cost-Based Routing** | Route to provider with lowest fees for each transaction type. Optimizes cost but ignores success rates. |
| **C: Success-Rate Routing** | Route based on historical success rates by card type, region, amount. Better conversion but complex. |
| **D: ML-Based Routing** | Machine learning model predicts best provider per transaction. Maximum optimization but requires significant data and infrastructure. |

### Questions to Answer

1. What are the primary routing goals? Rank in order: cost optimization, success rate, latency, redundancy
2. Do we have historical transaction data to inform routing decisions?
3. What fallback behavior when primary provider fails? (Immediate failover, queue for retry, fail the transaction?)
4. Should merchants be able to configure their own routing preferences?
5. Should merchants be able to exclude specific providers?
6. Do we need geographic routing (e.g., EU data stays in EU)?
7. Are there transaction types that must go to specific providers?
8. How do we handle provider outages? Manual switch or automatic detection?
9. What metrics should we track to evaluate routing effectiveness?
10. Should routing decisions be logged for audit purposes?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-004: Retry Timing Strategy

### Problem

When a soft decline occurs, we retry the payment. The timing of retries affects success rates. Industry research suggests aligning retries with paydays improves success.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Fixed Intervals** | Retry at fixed intervals (1 day, 3 days, 7 days). Simple but not optimized. |
| **B: Exponential Backoff** | Increasing delays between retries. Standard pattern but not payment-optimized. |
| **C: Payday-Aligned** | Retry on common paydays (1st, 15th of month, Fridays). Higher success but complex scheduling. |
| **D: Configurable per Merchant** | Let merchants configure retry schedules. Maximum flexibility but more complexity. |

### Questions to Answer

1. What is the maximum dunning window (how long to keep retrying)?
2. What is the maximum number of retry attempts?
3. Should retry timing differ by decline reason? (e.g., insufficient funds vs. temporary error)
4. Are there regulatory limits on retry frequency we need to comply with?
5. Should customers be notified before each retry attempt?
6. Should customers be able to update their payment method during the dunning period?
7. Do we know our customers' typical pay schedules?
8. Should we allow merchants to pause/resume dunning?
9. What happens when dunning exhausts all retries? (Cancel subscription, downgrade, notify?)
10. Should we track retry success rates by timing to optimize the schedule?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-005: Webhook Delivery Guarantees

### Problem

We publish events via CDC to Kafka. Downstream consumers need to receive these events reliably. We need to decide what delivery guarantees we provide and how external parties receive events.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Kafka Only** | Internal consumers read from Kafka. No external webhook delivery. |
| **B: Webhook Endpoints** | Push events to consumer-configured HTTP endpoints with retry. |
| **C: Both Kafka and Webhooks** | Kafka for internal consumers, webhooks for external. |
| **D: Webhook with Polling Fallback** | Webhooks as primary, but also expose API to poll for missed events. |

### Questions to Answer

1. Who are the consumers of payment events? (Internal services only, external merchants, both?)
2. What SLA do consumers expect for event delivery latency?
3. What is the acceptable event delivery failure rate?
4. Should consumers be able to replay historical events?
5. How far back should event replay be available?
6. What authentication should webhook endpoints use? (Signature verification, mTLS, API key?)
7. How many retry attempts for failed webhook delivery?
8. What is the retry backoff strategy for webhooks?
9. Should we support webhook filtering (only certain event types)?
10. Do we need to support multiple webhook endpoints per merchant?
11. How do we handle webhook endpoint changes? (Immediate switch, parallel delivery during transition?)

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-006: Idempotency Key Management

### Problem

Idempotency keys prevent duplicate payment processing. We need policies for key format, storage, expiration, and collision handling.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Client-Provided Keys** | Clients must provide idempotency keys. We validate format and uniqueness. |
| **B: Server-Generated Keys** | Server generates keys, returns to client for reference. |
| **C: Hybrid** | Accept client keys if provided, generate if not. |

### Questions to Answer

1. What format should idempotency keys follow? (UUID, custom format, any string?)
2. What is the maximum length for idempotency keys?
3. How long should idempotency keys be stored before expiration?
4. What happens when a key collision occurs with different request parameters?
5. Should we return the original response or an error on collision?
6. Are idempotency keys scoped per merchant or global?
7. Should we support idempotency for all endpoints or only payment mutations?
8. How do we handle idempotency for async operations (workflow started but not completed)?
9. Should idempotency key lookups be cached for performance?
10. What error message should be returned for expired idempotency keys?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-007: Payment Method Storage and Tokenization

### Problem

We need to store payment method references securely. Decisions needed around token lifecycle, multi-provider tokens, and customer data handling.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Provider Tokens Only** | Store only provider-issued tokens. Simple but provider-locked. |
| **B: Internal Token Layer** | Create our own tokens that map to provider tokens. Abstraction but more complexity. |
| **C: Vault Service** | Use dedicated vault service for all sensitive data. Maximum security but infrastructure cost. |

### Questions to Answer

1. Should a single payment method be usable across multiple providers?
2. How do we handle payment method expiration? (Proactive notification, fail on use, auto-update?)
3. What customer data do we store alongside the token? (Name, billing address, last four digits?)
4. How long do we retain payment methods after last use?
5. Should customers be able to delete their payment methods?
6. Do we need to support payment method sharing across merchant accounts?
7. What PCI compliance level are we targeting?
8. Should we support network tokenization (card-on-file tokens from Visa/Mastercard)?
9. How do we handle provider token format changes?
10. What happens to stored tokens if we stop using a provider?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-008: Authorization Hold Expiration

### Problem

Authorization holds have limited validity periods that vary by provider and card network. We need policies for monitoring, extending, and handling expired holds.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Use Provider Defaults** | Accept each provider's default hold period. Simple but inconsistent. |
| **B: Standardize to Shortest** | Use the shortest common hold period (3-5 days). Consistent but limiting. |
| **C: Configurable per Transaction** | Allow specifying hold period per transaction. Flexible but complex. |

### Questions to Answer

1. What is our typical time between authorization and capture?
2. Do we need holds longer than 7 days? What use cases?
3. Should we automatically void holds approaching expiration?
4. Should we attempt to re-authorize before hold expiration?
5. How do we notify merchants of approaching hold expiration?
6. What happens if capture is attempted on an expired hold?
7. Should we support hold extension where providers allow it?
8. How do we handle partial captures? (Remaining hold released or maintained?)
9. Should we track hold expiration per card network (Visa vs. Mastercard rules differ)?
10. What is the customer experience when a hold expires before capture?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-009: Dispute and Chargeback Handling

### Problem

Chargebacks require evidence submission, deadline tracking, and outcome recording. We need to define our role in the dispute process.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Notification Only** | We notify merchant of disputes, they handle directly with provider. |
| **B: Evidence Collection** | We provide tools to collect and submit evidence through our platform. |
| **C: Full Management** | We manage the entire dispute lifecycle including automatic evidence submission. |

### Questions to Answer

1. What dispute notification latency is acceptable?
2. Should we automatically pause related subscriptions during disputes?
3. What evidence types should we collect and store? (Transaction logs, delivery confirmation, customer communication?)
4. Should we provide dispute analytics and trends?
5. What is the merchant's role vs. our role in evidence submission?
6. Should we support pre-dispute alerts (like Visa's VMPI)?
7. How do we handle disputes that span multiple partial captures?
8. Should we integrate with chargeback management services (Chargebacks911, etc.)?
9. What reporting do merchants need for dispute tracking?
10. How do we record dispute outcomes in the ledger?
11. Should we support automatic refund to avoid disputes?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-010: Rate Limiting and Throttling

### Problem

We need to protect the system from abuse and ensure fair resource allocation across merchants.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Global Limits** | Same rate limits for all merchants. Simple but unfair to high-volume merchants. |
| **B: Tiered Limits** | Different limits based on merchant tier/plan. Fair but requires tier management. |
| **C: Dynamic Limits** | Adjust limits based on current system load. Optimal utilization but unpredictable for merchants. |

### Questions to Answer

1. What rate limits should apply to payment creation?
2. What rate limits should apply to read operations (status queries)?
3. Should rate limits be per-merchant, per-API-key, or per-endpoint?
4. How should rate limit errors be communicated? (HTTP 429, custom error codes?)
5. Should we support rate limit headers (X-RateLimit-Remaining, etc.)?
6. What happens when a merchant consistently hits rate limits?
7. Should high-value transactions bypass rate limits?
8. How do we handle burst traffic (e.g., flash sales)?
9. Should webhooks from providers have separate limits?
10. Do we need to rate limit by IP address for DDoS protection?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-011: Testing and Sandbox Environment

### Problem

Merchants need to test their integration without processing real payments. We need to define sandbox behavior and test data management.

### Options Under Consideration

| Option | Description |
|--------|-------------|
| **A: Separate Sandbox** | Completely separate environment with isolated data. |
| **B: Test Mode Flag** | Same environment but test_mode flag changes behavior. |
| **C: Provider Sandboxes** | Pass through to each provider's sandbox/test mode. |

### Questions to Answer

1. Should sandbox have the same API as production?
2. What test card numbers should trigger specific outcomes? (Success, various declines, fraud)
3. Should sandbox transactions appear in reporting?
4. How do we handle webhook testing in sandbox?
5. Should sandbox have rate limits?
6. How long should sandbox data be retained?
7. Can merchants copy sandbox configurations to production?
8. Should sandbox support all providers or a subset?
9. How do we simulate provider outages for testing?
10. Should we provide pre-built test scenarios (happy path, decline, dispute)?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-012: Audit Log Retention and Access

### Problem

Audit logs are required for compliance and debugging. We need policies for retention, access control, and archival.

### Questions to Answer

1. What is the minimum audit log retention period for compliance? (Often 7 years for financial)
2. Who should have access to audit logs? (Roles/permissions)
3. Should audit logs be searchable through the API?
4. Should we support audit log export for compliance audits?
5. How should PII in audit logs be handled? (Masking, encryption, separate storage?)
6. Should audit logs include request/response bodies?
7. What is the archival strategy? (Hot storage duration, cold storage format)
8. Should audit logs be immutable? (Write-once storage)
9. How do we handle audit log requests from law enforcement?
10. Should we provide audit log alerts for suspicious activity?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-013: Notification and Communication Channels

### Problem

Various events require notifying merchants, customers, or internal teams. We need to define notification channels and preferences.

### Questions to Answer

1. What events require merchant notification? (Disputes, large transactions, failures?)
2. What events require customer notification? (Receipt, decline, upcoming retry?)
3. What communication channels should we support? (Email, SMS, webhook, in-app?)
4. Should notification preferences be configurable per merchant?
5. Should notification preferences be configurable per event type?
6. What is the notification latency SLA?
7. How do we handle notification delivery failures?
8. Should we provide notification templates or allow merchant customization?
9. What branding options should be available for customer-facing notifications?
10. Do we need to support multiple languages for notifications?
11. Should notifications include transaction details or link to portal?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-014: Reporting and Analytics

### Problem

Merchants need insights into their payment data. We need to define what reports and analytics we provide.

### Questions to Answer

1. What standard reports should be available? (Daily summary, monthly statement, etc.)
2. What metrics should the dashboard display?
3. Should we support custom report building?
4. What export formats should be available? (CSV, PDF, Excel?)
5. How far back should historical data be queryable?
6. Should reports be scheduled or on-demand?
7. What granularity of data should be exposed? (Transaction-level, aggregated?)
8. Should we provide benchmarking against industry averages?
9. What reconciliation reports are needed for finance teams?
10. Should we integrate with business intelligence tools? (Looker, Tableau, etc.)

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-015: Merchant Onboarding and KYC

### Problem

New merchants need to be onboarded and verified before processing payments. We need to define the onboarding flow and verification requirements.

### Questions to Answer

1. What information is required during merchant signup?
2. What KYC (Know Your Customer) verification is required?
3. Should we integrate with identity verification services?
4. What documents need to be collected? (Business license, bank statements, etc.)
5. Should there be a manual review step or fully automated?
6. What are the criteria for automatic approval vs. manual review?
7. How long should onboarding take? (SLA)
8. Should merchants be able to process test transactions before full approval?
9. What ongoing monitoring is required after onboarding?
10. How do we handle merchant account suspension or termination?
11. What is the appeals process for rejected applications?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-016: Service Level Agreements

### Problem

We need to define and commit to service levels for availability, latency, and support response times.

### Questions to Answer

1. What is our target uptime SLA? (99.9%, 99.95%, 99.99%?)
2. What is the target API response latency? (p50, p95, p99)
3. What is the target webhook delivery latency?
4. How do we define "downtime"? (Complete outage, degraded performance, single endpoint?)
5. What compensation do we offer for SLA breaches?
6. What is the support response time SLA by severity?
7. What maintenance windows are acceptable?
8. How much notice for planned maintenance?
9. Should SLAs differ by merchant tier?
10. How do we communicate outages and incidents?

### Your Input

> _[Please provide your answers and decision here]_

---

## PD-017: Data Residency and Compliance

### Problem

Different regions have different data handling requirements. We need policies for data residency, cross-border transfers, and compliance certifications.

### Questions to Answer

1. What geographic regions do we need to support?
2. Do we need region-specific data storage? (EU data in EU, etc.)
3. What compliance certifications are required? (PCI-DSS level, SOC 2, ISO 27001?)
4. How do we handle GDPR data subject requests? (Access, deletion, portability)
5. Do we need to support data localization requirements? (Russia, China, etc.)
6. What is our data processing agreement template?
7. How do we handle cross-border data transfers?
8. What audit artifacts do we need to maintain?
9. How often should compliance audits be conducted?
10. Who is responsible for compliance monitoring?

### Your Input

> _[Please provide your answers and decision here]_

---

## How to Add a Pending Decision

1. Add a new section with the next PD number
2. Describe the problem clearly
3. List options with descriptions (not recommendations yet)
4. List questions that need answers
5. Optionally add a current recommendation if you have one

## How to Resolve a Pending Decision

1. Answer the questions with stakeholder input
2. Make a decision
3. Create a new ADR documenting the decision
4. Remove from this file
5. Update decisions/readme.md with the new ADR summary
