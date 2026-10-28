FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM build AS test
RUN go test ./...

FROM build AS integration-test
CMD ["go", "test", "-tags=integration", "-count=1", "-v", "./tests/integration"]

FROM alpine:3.22
RUN addgroup -S app && adduser -S app -G app
COPY --from=build /out/server /server
COPY --from=build /out/seed /seed
USER app
EXPOSE 8080
ENTRYPOINT ["/server"]

