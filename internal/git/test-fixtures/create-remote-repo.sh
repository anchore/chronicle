#!/usr/bin/env bash
set -eux -o pipefail

if [ -d "/path/to/dir" ]
then
    echo "fixture already exists!"
    exit 0
else
    echo "creating fixture..."
fi

git init remote-repo

git config user.email "nope@nope.com"
git config user.name "nope"

pushd remote-repo

trap 'popd' EXIT

git remote add origin git@github.com:wagoodman/count-goober.git
