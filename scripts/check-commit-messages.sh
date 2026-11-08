#!/bin/sh
set -eu
base="$1"
head="$2"
status=0
for sha in $(git rev-list "$base..$head"); do
    git log -1 --format=%B "$sha" > /tmp/msg
    if ! sh .githooks/commit-msg /tmp/msg; then
        echo "Rejected commit: $sha" >&2
        status=1
    fi
done
exit "$status"
