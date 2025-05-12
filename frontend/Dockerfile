FROM node:19-alpine as vite-app

WORKDIR /app
COPY . .

RUN npm i
RUN npm run build

FROM nginx:alpine

WORKDIR /

COPY ./nginx/nginx.conf /etc/nginx
COPY --from=vite-app ./app/dist /usr/share/nginx/html

ENTRYPOINT ["nginx", "-g", "daemon off;"]