# Contributing

Read [AGENTS.md](AGENTS.md) before working in this repository. It contains the privacy,
testing, dependency, and verification rules that apply to every contribution.

## Workflow

1. Install the repository hooks with `make hooks`.
2. Branch from `dev` using the naming rules in
   [docs/conventions/branching.md](docs/conventions/branching.md).
3. Keep commits small and use the format in
   [docs/conventions/commits.md](docs/conventions/commits.md).
4. Add or update tests with every behavior change.
5. Run `make check` and `make cross` before opening a pull request into `dev`.

Do not add a dependency without prior approval. Do not commit real session logs or fixtures
derived from them.

