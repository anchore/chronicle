#!/usr/bin/env bash
set -eux -o pipefail

# ignore global and system git config to prevent interference from the host
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null

if [ -d "/path/to/dir" ]
then
    echo "fixture already exists!"
    exit 0
else
    echo "creating fixture..."
fi

git init repos/tagged-repo

pushd repos/tagged-repo

git config --local user.email "nope@nope.com"
git config --local user.name "nope"
git config --local commit.gpgsign false

trap 'popd' EXIT

git commit -m 'something' --allow-empty
# show that the timestamp cannot be extracted from the lightweight tag
sleep 3
git tag v0.1.0