#!/usr/bin/env bash
set -eux -o pipefail

if [ -d "/path/to/dir" ]
then
    echo "fixture already exists!"
    exit 0
else
    echo "creating fixture..."
fi

git init repos/annotated-tagged-repo

pushd repos/annotated-tagged-repo

git config --local user.email "nope@nope.com"
git config --local user.name "nope"

trap 'popd' EXIT

git commit -m 'something' --allow-empty
# show that there is a difference between the resolved commit timestamp and the tag timestamp
sleep 3
git tag -a v0.1.0 -m "tagging v0.1.0"
