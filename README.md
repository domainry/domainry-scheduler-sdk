# Domainry Scheduler SDK

This repository contains the deployment-neutral Scheduler protocol:

- Go contracts for definitions, schedules, triggers, receipts, Module hosts and SaaS transports.
- `schedule` for deterministic validation, recurrence planning and window keys shared by Module, SaaS and Runtime compatibility paths.
- `saashost/httptransport` for authenticated Runtime-to-SaaS communication.
- `dispatchgateway` for authenticated Scheduler-SaaS-to-Runtime execution callbacks.
- `@domainry/scheduler-client` in `browser/` for the Admin Console.

Runtime supplies published configuration and downstream owner capabilities through the SDK. The Scheduler implementation remains replaceable between Module and SaaS without changing business handlers or frontend request semantics.

## Package layout

- The root package is the stable Scheduler `Factory`, `Binding`, definition, trigger, and receipt entrypoint.
- `persistence` owns durable definition projection contracts.
- `modulehost` describes embedded host infrastructure, providers, dispatcher, and run-store capabilities.
- `saashost` and `saashost/httptransport` describe authenticated SaaS composition.
- `dispatchgateway` is the execution callback boundary; `schedule` owns deterministic recurrence behavior.
- `browser` contains `@domainry/scheduler-client`.

Concrete Scheduler definition DML remains in the Scheduler implementation; the SDK exposes only its deployment-neutral contract through `persistence`.

Run `go test ./...` before publishing an immutable SDK version.
