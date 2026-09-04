# Domainry Scheduler SDK

This repository contains the deployment-neutral Scheduler protocol:

- Go contracts for definitions, schedules, triggers, receipts, Module hosts and SaaS transports.
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

The shared authoring contract accepts `workflow`, `report_snapshot_refresh`, and
deployment-owned `http` targets. Scheduled report export is intentionally not a
direct target: it must run through a scheduled Workflow so the Report owner and
project Action own approval, audit, export-job, artifact, and download evidence.

Run `go test ./...` before publishing an immutable SDK version.
