FROM node:22-alpine AS build

ARG APP
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY apps/admin/package.json apps/admin/package.json
COPY apps/web/package.json apps/web/package.json
COPY packages/api-client/package.json packages/api-client/package.json
COPY packages/ui/package.json packages/ui/package.json
RUN pnpm install --frozen-lockfile
COPY apps ./apps
COPY packages ./packages
RUN pnpm --filter "@snap/${APP}" build \
    && if [ "$APP" = "web" ]; then cp -R apps/admin/public/source-logos apps/web/dist/source-logos; fi \
    && cp -R "apps/${APP}/dist" /out

FROM nginx:1.29-alpine

COPY infra/production/frontend.conf /etc/nginx/conf.d/default.conf
COPY --from=build /out/ /usr/share/nginx/html/
