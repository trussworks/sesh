# Contributing to sesh

If you are interested in using or contributing to sesh, let us know! sesh@truss.works

We welcome contributions to sesh. Please feel free to submit focused PRs, and feel free to open issues first to discuss ideas first, if you like. This repository is governed by the [Contributor Covenant](CODE_OF_CONDUCT.md)

## Dev Setup

1. start a postgres server
2. create a .env file with `cp .env.example .env`, set values to match your postgres config
3. run `make reset_test_db`
4. run `make test`

To import sesh locally from your repository, so that changes you make to it are imported immediately in your repo, use the `replace` directive in your go.mod:

```
replace github.com/trussworks/sesh => ../sesh
```

## Package layout

The `sesh` package is meant to be the main interface to sesh. There are four packages used by the top level package.

- `pkg/domain` is home to all the types and interfaces that are used in common by the packages.
- `pkg/dbstore` is the postgres db implementation.
- `pkg/session` contains the session creation/checking/destruction logic. And
- `pkg/seshttp` has all the http wrangling and middleware.
- `pkg/mock` has some mocks used in tests.

## Oddities

In order to make sure that the package docs include everything you need to know to use the package, there are a couple interfaces that are defined both there and in the domain package.
