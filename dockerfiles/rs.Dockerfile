# syntax=docker/dockerfile:1
FROM golang:1.26.0-bookworm AS go-build
WORKDIR /app
COPY features ./features
COPY harness ./harness
COPY sdkbuild ./sdkbuild
COPY cmd ./cmd
COPY go.mod go.sum main.go ./
RUN CGO_ENABLED=0 go build -o temporal-features

FROM debian:bookworm-slim AS build
ARG TARGETARCH
# hadolint ignore=DL3008
RUN apt-get update \
    && apt-get install --no-install-recommends --assume-yes ca-certificates curl unzip python3 build-essential pkg-config \
    && rm -rf /var/lib/apt/lists/*
ENV CARGO_HOME=/usr/local/cargo RUSTUP_HOME=/usr/local/rustup
ENV PATH="${CARGO_HOME}/bin:${PATH}"
RUN curl --fail --silent --show-error --location https://sh.rustup.rs --output /tmp/rustup-init.sh \
    && sh /tmp/rustup-init.sh -y --no-modify-path --default-toolchain none \
    && rm /tmp/rustup-init.sh
WORKDIR /app
COPY --from=go-build /app /app
WORKDIR /app/harness/rust
RUN python3 -c 'import subprocess, tomllib; \
    toolchain = tomllib.load(open("rust-toolchain.toml", "rb"))["toolchain"]; \
    subprocess.run(["rustup", "toolchain", "install", toolchain["channel"], \
                    "--profile", toolchain["profile"], \
                    "--component", ",".join(toolchain["components"])], check=True)' \
    && cargo --version \
    && case "$TARGETARCH" in \
      amd64) arch=x86_64 ;; \
      arm64) arch=aarch_64 ;; \
      *) echo "Unsupported protoc architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && protoc_version="$(python3 -c 'import tomllib; print(tomllib.load(open("Cargo.toml", "rb"))["package"]["metadata"]["harness"]["protoc-version"])')" \
    && curl --fail --silent --show-error --location \
      "https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/protoc-${protoc_version}-linux-${arch}.zip" \
      --output /tmp/protoc.zip \
    && unzip -q /tmp/protoc.zip -d /usr/local \
    && rm /tmp/protoc.zip
WORKDIR /app
COPY dockerfiles/dynamicconfig ./dockerfiles/dynamicconfig
ARG SDK_VERSION
ARG REPO_DIR_OR_PLACEHOLDER
COPY ./${REPO_DIR_OR_PLACEHOLDER} ./${REPO_DIR_OR_PLACEHOLDER}
RUN ./temporal-features prepare --lang rs --dir prepared --version "$SDK_VERSION"

# The CLI still discovers feature.rs and compares filesystem history fixtures.
# Only the native executable is needed from the prepared Cargo package.
FROM debian:bookworm-slim AS runtime
# hadolint ignore=DL3008
RUN apt-get update \
    && apt-get install --no-install-recommends --assume-yes ca-certificates libgcc-s1 libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /app/temporal-features /app/temporal-features
COPY --from=build /app/prepared/program /app/prepared/program
COPY --from=build /app/features /app/features
COPY --from=build /app/dockerfiles/dynamicconfig /app/dockerfiles/dynamicconfig
ENTRYPOINT ["/app/temporal-features", "run", "--lang", "rs", "--prepared-dir", "prepared"]
