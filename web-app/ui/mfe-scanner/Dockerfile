# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-scanner/Dockerfile -t trading-agent-mfe-scanner .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-scanner/package*.json ui/mfe-scanner/.npmrc ./ui/mfe-scanner/
WORKDIR /app/ui/mfe-scanner
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-scanner/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-scanner/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-scanner/dist /usr/share/nginx/html
EXPOSE 80
