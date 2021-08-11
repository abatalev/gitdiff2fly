# GitDiff2fly
FROM golang:1.16.5 as golang_build
COPY ./ /src
RUN cd /src/; go build .

# Flyway
FROM bellsoft/liberica-openjdk-alpine-musl:11.0.11-9 as flyway_build
RUN apk --no-cache add --update bash openssl git

# Add the flyway user and step in the directory
RUN addgroup flyway \
    && adduser -S -h /flyway -D -G flyway flyway

WORKDIR /flyway

ENV FLYWAY_VERSION 7.10.0

RUN wget https://repo1.maven.org/maven2/org/flywaydb/flyway-commandline/${FLYWAY_VERSION}/flyway-commandline-${FLYWAY_VERSION}.tar.gz \
  && tar -xzf flyway-commandline-${FLYWAY_VERSION}.tar.gz \
  && mv flyway-${FLYWAY_VERSION}/* . \
  && rm flyway-commandline-${FLYWAY_VERSION}.tar.gz

COPY --from=golang_build /src/gitdiff2fly /opt/app/