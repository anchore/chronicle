#!/usr/bin/env bash
set -eux -o pipefail

# ignore global and system git config to prevent interference from the host
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null

git init repos/worktree-main-repo

pushd repos/worktree-main-repo

git config --local user.email "nope@nope.com"
git config --local user.name "nope"
git config --local commit.gpgsign false

git remote add origin git@github.com:wagoodman/count-goober.git

# a worktree can only be added once there is at least one commit
git commit --allow-empty -m "something"

# add a linked worktree alongside the main repo; its .git is a file pointing at
# the main repo's git dir, and config is shared via the common dir
git worktree add ../worktree-linked-repo -b linked

popd
