# Go project structure

```text
cmd/wlog/                 Application entry point and dependency wiring
domain/                   Business entities, rules, errors, and interfaces
application/              Use cases and orchestration
infrastructure/           Database and external service implementations
delivery/                 CLI and other transport adapters
docs/                     Product and architecture documentation
```

Dependency direction: `delivery → application → domain ← infrastructure`.

Define small interfaces where they are consumed, in `domain` or `application`.
Implement those interfaces in `infrastructure`, then connect implementations
to use cases and delivery adapters in `cmd/wlog/main.go`. Keep business rules
out of handlers and infrastructure code. Add feature-specific subpackages only
when the corresponding feature exists. The initial CLI and local storage design
is documented in `docs/speckit/01-initiate-cli-and-local-storage/` and follows
the requirements in the BRD, PRD, and ERD.
