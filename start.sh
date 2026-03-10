#!/bin/sh
set -e

# Ensure nginx can safely operate temp buffers asynchronously
mkdir -p /run/nginx /tmp/client_temp /tmp/proxy_temp_path /tmp/fastcgi_temp /tmp/uwsgi_temp /tmp/scgi_temp

# Force Go SERVER_PORT to be internally bound to 8081 
# to avoid clashing with NGINX routing on Docker container 8080 mapping
export SERVER_PORT=8081

# Start NGINX reverse proxy in the background daemon mode
echo "Starting NGINX reverse proxy..."
nginx -c /app/nginx.conf

# Replace shell context perfectly with Go server for robust system signaling (PID 1)
echo "Starting Go API Server internally on port 8081..."
exec server
