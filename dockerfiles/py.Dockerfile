# Build in a full featured container
FROM python:3.11-bullseye as build

# Install protobuf compiler
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
      protobuf-compiler=3.12.4-1 libprotobuf-dev=3.12.4-1

# Get go compiler
ARG PLATFORM=amd64
RUN wget -q https://go.dev/dl/go1.19.1.linux-${PLATFORM}.tar.gz \
    && tar -C /usr/local -xzf go1.19.1.linux-${PLATFORM}.tar.gz
# Install Rust for compiling the core bridge - only required for installation from a repo but is cheap enough to install
# in the "build" container (-y is for non-interactive install)
# hadolint ignore=DL4006
RUN wget -q -O - https://sh.rustup.rs | sh -s -- -y

ENV PATH="$PATH:/root/.cargo/bin"

# Install poetry
RUN pip install --no-cache-dir "poetry==1.2.2"

WORKDIR /app

# Copy CLI build dependencies
COPY features ./features
COPY harness ./harness
COPY cmd ./cmd
COPY go.mod go.sum main.go ./

# Build the CLI
RUN CGO_ENABLED=0 /usr/local/go/bin/go build

# Copy poetry config
COPY poetry.lock pyproject.toml ./

ARG SDK_VERSION
ARG SDK_REPO_URL
ARG SDK_REPO_REF
# Could be a cloned lang SDK git repo or just an arbitrary file so the COPY command below doesn't fail.
# It was either this or turn the Dockerfile into a template, this seemed simpler although a bit awkward.
ARG REPO_DIR_OR_PLACEHOLDER
COPY ./${REPO_DIR_OR_PLACEHOLDER} ./${REPO_DIR_OR_PLACEHOLDER}

# Prepare the feature for running. We need to use in-project venv so it is copied into smaller img.
ENV POETRY_VIRTUALENVS_IN_PROJECT=true
RUN CGO_ENABLED=0 ./features prepare --lang py --dir prepared --version "$SDK_VERSION"

# Copy the CLI and prepared feature to a smaller container for running
FROM python:3.11-slim-bullseye

# Poetry needed for running python tests
RUN pip install --no-cache-dir "poetry==1.2.2"

COPY --from=build /app/features /app/features
COPY --from=build /app/features /app/features
COPY --from=build /app/prepared /app/prepared
COPY --from=build /app/harness/python /app/harness/python
COPY --from=build /app/${REPO_DIR_OR_PLACEHOLDER} /app/${REPO_DIR_OR_PLACEHOLDER}
COPY --from=build /app/poetry.lock /app/pyproject.toml /app/

# Use entrypoint instead of command to "bake" the default command options
ENV POETRY_VIRTUALENVS_IN_PROJECT=true
ENTRYPOINT ["/app/features", "run", "--lang", "py", "--prepared-dir", "prepared"]