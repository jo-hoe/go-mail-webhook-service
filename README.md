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
mailClientConfig:
  mail: "example@gmail.com"
  credentialsPath: "/secrets/mail"

mailSelectors:
  - name: "OrderId"
    type: "subjectRegex"
    pattern: "Order ([0-9]+) confirmed"
    captureGroup: 1
  - name: "Amount"
    type: "bodyRegex"
    pattern: "Total: \\$([0-9]+\\.[0-9]{2})"
    captureGroup: 1

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

## Local development with k3d

A k3d cluster config is provided (dev/clusterconfig.yaml) and Makefile targets mirror the reference repo.
Use `make help` for details.
