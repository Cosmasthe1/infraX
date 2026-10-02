# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

- Add unit tests for `control-plane` HTTP handlers.
- Implement `workers` `Poller` and unit tests to make workers testable.
- Gate test coverage in CI for `control-plane` and enforce 60% threshold.
- Add Dockerfile for `apps/sample-app` so `docker compose up --build` works.
- Add Dependabot config for Go modules and npm.
- Create `terraform/modules/postgres` reusable module and add S3 backend scaffold.
- Add basic `CONTRIBUTING.md` to guide contributions and local development.
