# Go Mail Webhook Service

[![Test Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/test/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=test)
[![Lint Status](https://github.com/jo-hoe/go-mail-webhook-service/workflows/lint/badge.svg)](https://github.com/jo-hoe/go-mail-webhook-service/actions?workflow=lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/jo-hoe/go-mail-webhook-service)](https://goreportcard.com/report/github.com/jo-hoe/go-mail-webhook-service)
[![Coverage Status](https://coveralls.io/repos/github/jo-hoe/go-mail-webhook-service/badge.svg?branch=main)](https://coveralls.io/github/jo-hoe/go-mail-webhook-service?branch=main)

Still work in progress.
Webhook allowing to pull mails and send requests to an callback url.

## Configuration Example

```yaml
- mailClientConfig: 
    mail: "example@gmail.com" # mail address to be checked
    credentialsPath: "/path/to/client_secrets/file/" # location of the credentials files for the mail client
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
```

## Related Links

- [create gmail credentials](https://developers.google.com/gmail/api/auth/web-server#create_a_client_id_and_client_secret)
- [gmail usage limits](https://developers.google.com/gmail/api/reference/quota)
