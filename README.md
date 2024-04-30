# Go Mail Webhook Service

[![Test Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/test/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=test)
[![Lint Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/lint/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/jo-hoe/go-mail-webhook-service)](https://goreportcard.com/report/github.com/jo-hoe/go-mail-webhook-service)
[![Coverage Status](https://coveralls.io/repos/github/jo-hoe/go-mail-webhook-service/badge.svg?branch=main)](https://coveralls.io/github/jo-hoe/go-mail-webhook-service?branch=main)

Still work in progress.
Webhook allowing to pull mails and send requests to an callback url.

## Configuration Example

```yaml
- mailServiceConfig: 
    mail: "example@gmail.com"
    credentialsPath: "/path/to/client_secrets/file/"
  subjectSelectorRegex: ".*"
  bodySelectorRegexList:
  - name: "test"
    regex: "[a-z]{0,6}"
  - name: "test2"
    regex: ".*"
  callback:
    url: "https://example.com/callback"
    method: "POST"
```
