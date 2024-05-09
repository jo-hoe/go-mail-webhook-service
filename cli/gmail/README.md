# GMail API Token Generation

The current implementation relies on an OAuth.

## Prerequisites

- install [Golang](https://go.dev/doc/install)
- a GMail account

## Creation of client secret

1. Create a new project
   1. Go to <https://console.developers.google.com>
   1. Switch to your work account if need be (top right)
   1. Create a new project dropdown, top left next to your domain
1. Grant the project Calendar API access
   1. Click "GMail API"
   1. Click "Enable"
1. Grab your credentials
   1. Click "Credentials" on the left side
   1. Create a new OAuth credential with type "Web application"

## Example Usage

```bash
main "directory/of/client_credentials/json/file"
```

After a successful start you need to extract the auth code and enter it into the CLI.

```bash
http://localhost:8080/?state=state-token&code=<extract this>&scope=https://www.googleapis.com/auth/gmail.modify
```

The code will then create a file called `request.token`, which you put in the same directory as the client_credentials.json file.
