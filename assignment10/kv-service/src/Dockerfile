# Build stage
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
# Binary arch follows the build platform (Terraform uses linux/arm64 for Fargate ARM64).
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Run stage (amd64 image matches Fargate default; use buildx for multi-arch if needed)
FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/server /app/server
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/app/server"]
