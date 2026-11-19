#!/bin/sh
# Prints every environment variable Render documents as a platform default
# (render.com/docs/environment-variables), plus PORT and XDG_CACHE_HOME, Render's
# build cache directory, which that page does not list (w9/m493), one line per
# name:
#
#   DEFAULT_ENV <phase> NAME=value
#   DEFAULT_ENV <phase> NAME unset
#
# Only these names are printed, never the rest of the environment, so a
# service's own variables and secrets stay out of the log. Run it as the first
# step of a build command (`sh probe.sh build`) and of a start command
# (`sh probe.sh runtime && exec …`). It is POSIX sh, so dash and busybox run it.
phase=${1:?usage: sh probe.sh build|runtime}
for name in \
  IS_PULL_REQUEST RENDER RENDER_CPU_COUNT RENDER_DISCOVERY_SERVICE \
  RENDER_EXTERNAL_HOSTNAME RENDER_EXTERNAL_URL RENDER_GIT_BRANCH \
  RENDER_GIT_COMMIT RENDER_GIT_REPO_SLUG RENDER_INSTANCE_ID RENDER_SERVICE_ID \
  RENDER_SERVICE_NAME RENDER_SERVICE_TYPE RENDER_WEB_CONCURRENCY \
  WEB_CONCURRENCY PORT \
  MIX_ENV RELEASE_DISTRIBUTION \
  GO111MODULE GOPATH \
  NODE_ENV NODE_MODULES_CACHE \
  CI FORWARDED_ALLOW_IPS GUNICORN_CMD_ARGS PIPENV_YES VENV_ROOT \
  BUNDLE_APP_CONFIG BUNDLE_BIN BUNDLE_DEPLOYMENT BUNDLE_PATH GEM_PATH \
  MALLOC_ARENA_MAX PASSENGER_ENGINE PASSENGER_ENVIRONMENT PASSENGER_PORT \
  PIDFILE RAILS_ENV RAILS_SERVE_STATIC_FILES RAILS_LOG_TO_STDOUT \
  CARGO_HOME ROCKET_ENV ROCKET_PORT RUSTUP_HOME \
  XDG_CACHE_HOME; do
  if eval "[ -n \"\${$name+set}\" ]"; then
    eval "value=\$$name"
    printf 'DEFAULT_ENV %s %s=%s\n' "$phase" "$name" "$value"
  else
    printf 'DEFAULT_ENV %s %s unset\n' "$phase" "$name"
  fi
done
