# ---- build stage ----
FROM node:22-alpine AS build
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
# Rewrites in next.config.mjs are baked in at build time, so the backend
# address (the compose service name) must be known here.
ARG BACKEND_URL=http://backend:8080
ENV BACKEND_URL=$BACKEND_URL \
    NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ---- runtime stage ----
# output: "standalone" puts a self-contained server.js (with the rewrites and
# only the node_modules it uses) in .next/standalone. Static assets and
# public/ are not part of it and are copied next to it.
FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

# Owned by and run as the image's unprivileged "node" user, not root.
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public
USER node

EXPOSE 3000
CMD ["node", "server.js"]
