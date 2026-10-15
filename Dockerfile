# Stage 1: Build UI
FROM node:20-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Stage 2: Build Go
FROM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /aiwx ./cmd/aiwx

# Stage 3: Final image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-builder /aiwx /aiwx
COPY --from=ui-builder /app/ui/dist /ui/dist
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/aiwx"]
