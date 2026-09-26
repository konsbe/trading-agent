# Build context is web-app/ (the MFE compiles ../../shared-components from source):
#   docker build -f ui/mfe-education/Dockerfile -t trading-agent-mfe-education .
FROM node:22-alpine AS build
ARG MOMENTUM_API_URL=http://localhost:8090
ENV MOMENTUM_API_URL=${MOMENTUM_API_URL}
WORKDIR /app
COPY shared-components ./shared-components
COPY ui/mfe-education/package*.json ui/mfe-education/.npmrc ./ui/mfe-education/
WORKDIR /app/ui/mfe-education
RUN npm install --legacy-peer-deps --registry=https://registry.npmjs.org/
COPY ui/mfe-education/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY ui/mfe-education/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/ui/mfe-education/dist /usr/share/nginx/html
EXPOSE 80
