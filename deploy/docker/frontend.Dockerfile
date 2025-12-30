FROM nginx:alpine

WORKDIR /usr/share/nginx/html
COPY frontend/index.html ./index.html
COPY frontend/styles.css ./styles.css
COPY frontend/app.js ./app.js
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
