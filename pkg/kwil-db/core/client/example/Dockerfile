FROM golang:1.23 AS build

WORKDIR /app

# Copy the client example files
COPY core/client/example/ .

# Explicitly copy the core and parse directories from kwil-db
COPY core/ ./kwil-db/core/
COPY parse/ ./kwil-db/parse/

# Remove go.work if it exists
RUN test -f go.work && rm go.work || true

# Build the Go client app
RUN GOWORK=off CGO_ENABLED=0 go build -o kwil-client-app main.go

# Use a minimal image for the final stage
FROM ubuntu:24.04
WORKDIR /app
COPY --from=build /app/kwil-client-app ./kwil-client-app

ENTRYPOINT ["./kwil-client-app"]
