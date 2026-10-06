FROM node:24-slim AS build

# The lockfile was written by npm 11.6; newer npm rejects it under `npm ci`.
RUN npm install -g npm@11.6.2

WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npx nx build www --configuration=production

FROM nginx:alpine AS runtime

COPY apps/www/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /build/dist/apps/www/browser /usr/share/nginx/html

EXPOSE 80
