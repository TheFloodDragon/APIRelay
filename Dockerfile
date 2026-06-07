# 构建前端
FROM node:20-alpine AS frontend-builder
WORKDIR /web

COPY web/package*.json ./
RUN npm ci --prefer-offline --no-audit

COPY web/ ./
RUN npm run build

# 构建后端，并将前端产物嵌入 Go 二进制
FROM golang:1.21-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /web/dist/ ./internal/ui/assets/

RUN go mod tidy && CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags "-s -w" -o apirelay ./cmd/server

# 最终镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite-libs
WORKDIR /app

COPY --from=backend-builder /app/apirelay ./apirelay
COPY config.yaml.example ./config.yaml

RUN mkdir -p /app/data /app/logs

EXPOSE 8080

CMD ["./apirelay"]
