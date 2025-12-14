# QUERYOPS

# Stack

- [Go](https://go.dev/doc/)
- [Datastar](https://github.com/starfederation/datastar)
- [Templ](https://templ.guide/)
  - [Tailwind](https://tailwindcss.com/) x [DaisyUI](https://daisyui.com/)

# Setup

1. Install Dependencies

```shell
go mod tidy
```

2. Create 🚀

# Development

Live Reload is setup out of the box - powered by [Air](https://github.com/air-verse/air) + [esbuild](cmd/web/build/main.go)

Use the [live task](./Taskfile.yml#L113) from the [Taskfile](https://taskfile.dev/) to start with live reload setup

```shell
go tool task live
```

Navigate to [`http://localhost:8080`](http://localhost:8080) in your favorite web browser to begin

# Starting the Server

```shell
go tool task run
```

Navigate to [`http://localhost:8080`](http://localhost:8080) in your favorite web browser

# Deployment

## Building an Executable

The `task build` [task](./Taskfile.yml#L37) will assemble and build a binary

[Dockerfile](./Dockerfile)

# References

## Server

- [go](https://go.dev/)
- [datastar sdk](https://github.com/starfederation/datastar/tree/develop/sdk)
- [templ](https://templ.guide/)

## Web Components x Datastar

[🔗 Vanilla Web Components Setup](./web/libs/web-components/README.md)

[🔗 Lit Web Components Setup](./web/libs/lit/README.md)

## Client

- [datastar](https://www.jsdelivr.com/package/gh/starfederation/datastar)
- [tailwindcss](https://tailwindcss.com/)
- [daisyui](https://daisyui.com/)
- [esbuild](https://esbuild.github.io/)
- [lit](https://lit.dev/)
