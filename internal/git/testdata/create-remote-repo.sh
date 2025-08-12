#!/usr/bin/env bash
set -eux -o pipefail

if [ -d "/path/to/dir" ]
then
    echo "fixture already exists!"
    exit 0
else
    echo "creating fixture..."
fi

git init repos/remote-repo

pushd repos/remote-repo

git config --local user.email "nope@nope.com"
git config --local user.name "nope"

trap 'popd' EXIT

git remote add origin git@github.com:wagoodman/count-goober.git
git remote add upstream git@github.com:upstream/count-goober.git
