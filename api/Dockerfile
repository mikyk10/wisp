# --- Build image for local development
FROM public.ecr.aws/docker/library/golang:1.26.0-bookworm AS development

WORKDIR /app

COPY go.mod .
RUN go mod download

# For local development
ARG LOCAL_DEV
RUN if [ "${LOCAL_DEV}" = "true" ]; then \
      go install github.com/air-verse/air@latest; \
      go install github.com/go-delve/delve/cmd/dlv@latest; \
      go get github.com/idubinskiy/schematyper; go install github.com/idubinskiy/schematyper; fi

CMD ["air"]


# --- Build binary ---
FROM development AS builder
COPY . .
ARG GITHUB_REF_NAME
ARG GITHUB_SHA
RUN make


# --- Build image for orchestrated container
FROM public.ecr.aws/docker/library/debian:bookworm-slim AS runner

WORKDIR /app
ENV TZ=Asia/Tokyo

# Copy app binary/stuffs
COPY --from=builder /app/wspf .
COPY config config
COPY resources resources
#COPY --from=vue-builder /app/dist/ resources/public

ENTRYPOINT ["./wspf"]
CMD ["web", "run"] 
