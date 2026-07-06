FROM nginx:1.25-alpine
COPY nginx.conf.template /etc/nginx/nginx.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
COPY . /usr/share/nginx/html
EXPOSE 80
CMD ["/docker-entrypoint.sh"]
