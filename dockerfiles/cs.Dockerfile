# Build in a full featured container
FROM mcr.microsoft.com/dotnet/sdk:7.0-jammy as build

# Install protobuf compiler and build tools
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
      protobuf-compiler=3.12.4* libprotobuf-dev=3.12.4* build-essential=12.*

# Get go compiler
ARG PLATFORM=amd64
RUN wget -q https://go.dev/dl/go1.19.1.linux-${PLATFORM}.tar.gz \
    && tar -C /usr/local -xzf go1.19.1.linux-${PLATFORM}.tar.gz

# Install Rust for compiling the core bridge - only required for installation from a repo but is cheap enough to install
# in the "build" container (-y is for non-interactive install)
# hadolint ignore=DL4006
RUN wget -q -O - https://sh.rustup.rs | sh -s -- -y

ENV PATH="$PATH:/root/.cargo/bin:/usr/local/go/bin"

WORKDIR /app

# Copy CLI build dependencies
COPY features ./features
COPY harness ./harness
COPY sdkbuild ./sdkbuild
COPY cmd ./cmd
COPY go.mod go.sum main.go .editorconfig dotnet.csproj ./

# Build the CLI
RUN CGO_ENABLED=0 go build -o temporal-features

ARG SDK_VERSION
ARG SDK_REPO_URL
ARG SDK_REPO_REF
# Could be a cloned lang SDK git repo or just an arbitrary file so the COPY command below doesn't fail.
# It was either this or turn the Dockerfile into a template, this seemed simpler although a bit awkward.
ARG REPO_DIR_OR_PLACEHOLDER
COPY ./${REPO_DIR_OR_PLACEHOLDER} ./${REPO_DIR_OR_PLACEHOLDER}

# Prepare the feature for running
RUN CGO_ENABLED=0 ./temporal-features prepare --lang cs --dir prepared --version "$SDK_VERSION"

# Copy the CLI and prepared feature to a distroless "run" container
FROM mcr.microsoft.com/dotnet/sdk:7.0-jammy

COPY --from=build /app/temporal-features /app/temporal-features
COPY --from=build /app/features /app/features
COPY --from=build /app/prepared /app/prepared
# # Use entrypoint instead of command to "bake" the default command options
ENTRYPOINT ["/app/temporal-features", "run", "--lang", "cs", "--prepared-dir", "prepared"]