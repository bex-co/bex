# Build stage
FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS builder

WORKDIR /app

# Copy package files
COPY package.json .yarnrc.yml ./
COPY yarn.lock* ./

# Install dependencies (need devDependencies for building)
RUN corepack enable && yarn install --frozen-lockfile

# Copy source files
COPY . .

# Build arg for API URL (required at build time)
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL

# Optional: Internal API URL for SSR (defaults to VITE_API_URL if not set)
ARG VITE_SSR_API_URL
ENV VITE_SSR_API_URL=$VITE_SSR_API_URL

# Build arg for Ory Kratos public API (required at build time — docs/ADR012-auth.md)
ARG VITE_KRATOS_PUBLIC_URL
ENV VITE_KRATOS_PUBLIC_URL=$VITE_KRATOS_PUBLIC_URL

# Optional: in-cluster Kratos URL for SSR (defaults to VITE_KRATOS_PUBLIC_URL)
ARG VITE_KRATOS_SSR_URL
ENV VITE_KRATOS_SSR_URL=$VITE_KRATOS_SSR_URL

# Public signing identities for verified mobile invitation links. Both are
# optional together: absent keeps the association documents valid but disabled;
# a partial or malformed configuration fails the build. These values identify
# signing certificates and are not credentials.
ARG BEX_MOBILE_APPLE_TEAM_ID
ENV BEX_MOBILE_APPLE_TEAM_ID=$BEX_MOBILE_APPLE_TEAM_ID
ARG BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS
ENV BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS=$BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS

# Build the application (produces .output/)
RUN yarn build

# Runtime stage
FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000

# The server only needs the Node runtime. Drop npm/corepack from the final
# image so their unused dependency trees do not add executable attack surface
# (npm bundled a vulnerable node-tar even though the app lockfile was fixed).
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack \
      /opt/yarn-v1.22.22 && \
    rm -f /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
      /usr/local/bin/yarn /usr/local/bin/yarnpkg /usr/local/bin/pnpm /usr/local/bin/pnpx

# Copy built output — owned by the non-root `node` user node:22-alpine ships
# (readOnlyRootFilesystem: false in dashboard/deploy lets it write its cache).
COPY --from=builder --chown=node:node /app/.output ./.output

USER node

EXPOSE 3000

CMD ["node", ".output/server/index.mjs"]
