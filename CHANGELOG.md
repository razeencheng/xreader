# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Runtime-configurable Google Analytics via `XREADER_GA_ID`.
- Bilingual documentation (English + 简体中文): README, deployment, contributing.
- Docker Hub publishing alongside GHCR on tagged releases.
- Dependabot, PR template, and gitleaks secret scanning.

### Changed
- `docker-compose.yml` now requires `SESSION_SECRET` and binds Postgres to loopback.
