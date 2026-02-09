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

Create a file with the name `config.yaml` in directory `config`.
An example of the configuration file is described below.

```yaml
- mailClientConfig: 
    mail: "example@gmail.com" # mail address to be checked
    credentialsPath: "/path/to/client_secrets/file/" # location of the credentials files for the mail client, can also be a location relative to the current directory
  runOnce: false # if set to true, the service will run once and exit, default is false
  intervalBetweenExecutions: 0s # interval between executions of the service, default is 0 seconds
  subjectSelectorRegex: ".*" # regex to match the subject of the mail
  bodySelectorRegexList: # regex to match the body of the mail, if no body is needed do not set this
  - name: "test" # name json attribute in the callback 
    regex: "[a-z]{0,6}" # regex which matches the body, is set as value of the json attribute
  - name: "test2"
    regex: ".*"
  callback:
    url: "https://example.com/callback" # callback url
    method: "POST" # method of the callback, has to be provided as uppercase string
    timeout: 24s # timeout for the callback, default is 24 seconds
    retries: 0 # number of retries for the callback, default is 0
    fields:
      - name: "name1" # arbitrary name of the field, will be used as json attribute in the callback
        type: "jsonValue" # type of the field, currently only jsonValue is supported, which means that the value will be set as value of the json attribute in the callback
        value: "value1" #
      - name: "name2"
        type: "headerValue" # type of the field, currently only headerValue is supported, which means that the value will be set as value of the header in the callback
        value: "ContentType: application/json" # name of the header, which value will be set as value of the header in the callback
      - name: "name3"
        type: "queryParamValue"
        value: "value3" # value of the query parameter, which will be set as value of the query
        
```

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

A k3d cluster config is provided (k3d/clusterconfig.yaml) and Makefile targets mirror the reference repo.

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
