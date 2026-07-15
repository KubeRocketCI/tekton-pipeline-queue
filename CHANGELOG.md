<a name="unreleased"></a>
## [Unreleased]


<a name="v0.1.0"></a>
## v0.1.0 - 2026-07-15
### Features

- stamp actor annotations on admitted and cancelled runs
- implement queue reconciliation and admission strategies
- add PipelineRunQueue API and controller skeleton

### Bug Fixes

- address code review findings on RBAC, status, and tests
- status patch conflicts and same-second supersession
- drop stale lane gauges on queue deletion or invalid selector

### Code Refactoring

- apply simplify-review cleanups
- strip unread PipelineRun fields from the informer cache

### Testing

- replace scaffolded Go e2e with Chainsaw suite

### Routine

- drop CodeQL workflow, follow KubeRocketCI org convention
- add CodeQL workflow for parity with sibling repos
- add Helm chart, CI workflows, and repo conventions

### Documentation

- split docs into index, use-cases, and debugging guides
- add lane configuration examples for common scenarios


[Unreleased]: https://github.com/KubeRocketCI/tekton-pipeline-queue/compare/v0.1.0...HEAD
