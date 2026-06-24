FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/chatvault ./cmd/chatvault

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin/chatvault /chatvault
WORKDIR /
ENTRYPOINT ["/chatvault"]
