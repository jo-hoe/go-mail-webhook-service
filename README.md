# Go Mail Webhook Service

[![Test Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/test/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=test)
[![Lint Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/lint/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/jo-hoe/go-mail-webhook-service)](https://goreportcard.com/report/github.com/jo-hoe/go-mail-webhook-service)
[![Coverage Status](https://coveralls.io/repos/github/jo-hoe/go-mail-webhook-service/badge.svg?branch=main)](https://coveralls.io/github/jo-hoe/go-mail-webhook-service?branch=main)

This service polls mails and triggers HTTP webhooks. It now uses the dependency-free gohook library for building and executing webhooks with Go text/template support.

## Prerequisites

- Docker
- Gmail client credentials and token if using Gmail (currently the only supported mail client)
  - Place client_secret.json and request.token in /secrets/mail inside the container or mount via Helm chart (mailClient.gmail.secrets.*)
  - See cli/gmail/README.md for details

Optional:

- Go (to run without Docker)
- Make (for convenience targets)

## Configuration

Create a file `dev/config.yaml` using `dev/config.example.yaml` as a template. Configuration consists of:

- mailSelectors: extract values from incoming mails (subject/body/sender/recipient/attachment name) using regex
- callback: gohook.Config describing the webhook execution plan
- processing: how to mark mails after successful webhook execution

### Templates

- Use {{ .SelectorName }} in callback fields to reference values extracted by selectors.

### Example

```yaml
logLevel: "info"

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
  # gohook.Config (see https://github.com/jo-hoe/gohook)
  url: "https://example.com/callback?source={{ .OrderId }}"
  method: "POST"
  timeout: "24s"
  maxRetries: 0
  backoff: "0s"
  expectedStatus: [200, 201]
  strictTemplates: true
  headers:
    Content-Type: "application/json"
    X-Order-Id: "{{ .OrderId }}"
  query:
    amount: "{{ .Amount }}"
  body: |
    {
      "orderId": "{{ .OrderId }}",
      "amount": "{{ .Amount }}"
    }

processing:
  # markRead or delete
  processedAction: "markRead"
```

Notes:

- gohook applies defaults for method (POST if body set, else GET)
- headers and query are maps; keys and values can be templated
- Strict template handling (strictTemplates) controls missing key behavior
- expectedStatus limits acceptable HTTP status codes; unexpected codes and transport errors are retried up to maxRetries with backoff delays
- This version does not provide multipart/form-data form/attachment forwarding (no legacy callback fields)

See `dev/config.example.yaml` for a comprehensive example.

## How to run

Docker Compose:

```bash
docker compose up
```

Make (local build/run helpers):

```bash
make
```

## Linting

The project uses golangci-lint.

Install: <https://golangci-lint.run/usage/install/>

Run:

```bash
golangci-lint run ./...
```

## Local development with k3d

A k3d cluster config is provided at `dev/clusterconfig.yaml`, and Makefile targets mirror the reference repo.
Use `make help` for details.
