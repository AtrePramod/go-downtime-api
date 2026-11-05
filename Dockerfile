# ---- Build stage: compile the API and the seed tool ----
FROM golang:1.27-alpine AS build
WORKDIR /src

# Download dependencies first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 makes fully static binaries that run on a minimal image.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api . \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./seed

# ---- Run stage: small image with just the binaries ----
FROM alpine:3.20
# tzdata lets you set TZ (e.g. TZ=Asia/Kolkata) so ?date= uses your local day.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H app
WORKDIR /app
COPY --from=build /out/api /out/seed /app/

ENV PORT=8080
EXPOSE 8080
USER app

# Database settings come from environment variables (DB_HOST, DB_USER, ...).
CMD ["/app/api"]
