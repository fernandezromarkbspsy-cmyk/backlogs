FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/backlogs-bot ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates poppler-utils imagemagick \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/backlogs-bot /app/backlogs-bot
EXPOSE 8080
CMD ["/app/backlogs-bot"]
