from setuptools import Extension, setup

setup(name="bex-legacy-header-control", version="1.0.0",
      ext_modules=[Extension("legacy_probe", ["legacy_probe.c"])])
