# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-watchlist/Dockerfile -t trading-agent-mfe-watchlist .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-watchlist/package*.json ui/mfe-watchlist/.npmrc ./ui/mfe-watchlist/
WORKDIR /app/ui/mfe-watchlist
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-watchlist/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-watchlist/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-watchlist/dist /usr/share/nginx/html
EXPOSE 80
