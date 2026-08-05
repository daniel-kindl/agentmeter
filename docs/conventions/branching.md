# Branching model

Two long-lived branches, short-lived work branches. Same model across projects — learn it
once.

```
main   ──●────────────────●──────────────▶   released, tagged, always deployable
          ╲              ╱
dev    ────●────●────●──●─────────────────▶   integration
                 ╲  ╱
feat/x  ──────────●●──────────────────────▶   short-lived
```

## Branches

| Branch | Holds | Receives from | Protected |
|---|---|---|---|
| `main` | Tested, released code only | `dev` via PR, and release-please's Release PR | Yes |
| `dev` | Integrated work awaiting release | Work branches via PR | Yes |
| `<type>/<slug>` | One change in progress | — | No |

**Nothing is ever committed directly to `main` or `dev`.** Both take PRs only, including
from agents, including for one-line fixes.

## Work branch naming

`<type>/<short-kebab-slug>`, where `<type>` matches the Conventional Commit type of the
primary change:

```
feat/codex-token-diffing
fix/scanner-offset-reset
chore/scaffolding
ci/cross-compile-matrix
docs/adr-storage-schema
```

Cut from `dev`. Merge back into `dev`. Delete after merge.

## Merge strategy

| Merge | Strategy | Why |
|---|---|---|
| work branch → `dev` | **Squash** | One reviewed change becomes one commit; the squash message is what release tooling parses, so it must be a valid Conventional Commit |
| `dev` → `main` | **Merge commit** | Preserves the individual commits release-please needs to compute the version |
| Release PR → `main` | Whatever release-please asks for | Do not second-guess it |

The asymmetry matters. Squashing `dev` into `main` would collapse ten commits into one and
release-please would compute the wrong version from the single squashed message.

## Protection rules

On both `main` and `dev`:

- Require a PR, no direct pushes
- Require CI to pass
- Require the branch to be up to date before merge
- Disallow force pushes and deletion

`main` additionally requires that the source branch is `dev` or a release-please branch.

## Hotfixes

There is no separate hotfix flow. A production bug is a `fix/` branch off `dev`, merged to
`dev`, then `dev` to `main` — the same path as everything else. If that path is too slow,
the problem is that `dev` contains unreleasable work, which is a discipline problem rather
than a branching one.

## Rewriting history

Fine on an unmerged work branch — rebase, squash, and force-push freely to keep the branch
readable.

Never on `main` or `dev`. Not once, not "just this time."
