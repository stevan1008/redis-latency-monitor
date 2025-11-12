FROM golang:1.23.3-alpine AS builder

RUN apk add --no-cache git openssh bash build-base ca-certificates

ARG SSH_PRIVATE_KEY
ARG SSH_KEYSCAN
ARG BITBUCKET_USER
ARG BITBUCKET_PAT

# --- Configuración SSH idéntica a tu entorno local ---
RUN mkdir -p /root/.ssh \
    && echo "${SSH_PRIVATE_KEY}" > /root/.ssh/id_rsa \
    && chmod 600 /root/.ssh/id_rsa \
    && touch /root/.ssh/known_hosts \
    && echo "${SSH_KEYSCAN}" >> /root/.ssh/known_hosts

RUN echo "Host devops.bi.com.gt" >> /root/.ssh/config \
    && echo "    KexAlgorithms diffie-hellman-group1-sha1" >> /root/.ssh/config \
    && echo "    HostkeyAlgorithms +ssh-rsa" >> /root/.ssh/config \
    && echo "    PubkeyAcceptedKeyTypes ssh-ed25519,ssh-rsa,ecdsa-sha2-nistp256,ecdsa-sha2-nistp384,ecdsa-sha2-nistp521,ssh-ed25519-cert-v01@openssh.com,ssh-rsa-cert-v01@openssh.com,ecdsa-sha2-nistp256-cert-v01@openssh.com,ecdsa-sha2-nistp384-cert-v01@openssh.com,ecdsa-sha2-nistp521-cert-v01@openssh.com" >> /root/.ssh/config \
    && echo "    StrictHostKeyChecking no" >> /root/.ssh/config \
    && echo "    User git" >> /root/.ssh/config

# --- Config git para repos internos y bitbucket ---
RUN git config --global url."git@devops.bi.com.gt:".insteadOf "https://devops.bi.com.gt/"
RUN git config --global url."git@devops.bi.com.gt:BISistemas/Bi-en-linea-App-CI/_git/".insteadOf "https://devops.bi.com.gt/BISistemas/Bi-en-linea-App-CI/_git/"
RUN git config --global url."https://$BITBUCKET_USER@bitbucket.org/".insteadOf "https://bitbucket.org/"
RUN echo "machine bitbucket.org login $BITBUCKET_USER password $BITBUCKET_PAT" > ~/.netrc && chmod 600 ~/.netrc

ENV GOPRIVATE="devops.bi.com.gt/*,bitbucket.org/*"
RUN git config --global --add safe.directory /app

WORKDIR /app
COPY . .

# --- Compila (usa GIT_SSH_COMMAND para forzar tu clave y algoritmos) ---
RUN GIT_SSH_COMMAND="ssh -o KexAlgorithms=diffie-hellman-group1-sha1 -o HostkeyAlgorithms=+ssh-rsa -i /root/.ssh/id_rsa -o StrictHostKeyChecking=no" \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o redis-latency-monitor main.go

# =========================
# Etapa final
# =========================
FROM alpine:3.13.0 AS runner

RUN sed -ie "s/https/http/g" /etc/apk/repositories
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/redis-latency-monitor /app/

EXPOSE 8080 9090

CMD ["/app/redis-latency-monitor"]

