---
title: Making a change
description: What a pull request to NSGo needs, from the same change on every backend to tests, docs and a changelog line.
group: Contributing
order: 82
---

## Before you start

Open an issue or a discussion for anything larger than a fix, so the design is agreed before the code. Questions the design leaves open are listed in [ARCHITECTURE.md](https://github.com/nsgo-dev/nsgo/blob/main/ARCHITECTURE.md#open-questions); their answers are decided there first.

## Every backend

A fix or a feature in one backend usually belongs in the others. Make the same change on every target that needs it, or record what is missing in the [roadmap](https://github.com/nsgo-dev/nsgo/blob/main/docs/roadmap.md). Components look and behave the same on every platform.

## Checks

Use the `nsgo` tool rather than `go vet`, `go test` or `go build`:

```sh
go tool nsgo check   # vet every package
go tool nsgo test    # every package's tests
```

Add a test for what you change: `nsgotest` for what a user sees, a package's own tests for the rest. Run the gallery on the platforms you touched ([Development setup](../contributing.md#the-gallery)).

## The pull request

Title it as a Conventional Commit, such as `fix(linux): keep the caret after a paste`: a squash merge makes the title the commit's subject, and a check fails until it fits. The template asks for the targets the change touches, before and after screenshots of anything visible, and the checks you ran.

## Docs and the changelog

- Update the document whose facts change: a subject in `docs/architecture/` for how NSGo works, a page in `docs/public/` for how apps and components use it ([Docs and the website](docs.md)).
- Keep `README.md` and `README.zh-CN.md` equivalent; everything else is in English, with Chinese pages in `docs/public/zh/`.
- Record a change users can see as one line under `Unreleased` in `CHANGELOG.md`, with a reference such as `(#12 by @you)`.
