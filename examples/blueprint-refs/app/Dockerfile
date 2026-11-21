# The blueprint-refs app (w8/375): report.sh writes one line per Blueprint
# reference it was given, then busybox httpd serves that report at GET /.
FROM busybox:1.37.0@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e
COPY report.sh /report.sh
CMD ["sh", "/report.sh"]
