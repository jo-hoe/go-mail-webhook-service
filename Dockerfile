FROM golang:1.22-alpine as build

WORKDIR /app
COPY . .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/main ./main.go

FROM golang:1.22-alpine

COPY --from=build /bin/main main

ENTRYPOINT ["./main"]