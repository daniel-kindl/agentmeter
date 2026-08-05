#!/bin/sh
set -eu

base_branch=${1:-}
head_branch=${2:-}

case "$base_branch" in
    dev)
        case "$head_branch" in
            feat/*|fix/*|perf/*|refactor/*|docs/*|test/*|build/*|ci/*|chore/*|revert/*)
                ;;
            *)
                printf 'branch %s cannot target dev; use <type>/<slug> from dev\n' "$head_branch" >&2
                exit 1
                ;;
        esac
        ;;
    main)
        case "$head_branch" in
            dev|release-please--*)
                ;;
            *)
                printf 'branch %s cannot target main; promote dev or use release-please\n' "$head_branch" >&2
                exit 1
                ;;
        esac
        ;;
    *)
        printf 'unsupported protected base branch: %s\n' "$base_branch" >&2
        exit 1
        ;;
esac
