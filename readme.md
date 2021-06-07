# GitDiff2Fly

## Intro

GitDiff2Fly is a directory structure creating helper for flyway.

## Install

```sh
go build .
```

## Example

```sh
mkdir tmp_test
cd tmp_test
git init --bare
cd ..
git clone tmp_test tmp_test2

git clone url test_repo
cd test_repo
gitdiff2fly -next-version=1.4 -flyway-repo-path=../tmp_test2
```