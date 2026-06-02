<!--
Copyright 2026 Code Capsules

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# Contributing

This document describes how **public contributions** and **Code Capsules** changes are handled in this repository.

For what we actively maintain, see [`MAINTENANCE.md`](MAINTENANCE.md).

## Pull requests

All changes land through a **pull request**. Direct pushes to `main` are not used for feature or fix work from outside the team.

| Requirement                | Detail                                                                                                                                                                                       |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Pull request required**  | Every code, chart, or documentation change must be submitted as a PR against `main`.                                                                                                         |
| **Code Capsules approval** | A PR may be merged only after **approval from a member of the Code Capsules team**. External contributors cannot self-approve or merge.                                                      |
| **Tests must pass**        | PRs must pass the checks defined in this repository (see [Tests and CI](#tests-and-ci)). Fix failures before requesting review.                                                              |
| **Quality and security**   | Changes must be secure, maintainable, and consistent with existing patterns. PRs that are unclear, unsafe, or low quality may be **closed or rejected** without merge, even when tests pass. |

## Before you open a PR

1. **Scope** — Confirm your change fits [active maintenance](MAINTENANCE.md#in-scope). Out-of-scope work (for example built-in backup features) is unlikely to be accepted.
2. **Issue (optional)** — For larger changes, open an issue first to avoid wasted effort on work we will not merge.
3. **Branch** — Fork or branch from current `main` and keep the diff focused on one logical change where possible.
4. **License headers** — New or substantively modified files should include the Apache 2.0 header used elsewhere in the repo. Run `./hack/license-check` before submitting.
5. **Changelog** — User-visible behavior changes should be noted in [`CHANGELOG.md`](CHANGELOG.md) when appropriate.

## Tests and CI

Relevant automation lives under [`.github/workflows/`](.github/workflows/):

| Workflow                                      | When it runs                                   | What it does                            |
| --------------------------------------------- | ---------------------------------------------- | --------------------------------------- |
| [**Go Tests**](.github/workflows/go-test.yml) | Push and pull requests to `main`               | `make test` (unit tests)                |
| [**E2E Tests**](.github/workflows/e2e.yml)    | Manual dispatch; pushes to configured branches | End-to-end tests against a Kind cluster |

Locally, run the same checks that apply to your change:

```shell
make test
./hack/license-check
```

If your change touches Helm, CRDs, or cluster behavior, run additional validation described in [`docs/`](docs/) or ask in the PR which extra steps reviewers expect.

CI configuration may evolve; **the workflows and Makefile targets in this repository are the source of truth** for what must pass before merge.

## Review expectations

Reviewers may request:

- Clear PR description (problem, approach, test evidence).
- Tests for new behavior or regressions.
- Security considerations (credentials, RBAC, SQL injection, privilege boundaries).
- Updates to docs or chart README when behavior or values change.

Approval is at the discretion of Code Capsules. Meeting the checklist above does not guarantee merge.

## Questions

Open a GitHub issue for bugs or feature requests in scope. For security-sensitive findings, do not post secrets or exploit details in public issues; use your organization’s responsible disclosure process.
