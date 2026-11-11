# A deliberately slow, nearly idle build: the build-concurrency probe
# (w6/m306, docs/ADR060 §D6/§D8). The build sleeps long enough for an elastic
# build node to scale from zero, so four of them in one workspace show whether
# builds 3 and 4 wait for admission (they start only when 1 and 2 finish) or
# for a node (they start once it joins, while 1 and 2 still run).
FROM busybox:1.36
ARG PROBE_SECONDS=420
RUN sleep "$PROBE_SECONDS"
RUN mkdir -p /www && echo ok > /www/index.html
EXPOSE 10000
CMD ["httpd", "-f", "-p", "10000", "-h", "/www"]
