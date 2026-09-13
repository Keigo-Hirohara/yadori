FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -o /out/seed ./cmd/seed

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Tokyo
WORKDIR /app
COPY --from=build /out/ /app/
COPY db/migrations /app/db/migrations
USER nobody
CMD ["/app/api"]
