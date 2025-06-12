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
   1. Download the `client_secret.json` (the json will have a longer name, just rename it to `client_secret.json`)

## Creation of request.token

1. Place the `client_secret.json` in the folder
2. Execute the `main.go` via `go run main.go ".\"` (this is the PowerShell command), the input parameter is the directory of the `client_secret.json`
3. Open the webpage which is prompted in the terminal and continue in the wizard
4. The `request.token` file will be generated

## Notes

The code will then create a file called `request.token`, which you put in the same directory as the client_credentials.json file.
