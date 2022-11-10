# While reading this file you might be wondering, hmmm.. this looks a lot like go.Dockerfile, well.. it does!
# The author (git blame to reveal) prefers some copying over templating the Dockerfiles.

# Build in a full featured container
FROM node:16 as build

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
      protobuf-compiler libprotobuf-dev

WORKDIR /app

# Copy CLI build dependencies
COPY features ./features
COPY harness ./harness
COPY cmd ./cmd
COPY go.mod go.sum main.go ./

ARG PLATFORM=amd64
RUN wget https://go.dev/dl/go1.19.1.linux-${PLATFORM}.tar.gz
RUN tar -C /usr/local -xzf go1.19.1.linux-${PLATFORM}.tar.gz
# Install Rust for compiling the core bridge - only required for installation from a repo but is cheap enough to install
# in the "build" container (-y is for non-interactive install)
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y

ENV PATH="$PATH:/root/.cargo/bin"

# Build the CLI
RUN CGO_ENABLED=0 /usr/local/go/bin/go build

ARG SDK_VERSION
ARG SDK_REPO_URL
ARG SDK_REPO_REF
# Could be a cloned lang SDK git repo or just an arbitrary file so the COPY command below doesn't fail.
# It was either this or turn the Dockerfile into a template, this seemed simpler although a bit awkward.
ARG REPO_DIR_OR_PLACEHOLDER
COPY ./${REPO_DIR_OR_PLACEHOLDER} ./${REPO_DIR_OR_PLACEHOLDER}

# Prepare the feature for running
RUN CGO_ENABLED=0 ./sdk-features prepare --lang ts --dir prepared --version "$SDK_VERSION"

# Copy the CLI and prepared feature to a distroless "run" container
FROM gcr.io/distroless/nodejs:16

COPY --from=build /app/sdk-features /app/sdk-features
COPY --from=build /app/features /app/features
COPY --from=build /app/prepared /app/prepared
COPY --from=build /app/${REPO_DIR_OR_PLACEHOLDER} /app/${REPO_DIR_OR_PLACEHOLDER}

# Node is installed here 👇 in distroless
ENV PATH="/nodejs/bin"
# Use entrypoint instead of command to "bake" the default command options
ENTRYPOINT ["/app/sdk-features", "run", "--lang", "ts", "--prepared-dir", "prepared"]