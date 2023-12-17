FROM golang:1.21.5-alpine AS build_deps

RUN apk add --no-cache git make

WORKDIR /workspace

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY . .

RUN make build

FROM gcr.io/distroless/static:nonroot

COPY --from=build /workspace/bin/webhook /bin/webhook

ENTRYPOINT ["/bin/webhook"]
