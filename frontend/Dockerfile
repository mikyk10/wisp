# 1. Base image
FROM node:22-alpine AS builder

# 2. Set working directory
WORKDIR /app

# 3. Copy package files and install dependencies
COPY package.json package-lock.json ./
RUN npm install --frozen-lockfile

# 4. Copy application source code
COPY . .

# 5. Build the application
RUN npm run build

# 6. Production stage: Serve the built application with nginx
FROM nginx:1.27-alpine AS production

# Upgrade all packages to get latest security patches
RUN apk upgrade --no-cache

# Copy built artifacts from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Provide an nginx config that supports Vue Router (history API) deployments
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Entrypoint generates /usr/share/nginx/html/env.js from runtime env vars, then starts nginx
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Expose the port the app runs on
EXPOSE 80

ENTRYPOINT ["/docker-entrypoint.sh"]
