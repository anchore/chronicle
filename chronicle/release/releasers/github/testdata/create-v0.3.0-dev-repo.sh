#!/usr/bin/env bash
set -eux -o pipefail

if [ -d "/path/to/dir" ]
then
    echo "fixture already exists!"
    exit 0
else
    echo "creating fixture..."
fi

git init repos/v0.3.0-dev-repo

pushd repos/v0.3.0-dev-repo

git config --local user.email "nope@nope.com"
git config --local user.name "nope"

git remote add origin git@github.com:wagoodman/count-goober.git

trap 'popd' EXIT

git commit -m 'something' --allow-empty
git commit -m 'something-else' --allow-empty
git tag v0.1.0

git commit -m 'fix: after-0.1.0' --allow-empty
git commit -m 'fix: also-after-0.1.0' --allow-empty
git commit -m 'fix: nothing was working' --allow-empty
git tag v0.1.1

git commit -m 'fix: bad release of 0.1.1' --allow-empty
git commit -m 'feat: implement everything that wasnt there' --allow-empty
git commit -m 'fix: missed something of everything' --allow-empty
git tag v0.2.0

git commit -m 'feat: working on next release item' --allow-empty
