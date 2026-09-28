# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-tracked/Dockerfile -t trading-agent-mfe-tracked .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-tracked/package*.json ui/mfe-tracked/.npmrc ./ui/mfe-tracked/
WORKDIR /app/ui/mfe-tracked
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-tracked/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-tracked/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-tracked/dist /usr/share/nginx/html
EXPOSE 80
