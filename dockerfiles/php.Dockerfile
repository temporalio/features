# Build in a full featured container
FROM php:8.2-cli as build

# Install protobuf compiler
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
      protobuf-compiler=3.* libprotobuf-dev=3.* wget=* git=*

# Get go compiler
ARG PLATFORM=amd64
RUN wget -q https://go.dev/dl/go1.22.5.linux-${PLATFORM}.tar.gz \
    && tar -C /usr/local -xzf go1.22.5.linux-${PLATFORM}.tar.gz

# Install composer
COPY --from=composer:2.3 /usr/bin/composer /usr/bin/composer

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

# Prepare the feature for running. We need to use in-project venv so it is copied into smaller img.
RUN CGO_ENABLED=0 ./temporal-features prepare --lang php --dir prepared --version "$SDK_VERSION"

# Copy the CLI and prepared feature to a smaller container for running
FROM spiralscout/php-grpc:8.2

COPY --from=build /app/temporal-features /app/temporal-features
COPY --from=build /app/features /app/features
COPY --from=build /app/prepared /app/prepared
COPY --from=build /app/harness/php /app/harness/php
COPY --from=build /app/${REPO_DIR_OR_PLACEHOLDER} /app/${REPO_DIR_OR_PLACEHOLDER}

# Use entrypoint instead of command to "bake" the default command options
ENTRYPOINT ["/app/temporal-features", "run", "--lang", "php", "--prepared-dir", "prepared"]
