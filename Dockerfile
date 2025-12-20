# Stage 1: Build the Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build the Backend
FROM golang:1.23-alpine AS backend-builder
RUN apk add --no-cache build-base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy frontend build from previous stage
COPY --from=frontend-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=1 GOOS=linux go build -o main .

# Stage 3: Final Image
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

# Copy binary
COPY --from=backend-builder /app/main .
# The app expects web/dist to be present relative to the binary
COPY --from=frontend-builder /app/web/dist ./web/dist

# Expose port
EXPOSE 8090

# Persistence volumes
VOLUME ["/app/data", "/config"]

# Ensure directories exist
RUN mkdir -p /app/data /config

CMD ["./main"]
