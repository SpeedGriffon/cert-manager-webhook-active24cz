FROM golang:1.26 AS build
WORKDIR /workspace

COPY go.mod go.sum .
RUN go mod download

COPY --parents api/client.go main.go .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .


FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /workspace/webhook /

ENTRYPOINT ["/webhook"]
