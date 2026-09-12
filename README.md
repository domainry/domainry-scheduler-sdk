# Domainry Scheduler SDK

This repository contains the deployment-neutral Scheduler protocol:

- Go contracts for definitions, schedules, triggers, receipts, Module hosts and SaaS transports.
- Owner-scoped scheduled plan records for one-time and recurring product work.
- `schedule` for deterministic authoring validation, recurrence planning and window keys shared by Module, SaaS and Runtime compatibility paths.
- `saashost/httptransport` for authenticated Runtime-to-SaaS communication.
- `dispatchgateway` for HMAC-signed Scheduler-service-to-Runtime target execution.
- `@domainry/scheduler-client` in `browser/` for the Admin Console.

Runtime supplies published configuration and downstream owner capabilities through the SDK. The Scheduler implementation remains replaceable between Module and SaaS without changing business handlers or frontend request semantics.

## Package layout

- The root package is the stable Scheduler `Factory`, `Binding`, definition, trigger, receipt, run-history, and dead-letter entrypoint. Run and dead-letter queries read Scheduler-owned operational state; hosts do not project those rows into Runtime records.
- `persistence` owns durable definition projection contracts.
- `modulehost` describes embedded host infrastructure, providers, dispatcher, and run-store capabilities.
- `saashost` and `saashost/httptransport` describe authenticated SaaS composition.
- `authoring` owns Scheduler capability schemas, examples, source evidence, Tenant Admin DTOs, and definition projection contracts; hosts only aggregate/adapt them.
- Scheduler is the scheduling ingress. `dispatchgateway` only invokes a resolved Runtime target after Scheduler owns the run; `schedule` owns deterministic authoring validation, preview, and recurrence behavior.
- `browser` contains `@domainry/scheduler-client`.

Concrete Scheduler definition DML remains in the Scheduler implementation; the SDK exposes only its deployment-neutral contract through `persistence`.

The `remote` SaaS Binding intentionally is not a `modulehttp.Provider`. Its
`/v1/applications/{runtime}/...` endpoints are a private machine protocol: one
bearer credential is configured for exactly one Runtime, and the path Runtime
is only checked against that authenticated binding. No external header selects
an application. A standalone SaaS listener must not mount `/scheduler/...`.

The `/scheduler/...` Action surface is available only through
`SchedulerHTTPAdapterContractForTrustedHost`. Runtime may mount it only after a
trusted human-principal authenticator, exact Action-permission guard, and the
declared operation-evidence guards are installed. The legacy no-argument
contract function fails closed. Ordinary definition execution remains
`Binding.TriggerNow`; callers cannot supply `scheduled_for` or a global clock.

Products create plan records through the optional `ScheduledPlanService`
extension after resolving the current workspace and user. Module and SaaS use
the same DTO; the SaaS transport binds Runtime identity to its machine
credential, while the plan command carries only the already-resolved product
owner. A conversation reference is an association, not access authority.

The plan trigger stores its execution policy. Omitted values are normalized by
Scheduler: one-time work catches up once, recurring work skips missed windows,
the grace period is one minute, and dispatch gets three attempts with bounded
backoff. `catch_up_bounded` accepts 1–100 windows. Scheduler sends a signed
`ScheduledPlanDispatch` payload containing the plan ID, resolved owner, input,
allowed actions and conversation reference; the receiving product must
reauthorize those facts before execution.

## Definition publication fencing

Bare `DefinitionSnapshot.Revision` is process-local and is never a durable
cross-process ordering key. SaaS remote bindings must require descriptor
capability `definition_publication_fencing_v1`, then call:

```text
POST /v1/applications/{authenticated-runtime}/definition-publication-sessions
```

Scheduler atomically increments a durable per-application generation, creates
an unpredictable nonce, persists only its SHA-256 hash as the active session,
and returns:

```json
{
  "contract_version": "domainry-scheduler-definition-publication-v1",
  "generation": 8,
  "session_nonce": "<opaque random value>"
}
```

The remote Binding attaches this object to every snapshot automatically. The
Scheduler store compares snapshots transactionally by authenticated Runtime,
active generation, nonce hash, revision, and canonical content hash:

- only the currently active session may publish;
- within one session, revisions increase monotonically;
- the same revision and content is a no-write idempotent replay;
- the same revision with different content is a conflict;
- a new active generation may restart at revision 1;
- every request from an older generation remains fenced, including late packets;
- active session and accepted cursor survive Scheduler restart.

Session issuance and snapshot persistence use compare-and-swap transactions so
concurrent session claims leave only the greatest generation active. During
migration, Scheduler first hydrates existing legacy definitions, atomically
issues the first fenced session, then permanently rejects sessionless SaaS
reconcile. There is no silent fallback when the descriptor capability or
session endpoint is absent.

Module mode does not use the SaaS publication session. Its Reconcile calls are
serialized in-process and `ValidateModuleDefinitionSnapshot` deliberately
allows a new Runtime process to publish revision 1 after restart.

## Runtime callback authentication

Standalone Scheduler callbacks use HMAC contract
`domainry-scheduler-runtime-callback-hmac-v2`. The signature binds the uppercase
HTTP method, canonical path, configured Runtime identity, client identity,
timestamp, callback idempotency key, and exact body SHA-256. The internal
`X-Domainry-Runtime-ID` header is signed consistency evidence for one Runtime;
it is not an external tenant selector.

Receivers durably key callback claims by Runtime, method, path and idempotency
key. Exact-body retries reuse that claim: a terminal claim returns its stored
receipt, an active claim is retryable, and an expired claim may be fenced and
recovered by invoking the downstream owner with the same idempotency key.
Different bytes under the same key return conflict. This avoids pretending the
HTTP boundary can share one database transaction with every downstream owner;
the downstream owner's durable idempotency closes the crash window.
Timestamp freshness limits exposure but is not replay protection. HTTP 400/401,
403, 409, 429 and 5xx responses map to typed invalid, authentication, rejection,
idempotency-conflict and retryable SDK errors without copying response bodies.

The shared authoring contract accepts `workflow`, `report_snapshot_refresh`, and
deployment-owned `http` targets. Scheduled report export is intentionally not a
direct target: it must run through a scheduled Workflow so the Report owner and
project Action own approval, audit, export-job, artifact, and download evidence.

Run `go test ./...` before publishing an immutable SDK version.
