FROM golang:1.25-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o receiver main.go

FROM public.ecr.aws/docker/library/alpine:3.20
WORKDIR /app
COPY --from=build /app/receiver /app/receiver

EXPOSE 8080
CMD ["/app/receiver"]