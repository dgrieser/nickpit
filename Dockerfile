FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /llm-review ./cmd/llm-review

FROM alpine:3.20
RUN apk add --no-cache git ca-certificates
COPY --from=builder /llm-review /usr/local/bin/llm-review
COPY prompts/ /etc/llm-review/prompts/
ENTRYPOINT ["llm-review"]
