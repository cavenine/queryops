FROM docker.io/golang:1.25.5-trixie AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

# Use Taskfile.yml as the single source of truth for prod builds.
RUN --mount=type=cache,target=/root/.cache/go-build \
	go tool task build

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

LABEL service="queryops"

ENV HOST=0.0.0.0
ENV PORT=8080

COPY --from=build /src/bin/queryops /queryops

ENTRYPOINT ["/queryops"]
CMD ["web"]
