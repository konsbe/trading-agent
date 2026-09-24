# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-backtest-lab/Dockerfile -t trading-agent-mfe-backtest-lab .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-backtest-lab/package*.json ui/mfe-backtest-lab/.npmrc ./ui/mfe-backtest-lab/
WORKDIR /app/ui/mfe-backtest-lab
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-backtest-lab/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-backtest-lab/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-backtest-lab/dist /usr/share/nginx/html
EXPOSE 80
