FROM rust:1.99-slim-trixie AS build
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release --locked

FROM debian:trixie-slim
COPY --from=build /src/target/release/notifier /usr/local/bin/notifier
EXPOSE 7070
ENTRYPOINT ["/usr/local/bin/notifier"]
