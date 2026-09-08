# Stage 1: Build Go binary — API-only as of 2026-04-22.
# The embedded Vue SPA that used to be here moved to:
#   construct-space/domains-web    (public landing → domains.lisaos.dev)
#   construct-space/my-web/spaces/domains (tenant dashboard → my.lisaos.dev/domains)
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o domains .

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /app/domains .
ENV PORT=80
EXPOSE 80
CMD ["./domains"]
