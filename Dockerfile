FROM docker.io/golang:1.25.5-trixie AS build

WORKDIR /src
COPY . ./

RUN go mod download

# Generate templates and build static assets for prod embed.
RUN go tool templ generate
RUN go tool gotailwind -i web/resources/styles/styles.css -o web/resources/static/index.css
RUN go run cmd/web/build/main.go

RUN --mount=type=cache,target=/root/.cache/go-build \
	go build -tags=prod -ldflags="-s" -o /bin/main ./cmd

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

LABEL org.opencontainers.image.source="https://github.com/cavenine/queryops"
LABEL org.opencontainers.image.description="QueryOps"

ENV HOST=0.0.0.0
ENV PORT=8080

COPY --from=build /bin/main /main

ENTRYPOINT ["/main"]
CMD ["web"]
