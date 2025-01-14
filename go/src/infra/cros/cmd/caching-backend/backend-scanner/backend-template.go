// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

// nginxTemplate is to generate the configuration for Nginx, the caching service
// backend.
const nginxTemplate = `
# This file is generated. DO NOT EDIT!

user www-data;
worker_processes auto;
worker_rlimit_nofile 65535;

pid        /var/run/nginx.pid;
error_log  /var/log/nginx/error.log error;

{{ if .OtelTraceEndpoint }}
load_module /opt/opentelemetry-webserver-sdk/WebServerModule/Nginx/ngx_http_opentelemetry_module.so;
{{ end }}

events {
  accept_mutex on;
  accept_mutex_delay 500ms;
  worker_connections 1024;
}

http {
  default_type  application/octet-stream;

  log_format main_json escape=json
  '{'
    '"access_time":"$time_iso8601",'
    '"bytes_sent":$body_bytes_sent,'
    '"content_length":$sent_http_content_length,'
    '"host":"$host",'
    '"hostname":"$hostname",'
    '"method":"$request_method",'
    '"proxy_host":"$proxy_host",'
    '"referer":"$http_referer",'
    '"remote_addr":"$remote_addr",'
    '"remote_user":"$remote_user",'
    '"request":"$request",'
    '"request_time":$request_time,'
    '"status":$status,'
    '"uri":"$uri",'
    '"user_agent":"$http_user_agent",'
    '"upstream":"$upstream_addr",'
    '"upstream_cache_status":"$upstream_cache_status",'
    '"upstream_response_time":"$upstream_response_time",'
    '"swarming_task_id": "$http_x_swarming_task_id",'
    '"bbid": "$http_x_bbid",'
    '"x_forwarded_for":"$http_x_forwarded_for"'
  '}';

  proxy_cache_path  /var/cache/nginx levels=1:2 keys_zone=google-storage:80m
                    max_size={{ .CacheSizeInGB }}g inactive=720h;
  proxy_temp_path   /var/cache/nginx/proxy_temp 1 2 2;
  proxy_cache           google-storage;
  proxy_connect_timeout 90;
  proxy_read_timeout    3600;
  proxy_redirect        off;
  proxy_http_version    1.1;
  proxy_cache_bypass    $http_x_no_cache;
  proxy_set_header      Connection "";
  proxy_set_header      X-SWARMING-TASK-ID $http_x_swarming_task_id;
  proxy_set_header      X-BBID $http_x_bbid;
  proxy_set_header      X-Forwarded-For $proxy_add_x_forwarded_for;
  proxy_set_header      X-NO-CACHE $http_x_no_cache;
  proxy_cache_valid     200 720h;
  send_timeout          3600;
  proxy_ignore_client_abort on;
  expires max;

  upstream downloader{
    server downloader-svc;
  }

  upstream l7_upstream {
    hash $uri$is_args$args consistent;
{{ range .L7Servers }}
    server {{ .IP }}:{{ $.L7Port }};
{{else}}
    # For bootstrapping.
    server downloader-svc;
{{ end }}
  }
  server {
    listen *:{{ .Port }};
    server_name           gs-cache-l4;
    index                 index.html index.htm index.php;
    access_log            /dev/stdout main_json;
    error_log             /dev/stdout info;

    add_header            'X-Cache-L4' '$upstream_cache_status';
    add_header            'X-CACHING-BACKEND-L4' '$hostname';
    add_header            'X-SWARMING-TASK-ID' '$http_x_swarming_task_id';
    add_header            'X-BBID' '$http_x_bbid';

    location / {
      proxy_pass            http://l7_upstream;
      proxy_cache_key       $request_method$uri$is_args$args;
    }

    # b/281868022: A location dedicated for AU tests.
    location ~ ^/swarming/(?<swarming>[^/]+)/bbid/(?<bbid>[^/]+)/(?<end>.*)$ {
      set $http_x_swarming_task_id "$swarming";
      set $http_x_bbid "$bbid";
      rewrite .* "/$end" last;
    }

    # Rewrite rules converting devserver client requests to gs_cache.
    location @gs_cache {
      if ($arg_gs_bucket = "") {
        set $arg_gs_bucket "chromeos-image-archive";
      }
      # The ending '?' erase any query string from the incoming request.
      rewrite "^/static/([^/]+-channel/.+)$" "/download/chromeos-releases/$1?" last;
      rewrite "^/static/([^/]+/[^/]+)/(autotest/packages)/(.*)" "/extract/$arg_gs_bucket/$1/autotest_packages.tar?file=$2/$3?" last;
      rewrite "^/static/([^/]+/[^/]+/chromiumos_test_image)\.bin$" "/extract/$arg_gs_bucket/$1.tar.xz?file=chromiumos_test_image.bin?" last;
      rewrite "^/static/([^/]+/[^/]+/recovery_image)\.bin$" "/extract/$arg_gs_bucket/$1.tar.xz?file=recovery_image.bin?" last;
      rewrite "^/static/(.+)$" "/download/$arg_gs_bucket/$1?" last;
    }
    # Some legacy RPCs in order to be backward compatible with devserver.
    location /check_health {
      default_type application/json;
      return 200 '{"disk_total_bytes_per_second": 0, "network_total_bytes_per_second": 0, "network_sent_bytes_per_second": 0, "apache_client_count": 0, "disk_write_bytes_per_second": 0, "cpu_percent": 0, "disk_read_bytes_per_second": 0, "gsutil_count": 0, "network_recv_bytes_per_second": 0, "free_disk": 5678, "au_process_count": 0, "staging_thread_count": 0, "telemetry_test_count": 0}';
    }
    location /stage {
      return 200 'Success';
    }
    location /is_staged {
      return 200 'True';
    }
    location = /download/chromeos-image-archive {
      return 400;
    }
    location = /static {
      alias /var/www/nginx_static;
      autoindex on;
    }
    location /static/ {
      alias /var/www/nginx_static/;
      try_files $uri @gs_cache;
    }
    location /list_image_dir {
      return 200 'The /list_image_dir RPC is not supported by GS Cache. Usage is discouraged.';
    }

    location /gscache {
      return 200 'I am gscache.';
    }
  }

  server {
    listen                *:{{ .L7Port }};
    server_name           gs-cache-l7;
    access_log            /dev/stdout main_json;
    error_log             /dev/stdout info;

    add_header            'X-Cache-L7' '$upstream_cache_status';
    add_header            'X-CACHING-BACKEND-L7' '$hostname';

    proxy_cache_lock on;
    proxy_cache_lock_age 900s;
    proxy_cache_lock_timeout 900s;

    # CQ build cache configuration.
    # The configuration is exactly same with the "location /" except
    # "proxy_cache_valid" which is much shorter than a release build.
    # A CQ build URL is like "/download/chromeos-image-archive/coral-cq/R92-13913.0.0-46943-8850024658050820208/...".
    location ~ ^/download/[^/]+/[^/]+-cq/ {
      slice 30m;
      proxy_pass            http://downloader;
      proxy_cache_valid     200 206 48h;
      proxy_cache_key       $request_method$uri$is_args$args$slice_range;
      proxy_set_header      Range $slice_range;
      proxy_force_ranges    on;
    }
    location ~ ^/[^/]+/[^/]+/[^/]+-cq/ {
      proxy_pass            http://downloader;
      proxy_cache_valid     200 48h;
      proxy_cache_key       $request_method$uri$is_args$args;
    }

    # The difference of location '/' and '/download' is that we use slice
    # downloading in '/download', which doesn't work for other RPCs like
    # '/extract' etc.
    location / {
      proxy_pass            http://downloader;
      proxy_cache_key       $request_method$uri$is_args$args;
    }
    # Android build doesn't support slice downloading.
    location ~ ^/download/android-build/ {
        proxy_pass            http://downloader;
        proxy_cache_valid     200 720h;
        proxy_cache_key       $request_method$uri$is_args$args;
        proxy_force_ranges    on;
      }
    location ~ ^/download/ {
        slice 30m;
        proxy_pass            http://downloader;
        proxy_cache_valid     200 206 720h;
        proxy_cache_key       $request_method$uri$is_args$args$slice_range;
        proxy_set_header      Range $slice_range;
        proxy_force_ranges    on;
      }
  }

{{ if .OtelTraceEndpoint }}
  NginxModuleEnabled ON;
  NginxModuleOtelSpanExporter otlp;
  NginxModuleOtelExporterEndpoint collector-collector:4317;
  NginxModuleServiceName CachingBackendNginx;
  NginxModuleServiceNamespace CachingBackendNginx;
  NginxModuleServiceInstanceId CachingBackendNginxId;
  NginxModuleResolveBackends ON;
{{ end }}
}

`
