#!/bin/sh
set -e

# Substitute only ${APP_HOST} so nginx's own $variables are left untouched
envsubst '${APP_HOST}' < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf

exec nginx -g 'daemon off;'
