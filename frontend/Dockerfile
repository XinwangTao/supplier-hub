FROM node:22-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm install
FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# 容器编排下反代目标用服务名；默认本地 8080，可用 --build-arg 覆盖
ARG API_PROXY_ORIGIN=http://127.0.0.1:8080
ENV API_PROXY_ORIGIN=$API_PROXY_ORIGIN
RUN npm run build
FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
EXPOSE 3000
CMD ["node", "server.js"]
