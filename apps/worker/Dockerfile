FROM golang:1.26.2-trixie AS builder
WORKDIR /workspace

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

COPY . .
WORKDIR /workspace/apps/worker
RUN go mod download

RUN go build -o /out/worker ./

FROM debian:trixie-slim AS runner
WORKDIR /app

ENV TZ=Asia/Shanghai

COPY --from=builder /out/worker /usr/local/bin/zhblogs-worker

EXPOSE 9301
CMD ["zhblogs-worker"]
