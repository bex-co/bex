# Build stage
FROM node:22-alpine AS builder

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

# Build the application (produces .output/)
RUN yarn build

# Runtime stage
FROM node:22-alpine

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000

# Copy built output
COPY --from=builder /app/.output ./.output

EXPOSE 3000

CMD ["node", ".output/server/index.mjs"]
