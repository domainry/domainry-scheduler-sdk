# Domainry Scheduler SDK

This repository contains the deployment-neutral Scheduler protocol:

- Go contracts for definitions, schedules, triggers, receipts, Module hosts and SaaS transports.
- `schedule` for deterministic validation, recurrence planning and window keys shared by Module, SaaS and Runtime compatibility paths.
- `saashost/httptransport` for authenticated Runtime-to-SaaS communication.
- `@domainry/scheduler-client` in `browser/` for the Admin Console.

Runtime supplies published configuration and downstream owner capabilities through the SDK. The Scheduler implementation remains replaceable between Module and SaaS without changing business handlers or frontend request semantics.
