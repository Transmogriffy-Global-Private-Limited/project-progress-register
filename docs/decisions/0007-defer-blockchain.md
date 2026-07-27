# ADR 0007 — Defer blockchain

Status: Accepted

Date: 2026-07-27

## Context

The product records evidence, hashes, revisions, assessments, and append-only audit events, which can appear suitable for blockchain. V1 nevertheless has one internal organization controlling the application, PostgreSQL, VPS, and signing material.

## Decision

Do not introduce blockchain or a distributed ledger in v1. Use PostgreSQL transactions and constraints, append-only histories, a least-privileged runtime role, server-authoritative timestamps, SHA-256 attachment hashes, and tested backups.

Preserve a future option to compute signed periodic Merkle-root manifests and store checkpoints outside the primary server. Only non-sensitive hashes could be anchored publicly.

## Consequences

- The system avoids consensus, node, wallet, fee, privacy, key-custody, and recovery complexity that provides no independent trust boundary today.
- Blockchain cannot turn spoofed browser evidence into true source evidence; this limitation remains explicit.
- Reconsideration requires multiple mutually distrustful writers or independent third-party verification without trust in the operator, plus an approved threat model and operational plan.
