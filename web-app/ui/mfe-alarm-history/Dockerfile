# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-alarm-history/Dockerfile -t trading-agent-mfe-alarm-history .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-alarm-history/package*.json ui/mfe-alarm-history/.npmrc ./ui/mfe-alarm-history/
WORKDIR /app/ui/mfe-alarm-history
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-alarm-history/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-alarm-history/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-alarm-history/dist /usr/share/nginx/html
EXPOSE 80
