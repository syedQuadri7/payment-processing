# Deferred Decisions

Enterprise-scale decisions that are beyond the scope of this learning project. These are documented here to understand what production systems need to address, but implementation is deferred.

---

## Why These Are Deferred

This is a learning project focused on understanding production patterns at a conceptual level. The decisions below require:

- Significant infrastructure investment
- Operational expertise that takes years to develop
- Scale that justifies the complexity
- Business requirements we don't have

For each deferred decision, we document:
- What the problem is
- What production systems typically do
- Why it's beyond learning scope

---

## DD-001: Database Sharding Strategy

**Originally:** PD-001

### Problem

As transaction volume grows, a single PostgreSQL instance becomes a bottleneck. Production systems processing millions of transactions need horizontal scaling.

### What Production Systems Do

| Approach | Used By | Trade-offs |
|----------|---------|------------|
| **Shard by Customer ID** | Stripe (early) | Simple queries but potential hot spots |
| **Shard by Payment ID** | Square | Even distribution but cross-shard customer queries |
| **Vitess/Citus** | Many | Transparent sharding with some SQL limitations |
| **Read Replicas First** | Most | Defer sharding until actually needed |

### Why Deferred

- Single PostgreSQL handles millions of transactions before sharding is needed
- Sharding decisions depend on actual access patterns we don't have
- This is a learning project, not a production system at scale
- Better to understand the patterns conceptually than implement prematurely

### Learning Takeaway

Start with a single database, add read replicas when needed, and only shard when you have data proving it's necessary. Most systems never reach sharding scale.

---

## DD-002: Multi-Currency Support

**Originally:** PD-002

### Problem

Supporting multiple currencies requires decisions about exchange rates, settlement currencies, and accounting complexity.

### What Production Systems Do

| Approach | Description | Used By |
|----------|-------------|---------|
| **Store Original + Base** | Keep both for flexibility | Stripe |
| **Original Only** | Convert at reporting time | Simpler systems |
| **Multi-ledger** | Separate ledgers per currency | Large banks |

Production systems also handle:
- Exchange rate sources and update frequency
- FX spread and markup
- Settlement currency preferences per merchant
- Regulatory requirements by region

### Why Deferred

- Single currency (USD) is sufficient for learning the core patterns
- Multi-currency adds significant complexity without teaching new concepts
- Exchange rate handling is a separate domain (not payment processing)
- Regulatory requirements vary by jurisdiction and change frequently

### Learning Takeaway

The core ledger patterns work the same regardless of currency. Multi-currency is mostly an extension of the same double-entry principles with additional conversion tracking.

---

## DD-003: Smart Routing Between Providers

**Originally:** PD-003

### Problem

With multiple payment providers, intelligent routing can optimize cost, success rates, and redundancy.

### What Production Systems Do

| Routing Strategy | Description |
|------------------|-------------|
| **Static Rules** | EU cards to Adyen, US cards to Stripe |
| **Cost-Based** | Route to cheapest provider for transaction type |
| **Success-Rate** | Route based on historical success by card type/region |
| **ML-Based** | Predict best provider per transaction |
| **Cascading** | Failover to backup provider on failure |

### Why Deferred

- Routing optimization requires historical transaction data we don't have
- ML-based routing needs data science infrastructure
- The learning value is in understanding multi-provider architecture, not optimization
- Simple static routing or round-robin is sufficient for learning

### Learning Takeaway

The adapter pattern we've implemented supports any routing strategy. The core architecture doesn't change - only the provider selection logic does.

---

## DD-004: Reporting and Analytics

**Originally:** PD-014

### Problem

Merchants need insights into their payment data through reports and dashboards.

### What Production Systems Do

| Component | Description |
|-----------|-------------|
| **Pre-built Reports** | Daily summary, monthly statement, transaction export |
| **Custom Reporting** | Query builder, scheduled reports |
| **Real-time Dashboards** | Current volume, success rates, errors |
| **BI Integration** | Looker, Tableau, custom exports |
| **Reconciliation** | Bank statement matching, fee reconciliation |

### Why Deferred

- Reporting is a consumer of payment data, not core payment processing
- Would require building a separate reporting service
- Dashboard/BI work is frontend and visualization focused
- The outbox events we publish provide the data; reporting consumes it

### Learning Takeaway

The transactional outbox pattern ensures all payment events are available for downstream consumers. Reporting services subscribe to these events and build their own read models.

---

## DD-005: Merchant Onboarding and KYC

**Originally:** PD-015

### Problem

New merchants need verification (KYC/KYB) before processing payments.

### What Production Systems Do

| Component | Description |
|-----------|-------------|
| **Identity Verification** | Document verification, selfie matching |
| **Business Verification** | Business registration, beneficial owners |
| **Risk Scoring** | Automated risk assessment |
| **Manual Review** | Human review for edge cases |
| **Ongoing Monitoring** | Transaction pattern monitoring |

### Why Deferred

- KYC/KYB is a regulatory and compliance domain, not payment processing
- Requires integration with identity verification providers
- Compliance requirements vary by jurisdiction
- This project assumes merchants are already onboarded

### Learning Takeaway

Payment processing systems typically receive merchant_id as input, assuming onboarding happened elsewhere. The integration point is merchant configuration (API keys, webhook URLs, routing preferences).

---

## DD-006: Service Level Agreements

**Originally:** PD-016

### Problem

Production systems need defined SLAs for uptime, latency, and support response times.

### What Production Systems Do

| Metric | Typical SLA |
|--------|-------------|
| **Uptime** | 99.95% - 99.99% |
| **API Latency (p99)** | < 500ms |
| **Webhook Delivery** | < 5 seconds |
| **Support Response** | 15 min (critical), 4 hours (high) |

### Why Deferred

- SLAs require production operations experience
- Meaningful SLAs need historical performance data
- Compensation structures require business decisions
- This is a learning project, not a production service

### Learning Takeaway

SLAs are important for production systems but are operational commitments, not architectural patterns. The patterns we're learning (idempotency, durability, observability) are what make SLAs achievable.

---

## DD-007: Data Residency and Compliance

**Originally:** PD-017

### Problem

Different regions have different data handling requirements (GDPR, data localization laws).

### What Production Systems Do

| Requirement | Approach |
|-------------|----------|
| **GDPR** | Data subject access/deletion, consent tracking |
| **Data Residency** | Region-specific deployments |
| **PCI-DSS** | Cardholder data isolation, audit trails |
| **Cross-border** | Data transfer agreements, SCCs |

### Why Deferred

- Compliance requirements are jurisdiction-specific
- Data residency requires multi-region infrastructure
- PCI compliance has specific implementation requirements
- Compliance is an ongoing operational concern, not a one-time implementation

### Learning Takeaway

The patterns we're learning (audit trails, immutable ledgers, outbox for event sourcing) are foundational for compliance. But actual compliance requires operational processes, not just code.

---

## When These Become Relevant

These decisions become relevant when:

| Trigger | Decisions to Revisit |
|---------|----------------------|
| Processing > 1M transactions/month | Sharding (DD-001) |
| Expanding to new countries | Multi-currency (DD-002), Data residency (DD-007) |
| Multiple providers in production | Smart routing (DD-003) |
| External merchants using the API | KYC (DD-005), SLAs (DD-006), Reporting (DD-004) |

---

## See Also

- [Finalized Decisions](finalized-decisions.md) - Decisions resolved with simple defaults
- [Pending Decisions](pending-decisions.md) - Decisions still needing input
- [ADR Index](readme.md) - Architectural decisions that have been implemented
