# Fixture for w1/m178: proves a Dockerfile build sees the service's env vars as
# build args and its secret files as build secrets, the way Render passes them.
FROM busybox:1.37.0@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e
# GREETING is a service env var; Render (and bex) pass it as a build arg.
ARG GREETING
# `.env` is a service secret file; its build-secret id is `_env`. Only its size
# is written into the image, never its content.
RUN --mount=type=secret,id=_env,required=true \
    mkdir -p /www && \
    test -n "$GREETING" && \
    printf '%s\n' "$GREETING" > /www/greeting && \
    wc -c < /run/secrets/_env | tr -d ' ' > /www/env-bytes
EXPOSE 3000
CMD ["httpd", "-f", "-v", "-p", "3000", "-h", "/www"]
