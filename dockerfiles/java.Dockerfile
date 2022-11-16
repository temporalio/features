# Build in a full featured container
FROM eclipse-temurin:11 as build

# Install protobuf compiler & git (needed if building SDK, rather than pulling it as dep)
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
      protobuf-compiler=3.12.4-1ubuntu7 git=1:2.34.1-1ubuntu1

ARG PLATFORM=amd64
RUN wget -q https://go.dev/dl/go1.19.1.linux-${PLATFORM}.tar.gz \
    && tar -C /usr/local -xzf go1.19.1.linux-${PLATFORM}.tar.gz

WORKDIR /app

# Copy CLI build dependencies
COPY gradle ./gradle
COPY gradlew build.gradle settings.gradle ./
COPY go.mod go.sum main.go ./
COPY cmd ./cmd
COPY harness ./harness
COPY features ./features

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
ENV GRADLE_USER_HOME="/gradle"
RUN CGO_ENABLED=0 ./sdk-features prepare --lang java --dir prepared --version "$SDK_VERSION"

# Copy the CLI and prepared feature to a "run" container. Distroless isn't used here since we run
# through Gradle and it's more annoying than it's worth to get its deps to line up
FROM eclipse-temurin:11
ENV GRADLE_USER_HOME="/gradle"

COPY --from=build /app/sdk-features /app/sdk-features
COPY --from=build /app/features /app/features
COPY --from=build /app/prepared /app/prepared
COPY --from=build /app/harness/java /app/harness/java
COPY --from=build /app/gradle /app/gradle
COPY --from=build /app/gradlew /app/build.gradle /app/settings.gradle /app/
COPY --from=build /gradle /gradle
# Use entrypoint instead of command to "bake" the default command options
ENTRYPOINT ["/app/sdk-features", "run", "--lang", "java", "--prepared-dir", "prepared"]