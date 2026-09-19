# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# SonarQube Server Community Edition Containerfile
# Built from SonarQube Community Edition distribution archive using Eclipse Temurin JDK 21.

FROM docker.io/library/eclipse-temurin:21-jdk

# Install base utilities and procps for health checks and process management
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    procps \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Configure unprivileged sonarqube user (UID 1000, GID 1000)
RUN if id ubuntu >/dev/null 2>&1; then \
        usermod -l sonarqube -d /opt/sonarqube -m ubuntu && \
        groupmod -n sonarqube ubuntu; \
    else \
        groupadd -g 1000 sonarqube && \
        useradd -u 1000 -g sonarqube -m -d /opt/sonarqube -s /bin/bash sonarqube; \
    fi

# Copy extracted SonarQube application directory
ARG SONARQUBE_DIR="sonarqube"
COPY --chown=sonarqube:sonarqube ${SONARQUBE_DIR} /opt/sonarqube

# Remove bundled unextracted JREs to minimize image footprint and set proper permissions
RUN rm -rf /opt/sonarqube/jres /opt/sonarqube/jre 2>/dev/null || true && \
    chmod -R a+rX /opt/sonarqube && \
    chmod -R +x /opt/sonarqube/bin/linux-x86-64 /opt/sonarqube/elasticsearch/bin && \
    mkdir -p /opt/sonarqube/data /opt/sonarqube/extensions /opt/sonarqube/logs /opt/sonarqube/temp && \
    chown -R sonarqube:sonarqube /opt/sonarqube

USER sonarqube
WORKDIR /opt/sonarqube

ENV JAVA_HOME=/opt/java/openjdk \
    SONAR_JAVA_PATH=/opt/java/openjdk/bin/java \
    PATH="/opt/java/openjdk/bin:${PATH}"

EXPOSE 9000

VOLUME ["/opt/sonarqube/data", "/opt/sonarqube/extensions", "/opt/sonarqube/logs"]

ENTRYPOINT ["/opt/sonarqube/bin/linux-x86-64/sonar.sh", "console"]
