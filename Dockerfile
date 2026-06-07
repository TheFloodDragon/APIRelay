# 构建后端
FROM golang:1.21-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o apirelay ./cmd/server

# 最终镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite-libs
WORKDIR /app

COPY --from=backend-builder /app/apirelay ./apirelay
COPY config.yaml.example ./config.yaml

RUN mkdir -p /app/data /app/logs

EXPOSE 8080

CMD ["./apirelay"]
