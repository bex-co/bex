#include <Python.h>
static PyObject *weighted(PyObject *self, PyObject *args) {
    long a,b;
    if (!PyArg_ParseTuple(args,"ll",&a,&b)) return NULL;
    return PyLong_FromLong(a*2+b*3);
}
static PyObject *dessert(PyObject *self, PyObject *args) {
    return PyUnicode_FromString("Crème brûlée & 李");
}
static PyMethodDef methods[]={
    {"weighted_sum",weighted,METH_VARARGS,"Own weighted sum"},
    {"dessert",dessert,METH_NOARGS,"Own Unicode result"},
    {NULL,NULL,0,NULL}
};
static struct PyModuleDef module={PyModuleDef_HEAD_INIT,"qa_native_ext",NULL,-1,methods};
PyMODINIT_FUNC PyInit_qa_native_ext(void) { return PyModule_Create(&module); }
