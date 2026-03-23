# Build in a full featured container
FROM ruby:4.0-bookworm as build

# Install protobuf compiler
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
    protobuf-compiler=3.21.12* libprotobuf-dev=3.21.12*
# Install Rust for compiling the core bridge - only required for installation from a repo but is cheap enough to install
# in the "build" container (-y is for non-interactive install)
# hadolint ignore=DL4006
RUN wget -q -O - https://sh.rustup.rs | sh -s -- -y

ENV PATH="$PATH:/root/.cargo/bin"

# Get go compiler
ARG PLATFORM=amd64
RUN wget -q https://go.dev/dl/go1.22.5.linux-${PLATFORM}.tar.gz \
    && tar -C /usr/local -xzf go1.22.5.linux-${PLATFORM}.tar.gz

WORKDIR /app

# Copy CLI build dependencies
COPY features ./features
COPY harness ./harness
COPY sdkbuild ./sdkbuild
COPY cmd ./cmd
COPY go.mod go.sum main.go ./

# Build the CLI
RUN CGO_ENABLED=0 /usr/local/go/bin/go build -o temporal-features

ARG SDK_VERSION
ARG SDK_REPO_URL
ARG SDK_REPO_REF
# Could be a cloned lang SDK git repo or just an arbitrary file so the COPY command below doesn't fail.
# It was either this or turn the Dockerfile into a template, this seemed simpler although a bit awkward.
ARG REPO_DIR_OR_PLACEHOLDER
COPY ./${REPO_DIR_OR_PLACEHOLDER} ./${REPO_DIR_OR_PLACEHOLDER}

# Override BUNDLE_APP_CONFIG so bundler reads .bundle/config from the prepared dir
# (the Ruby Docker image sets this to /usr/local/bundle which is lost in multi-stage builds)
ENV BUNDLE_APP_CONFIG=.bundle

# Prepare the feature for running.
RUN CGO_ENABLED=0 ./temporal-features prepare --lang rb --dir prepared --version "$SDK_VERSION"

# Copy the CLI and prepared feature to a smaller container for running
FROM ruby:4.0-slim-bookworm

COPY --from=build /app/temporal-features /app/temporal-features
COPY --from=build /app/features /app/features
COPY --from=build /app/prepared /app/prepared
COPY --from=build /app/harness/ruby /app/harness/ruby
COPY --from=build /app/${REPO_DIR_OR_PLACEHOLDER} /app/${REPO_DIR_OR_PLACEHOLDER}

# Override BUNDLE_APP_CONFIG so bundler reads .bundle/config from the prepared dir
ENV BUNDLE_APP_CONFIG=.bundle

# Use entrypoint instead of command to "bake" the default command options
ENTRYPOINT ["/app/temporal-features", "run", "--lang", "rb", "--prepared-dir", "prepared"]
