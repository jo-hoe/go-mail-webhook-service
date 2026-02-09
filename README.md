# Go Mail Webhook Service

[![Test Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/test/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=test)
[![Lint Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/lint/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/jo-hoe/go-mail-webhook-service)](https://goreportcard.com/report/github.com/jo-hoe/go-mail-webhook-service)
[![Coverage Status](https://coveralls.io/repos/github/jo-hoe/go-mail-webhook-service/badge.svg?branch=main)](https://coveralls.io/github/jo-hoe/go-mail-webhook-service?branch=main)

Webhook allows to pull mails and send requests to a callback URL.

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/)

### Mail Client

Currently, the only supported mail client is GMail.
You will need the client credentials file, which you should set to the name `client_secret.json` and the `request.token` file.
An example of creating it is described [in this README](cli/gmail/README.md).
Once created, use the [configuration file](#configuration-example) to set the directory where both files are stored.

### Optional Components

Run the project using `make`. Make is typically installed by default on Linux and Mac.

If you do not have it and run on Windows, you can directly install it from [gnuwin32](https://gnuwin32.sourceforge.net/packages/make.htm) or via `winget`

```PowerShell
winget install GnuWin32.Make
```

If you want to run the project without Docker, you can install [Golang](https://go.dev/doc/install)

## Configuration Example

Create a file `dev/config.yaml` (use `dev/config.example.yaml` as a template). The application supports structured callback sections and selector-based placeholders.

Placeholders:

- Use ${SelectorName} in headers/queryParams/form/body to substitute values extracted by selectors.
- Selector names must be alphanumeric only (^[0-9A-Za-z]+$).

Example:

```yaml
- mailClientConfig:
    mail: "example@gmail.com"
    credentialsPath: "/secrets/mail"

  mailSelectors:
    - name: "OrderId"
      type: "subjectRegex"
      pattern: "Order ([0-9]+) confirmed"
      captureGroup: 1
      scope: true
    - name: "Amount"
      type: "bodyRegex"
      pattern: "Total: \\$([0-9]+\\.[0-9]{2})"
      captureGroup: 1
      scope: false

  callback:
    url: "https://example.com/callback"
    method: "POST"
    timeout: "24s"
    retries: 0

    headers:
      - key: "X-Order-Id"
        value: "${OrderId}"
      - key: "Content-Type"
        value: "application/json"

    queryParams:
      - key: "campaign"
        value: "winter"

    form:
      - key: "note"
        value: "Processed order ${OrderId}"

    body: |
      {
        "amount": "${Amount}"
      }
```

Notes:
- headers keys allow alphanumeric and hyphens.
- queryParams and form keys must be alphanumeric only.
- body is a raw string; set Content-Type via headers when needed (e.g., application/json).

## How to use

After you have fulfilled the prerequisites, you can start the service.

### Start

Either via docker compose

```bash
docker compose up
```

or use `make`

```bash
make
```

## Linting

Project used golangci-lint for linting.

### Installation

See <https://golangci-lint.run/usage/install/>

### Execution

Run the linting locally by executing

```bash
golangci-lint run ./...
```

in the working directory

## Kubernetes Helm Chart

A Helm chart is provided under charts/go-mail-webhook-service. The application reads its configuration from /go/config/config.yaml inside the container, rendered from .Values.configs (array).

Basic install from local chart directory:
- helm install go-mail-webhook-service ./charts/go-mail-webhook-service -f your-values.yaml

Key values:
- image.repository: container image repository (default ghcr.io/jo-hoe/go-mail-webhook-service)
- image.tag: image tag (default latest)
- configs: array of application config objects; this is rendered verbatim into the ConfigMap at /go/config/config.yaml
- job.enabled: if true, runs a Helm hook Job once after install/upgrade
- cronjob.enabled: if true, runs the app on a schedule

See charts/go-mail-webhook-service/values.yaml for full options and an example.

## Local development with k3d

A k3d cluster config is provided (dev/clusterconfig.yaml) and Makefile targets mirror the reference repo.

Flow:
- make start-k3d
  - Creates a k3d cluster with a local registry
  - Builds the image locally and pushes to localhost:5000
  - Installs the Helm chart with dev/config.yaml overrides
- make upgrade-k3d
  - Rebuilds/pushes the image and upgrades the Helm release using dev/config.yaml
- make uninstall-k3d
  - Uninstalls the Helm release
- make stop-k3d
  - Deletes the k3d cluster and local registry
- make restart-k3d
  - Stops and recreates the cluster and re-installs the chart

Dev overrides:

- Edit dev/config.yaml to set image repository (registry.localhost:5000/go-mail-webhook-service) and supply your configs array.

## CI/CD

Two GitHub Actions workflows are included:

- .github/workflows/image-release.yml
  - Builds and publishes the Docker image to GHCR when a tag matching v[0-9]+.[0-9]+.[0-9]+ is pushed
  - Tags include full semver and major.minor, plus sha for non-tag builds

- .github/workflows/chart-release.yml
  - Publishes Helm charts using helm/chart-releaser when changes are pushed to the charts/ folder on main
  - Ensure GitHub Pages is enabled for the gh-pages branch if you want to serve an index.yaml as a Helm repo

Tagging a new release:

- Bump versions as needed (appVersion in Chart.yaml for image tag reference, version for chart)
- Push a git tag like v1.2.3 to trigger image publishing
- Merge changes to charts/ on main to trigger chart release
