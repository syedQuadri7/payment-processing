# Terms and Definitions

Reference glossary for payment processing terminology, technical concepts, and project-specific terms.

---

## Payment Industry Terms

### Authorization

The process of verifying that a cardholder has sufficient funds or credit and approving a transaction. Authorization places a **hold** on funds but does not transfer money. The authorization must be **captured** to complete the transaction.

### Capture

The process of claiming previously authorized funds. Capture initiates the actual transfer of money from the cardholder's account. Can be **automatic** (immediate after authorization) or **manual** (merchant triggers later).

### Void

Canceling an authorization before capture. Releases the hold on the cardholder's funds. No money moves and typically no fees are charged.

### Refund

Returning funds to a cardholder after a capture has occurred. Creates a credit transaction that reverses all or part of the original charge. May incur processing fees even though the original transaction is reversed.

### Settlement

The actual transfer of funds between banks. Occurs in batches, typically daily. The time between capture and settlement is the **settlement period**.

### Chargeback

A dispute initiated by a cardholder through their bank. The bank reverses the transaction and debits the merchant. Merchants can contest chargebacks by submitting evidence.

### Dispute

A broader term for any contested transaction. Includes chargebacks, inquiries, and pre-arbitration cases. Different from a refund because the customer initiates it through their bank, not the merchant.

### Decline

When a payment authorization is rejected. Can be **soft decline** (temporary, retry may succeed) or **hard decline** (permanent, retry will fail).

### Soft Decline

A temporary authorization failure that may succeed on retry. Examples: insufficient funds, temporary hold, processing error. The system should schedule automatic retries.

### Hard Decline

A permanent authorization failure that will not succeed on retry. Examples: invalid card number, expired card, stolen card. The system should not retry and should request a new payment method.

### Interchange

Fees paid between banks for processing card transactions. Set by card networks (Visa, Mastercard). Varies by card type, merchant category, and transaction type. A major component of payment processing costs.

### Payment Intent

An object representing the customer's intention to pay a specific amount. Tracks the payment through its lifecycle from creation to completion. Our primary payment entity.

### Payment Method

A tokenized reference to how the customer will pay. Examples: stored card token, bank account reference, digital wallet. Contains no sensitive data directly—only references to tokens held by providers.

### Token / Tokenization

Replacing sensitive payment data (card numbers) with non-sensitive placeholders (tokens). Tokens can be used for transactions but cannot be reverse-engineered to obtain the original data. Essential for PCI compliance.

### PAN

Primary Account Number. The 15-19 digit number on a payment card. We never store PANs—only tokens.

### CVV / CVC / CVV2

Card Verification Value. The 3-4 digit security code on a card. We never store CVV values. Used only during initial authorization.

### BIN

Bank Identification Number. The first 6-8 digits of a card number that identify the issuing bank. Can be used for routing decisions without storing full card data.

### Acquirer

The bank or financial institution that processes payments on behalf of a merchant. Also called the **acquiring bank** or **merchant bank**.

### Issuer

The bank that issued the payment card to the cardholder. Also called the **issuing bank** or **card issuer**. Approves or declines authorization requests.

### Card Network

Organizations that facilitate transactions between acquirers and issuers. Examples: Visa, Mastercard, American Express, Discover. Set rules and interchange rates.

### PSP (Payment Service Provider)

A company that provides payment processing services. Examples: Stripe, Adyen, PayPal. Also called **payment gateway** or **payment processor**. We integrate with multiple PSPs.

### Merchant of Record

The entity that appears on the customer's statement and is responsible for the transaction. Affects liability for chargebacks and refunds.

### Dunning

The process of attempting to collect failed payments through retries. A **dunning window** is the period during which retries are attempted.

### 3D Secure (3DS)

An authentication protocol that adds a verification step during online payments. Reduces fraud and shifts chargeback liability to the issuer. Examples: Visa Secure, Mastercard Identity Check.

### SCA (Strong Customer Authentication)

A European regulatory requirement (PSD2) for two-factor authentication on electronic payments. Requires two of: something you know, something you have, something you are.

---

## Technical Architecture Terms

### Temporal

An open-source workflow orchestration platform. Provides durable execution, meaning workflows survive process crashes and restarts. We use Temporal to manage payment workflows.

### Workflow

In Temporal, a function that orchestrates activities and defines the payment flow. Workflows must be **deterministic**—given the same inputs and history, they produce the same outputs.

