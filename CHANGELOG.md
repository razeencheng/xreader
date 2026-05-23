# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.7] - 2026-05-23

### Added
- Runtime-configurable Google Analytics via `XREADER_GA_ID`.
- Bilingual documentation (English + 简体中文): README, deployment, contributing.
- Docker Hub publishing alongside GHCR on tagged releases.
- Dependabot, PR template, and gitleaks secret scanning.

### Changed
- `docker-compose.yml` now requires `SESSION_SECRET` and binds Postgres to loopback.
- Dependency upgrades: react/react-dom 19.2.6, typescript 6.0.3, vitest 4.1.7,
  zustand 5.0.13, eslint-config-next 16.2.6, golang.org/x/net 0.55.0.
