/usr/bin/python3 -m venv /tmp/bex-poetry-bootstrap \
  && /tmp/bex-poetry-bootstrap/bin/python -I -m pip --version \
  && rm -rf /tmp/bex-poetry-bootstrap \
  && /usr/bin/python3 /usr/share/bex-native-common/check-common.py