### Activity

In Temporal, a function that performs a single unit of work with side effects. Activities handle external calls (PSP APIs, database writes). Activities can be retried independently.

### Signal

In Temporal, a way to send data to a running workflow from outside. We use signals to deliver webhook events to payment workflows.

### Query

In Temporal, a way to read workflow state without affecting it. We use queries to expose payment status to the API.

### Durable Execution

The property that code execution can be paused and resumed, surviving crashes and restarts. Temporal provides durable execution through event sourcing of workflow state.

### Durable Timer

A timer that survives process restarts. In Temporal, `workflow.Sleep()` creates a durable timer that can span days without the worker running continuously.

### Task Queue

In Temporal, a named queue that workers poll for work. Workers register for specific task queues. We use a single `payment-processing` task queue.

### Idempotency

The property that performing an operation multiple times produces the same result as performing it once. Critical for payment systems to prevent duplicate charges.

### Idempotency Key

A unique identifier provided by the client to ensure an operation is only performed once. If a request is retried with the same key, the original result is returned.

### Exactly-Once Semantics

The guarantee that a message or operation is processed exactly one time, not zero, not more. Achieved through **at-least-once delivery** combined with **idempotent processing**.

### At-Least-Once Delivery

The guarantee that a message will be delivered one or more times. May result in duplicates. Combined with idempotency to achieve exactly-once semantics.

### CDC (Change Data Capture)

A technique for capturing changes to data as they occur. We use CDC to stream database changes to Kafka. Debezium reads PostgreSQL's WAL to capture changes.

### WAL (Write-Ahead Log)

A log where database changes are written before being applied to data files. PostgreSQL's WAL enables point-in-time recovery and CDC. Debezium reads the WAL to capture changes.

### Transactional Outbox

A pattern for reliable event publishing. Events are written to an **outbox table** in the same transaction as business data, then published asynchronously. Solves the dual-write problem.

### Dual-Write Problem

The challenge of atomically updating a database AND publishing an event. If one succeeds and the other fails, systems become inconsistent. The transactional outbox pattern solves this.

### Debezium

An open-source CDC platform. Reads database change logs and publishes to Kafka. We use Debezium to publish outbox events.

### Kafka

A distributed event streaming platform. Provides durable, ordered message delivery. We use Kafka as the transport for CDC events.

### Optimistic Locking

A concurrency control strategy where conflicts are detected at update time rather than prevented with locks. Uses a **version** column; updates fail if the version has changed.

### Double-Entry Bookkeeping

An accounting method where every transaction affects at least two accounts with equal debits and credits. Provides mathematical proof that books balance.

### Ledger

An accounting record of all transactions. In double-entry bookkeeping, the ledger contains all debit and credit entries.

### Journal Entry

A record of a single transaction in the ledger. Groups related debit and credit entries that must balance.

### Debit

In double-entry bookkeeping, an entry that increases asset/expense accounts or decreases liability/equity/revenue accounts. Always on the left side of a journal entry.

### Credit

In double-entry bookkeeping, an entry that increases liability/equity/revenue accounts or decreases asset/expense accounts. Always on the right side of a journal entry.

### Clearing Account

A temporary account used to track in-flight transactions. Should trend toward zero at steady state. Non-zero balances indicate transactions needing attention.

---

## Canonical Model Terms

### Canonical Event

A standardized event format used internally, independent of payment provider. Provider-specific events are **normalized** to canonical events at the edge.

### Canonical Decline Code

A standardized decline reason used internally. Provider-specific decline codes are **mapped** to canonical codes. Enables consistent retry logic across providers.

### Adapter

A component that translates between provider-specific formats and canonical formats. Each payment provider has its own adapter. Adapters handle signature verification, event parsing, and code mapping.

### Normalize

The process of converting provider-specific data to canonical format. Happens at the **edge** (in adapters) so the core system only sees canonical data.

---

## Provider-Specific Terms

### Stripe

A payment service provider. Uses **Payment Intents** as their primary payment object. Webhooks are signed with HMAC-SHA256 including a timestamp.

### Adyen

A payment service provider popular in Europe. Sends webhook notifications in batches. Uses **pspReference** as the unique transaction identifier.

### PayPal

A payment service provider and digital wallet. Webhook verification requires calling the PayPal API (not local signature verification). Uses **invoice_id** for correlation.

### Webhook

