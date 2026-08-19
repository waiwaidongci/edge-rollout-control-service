FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /edge-rollout ./cmd/edge-rollout

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /edge-rollout /usr/local/bin/edge-rollout
COPY configs/config.yaml /app/configs/config.yaml
RUN mkdir -p /app/data && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["edge-rollout", "-config", "/app/configs/config.yaml"]
