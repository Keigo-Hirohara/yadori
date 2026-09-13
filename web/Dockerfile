FROM node:24-alpine AS build
ARG APP
ARG VITE_API_BASE_URL
ARG VITE_OIDC_AUTHORITY
WORKDIR /src
COPY ${APP}/package.json ${APP}/package-lock.json ./
RUN npm ci
COPY ${APP}/ .
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL} VITE_OIDC_AUTHORITY=${VITE_OIDC_AUTHORITY}
RUN npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
