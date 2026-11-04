FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/crawler ./cmd/crawler

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/crawler /crawler
COPY --from=build /src/db/migrations /db/migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/crawler"]
