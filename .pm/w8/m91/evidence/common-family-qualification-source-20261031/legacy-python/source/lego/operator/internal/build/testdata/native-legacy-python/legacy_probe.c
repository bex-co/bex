#include <Python.h>

static PyObject *answer(PyObject *self, PyObject *args) {
  return Py_BuildValue("{s:s,s:i,s:s}", "headers", PY_VERSION,
                       "number", 7 * 17 + 3 * -4, "label", "Crème brûlée & 李");
}

static PyMethodDef methods[] = {
  {"answer", answer, METH_NOARGS, "Return compiled header and UTF-8 evidence."},
  {NULL, NULL, 0, NULL}
};
static struct PyModuleDef module = {
  PyModuleDef_HEAD_INIT, "legacy_probe", NULL, -1, methods
};
PyMODINIT_FUNC PyInit_legacy_probe(void) { return PyModule_Create(&module); }
