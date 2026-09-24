# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-data-source/Dockerfile -t trading-agent-mfe-data-source .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-data-source/package*.json ui/mfe-data-source/.npmrc ./ui/mfe-data-source/
WORKDIR /app/ui/mfe-data-source
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-data-source/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-data-source/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-data-source/dist /usr/share/nginx/html
EXPOSE 80
