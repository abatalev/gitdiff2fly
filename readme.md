# GitDiff2Fly

[![Go](https://github.com/abatalev/gitdiff2fly/actions/workflows/go.yml/badge.svg)](https://github.com/abatalev/gitdiff2fly/actions/workflows/go.yml)

## Intro

GitDiff2Fly is a directory structure creating helper for flyway.

## Install

```sh
go build .
```

## build docker images

```
docker build -f Dockerfile.gitdiff2fly -t abatalev/gitdiff2fly:1.2 .
docker build -f Dockerfile.flyway -t abatalev/flyway:7.10.0
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
