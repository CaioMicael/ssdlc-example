FROM node:22-alpine AS web
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26.8-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/api /app/api
COPY --from=web /src/dist /web
ENV PORT=8080 WEB_DIR=/web
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
