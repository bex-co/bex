from setuptools import Extension,setup
setup(name='qa-native-helper',version='1.0',ext_modules=[Extension('qa_native_ext',['qa_native_ext.c'])])
