FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    nginx \
    curl \
    procps \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /var/log/nginx /services

COPY service-manager /usr/local/bin/tm-orchestrator
COPY services.toml /services.toml

RUN chmod +x /usr/local/bin/tm-orchestrator && \
    ln -sf /usr/local/bin/tm-orchestrator /usr/local/bin/entrypoint

RUN echo '#!/bin/bash\necho "Logger service started"\nwhile true; do\n  echo "$(date): Log entry from logger service"\n  sleep 5\ndone' > /services/logger.sh && \
    chmod +x /services/logger.sh

RUN echo '#!/bin/bash\necho "Background task started"\nwhile true; do\n  echo "$(date): Background task running..."\n  sleep 10\ndone' > /services/background-task.sh && \
    chmod +x /services/background-task.sh

EXPOSE 80

ENTRYPOINT ["tm-orchestrator"]