An HTTP callback from a provider to notify us of events. Providers send webhooks when payment status changes. Must be verified using provider-specific signature methods.

### Signature Verification

The process of confirming a webhook is authentic and from the claimed provider. Methods vary: HMAC-SHA256 (Stripe, Adyen), API verification (PayPal).

### HMAC

Hash-based Message Authentication Code. A cryptographic method to verify both data integrity and authenticity. We verify provider webhooks using HMAC-SHA256.

---

## Database Terms

### UUID

Universally Unique Identifier. A 128-bit identifier used for primary keys. Prevents enumeration attacks and supports distributed ID generation.

### DECIMAL(19,4)

A precise numeric type for monetary values. 19 total digits with 4 decimal places. Avoids floating-point precision issues with money.

### REPLICA IDENTITY FULL

A PostgreSQL setting that includes all column values in WAL for UPDATE and DELETE operations. Required for CDC to capture complete row data.

### Foreign Key

A column that references a primary key in another table. Enforces referential integrity. Example: `payment_intent_id` in `authorization_holds` references `payment_intents.id`.

### Index

A database structure that speeds up queries on specific columns. We create indexes on frequently queried columns like `customer_id`, `status`, and `created_at`.

### Migration

A versioned change to the database schema. Migrations are applied in order and should be reversible. Managed by migration tools.

---

## State Machine Terms

### State Machine

A model where an entity exists in one of several defined **states** and transitions between states based on **events**. Payment intents follow a state machine.

### Terminal State

A state from which no further transitions are possible. Examples: CAPTURED, VOIDED, FAILED, CANCELLED. Once a payment reaches a terminal state, it cannot change.

### State Transition

Moving from one state to another based on an event. Example: AUTHORIZED → CAPTURED when capture succeeds. Invalid transitions should be rejected.

### Linear State Machine

A state machine where states only move forward, never backward. Each retry attempt creates a new record rather than transitioning back. Simplifies reasoning and creates better audit trails.

---

## Compliance and Security Terms

### PCI-DSS

Payment Card Industry Data Security Standard. A set of security requirements for handling cardholder data. Compliance levels range from 1 (highest) to 4.

### PII

Personally Identifiable Information. Data that can identify an individual. Examples: name, email, address. Must be handled according to privacy regulations.

### GDPR

General Data Protection Regulation. European privacy law governing personal data handling. Requires consent, data minimization, and honoring data subject requests.

### KYC

Know Your Customer. The process of verifying merchant identity before allowing them to process payments. Required by financial regulations.

### SOC 2

Service Organization Control 2. An audit standard for service providers storing customer data. Covers security, availability, processing integrity, confidentiality, and privacy.

### mTLS

Mutual TLS. A security protocol where both client and server authenticate each other using certificates. Stronger than one-way TLS.

---

## Acronyms Quick Reference

| Acronym | Full Term |
|---------|-----------|
| ACH | Automated Clearing House |
| API | Application Programming Interface |
| BIN | Bank Identification Number |
| CDC | Change Data Capture |
| CVV | Card Verification Value |
| GDPR | General Data Protection Regulation |
| HMAC | Hash-based Message Authentication Code |
| KYC | Know Your Customer |
| mTLS | Mutual Transport Layer Security |
| PAN | Primary Account Number |
| PCI | Payment Card Industry |
| PII | Personally Identifiable Information |
| PSP | Payment Service Provider |
| SCA | Strong Customer Authentication |
| SLA | Service Level Agreement |
| SOC | Service Organization Control |
| TLS | Transport Layer Security |
| UUID | Universally Unique Identifier |
| WAL | Write-Ahead Log |
| 3DS | 3D Secure |

---

## Project-Specific Terms

### Payment Processing Service

The system we are building. Handles payment orchestration across multiple providers with double-entry bookkeeping.

### Core System

The provider-agnostic parts of the codebase: workflows, ledger, outbox. Never contains provider-specific logic.

### Adapter Layer

The provider-specific parts of the codebase. Handles webhook verification, event mapping, and API calls for each provider.

### Recovery Workflow

A child workflow spawned when a soft decline occurs. Manages the retry schedule and attempts to recover the payment.

### Outbox Event

A record written to the outbox table as part of a business transaction. Published to Kafka via CDC. Contains only canonical data.

### Decline Code Mapping

A database table that maps provider-specific decline codes to canonical decline codes and decline types. Can be updated without code changes.
