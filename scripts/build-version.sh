#!/bin/sh
set -eu

context_dir=$1
explicit_version=$2
fallback_version="unknown-000000000000Z"

if [ -n "$explicit_version" ]; then
    printf '%s\n' "$explicit_version"
    exit 0
fi

if command -v git >/dev/null 2>&1 && git -C "$context_dir" rev-parse --verify HEAD >/dev/null 2>&1; then
    commit_hash=$(git -C "$context_dir" rev-parse HEAD | cut -c1-7)
    build_time=$(date -u +%y%m%d%H%MZ)
    case "${commit_hash}-${build_time}" in
        ???????-??????????Z)
            printf '%s\n' "${commit_hash}-${build_time}"
            exit 0
            ;;
    esac
fi

printf '%s\n' "$fallback_version"
