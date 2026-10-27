FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/supplier-hub ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/supplier-hub /supplier-hub
EXPOSE 8080
ENTRYPOINT ["/supplier-hub"]
