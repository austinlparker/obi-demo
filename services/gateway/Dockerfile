FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Keep the symbol table (no -ldflags "-s -w"): OBI's Go probes find
# net/http and gRPC functions by symbol name.
RUN CGO_ENABLED=0 go build -o /out/gateway .

FROM debian:trixie-slim
COPY --from=build /out/gateway /usr/local/bin/gateway
EXPOSE 8088
ENTRYPOINT ["/usr/local/bin/gateway"]
