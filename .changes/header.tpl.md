# Changelog

All notable user-facing changes to PowerLab. We follow
[Semantic Versioning](https://semver.org/) — `vMAJOR.MINOR.PATCH`. While
PowerLab is in `v0.x`, breaking changes can land in MINOR bumps; from
`v1.0` onward we commit to backwards compatibility within MAJOR.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## How entries land here

Each PR adds a tiny YAML fragment under `.changes/unreleased/<id>.yaml`.
At release time, `changie batch <version>` aggregates the fragments into
a new section below this header. See `CONTRIBUTING.md` for the workflow.
