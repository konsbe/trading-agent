# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-market-report/Dockerfile -t trading-agent-mfe-market-report .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-market-report/package*.json ui/mfe-market-report/.npmrc ./ui/mfe-market-report/
WORKDIR /app/ui/mfe-market-report
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-market-report/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-market-report/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-market-report/dist /usr/share/nginx/html
EXPOSE 80
