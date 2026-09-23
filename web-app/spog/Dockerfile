# Build context is web-app/ (spog depends on file:../shared-components):
#   docker build -f spog/Dockerfile -t trading-agent-spog .
FROM node:22-alpine AS build
WORKDIR /app
COPY shared-components ./shared-components
COPY spog/package.json spog/package-lock.json ./spog/
WORKDIR /app/spog
RUN npm ci --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY spog/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY spog/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/spog/dist /usr/share/nginx/html
COPY spog/public/config.json /usr/share/nginx/html/config.json
EXPOSE 80
