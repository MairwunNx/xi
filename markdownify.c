#include <Python.h>
#include <string.h>
#include <stdlib.h>
#include "markdownify.h"

static int python_initialized = 0;
static PyObject *telegramify_module = NULL;
static PyObject *markdownify_func = NULL;

int init_python()
{
  if (python_initialized)
  {
    return 1;
  }

  if (!Py_IsInitialized())
  {
    Py_Initialize();
    if (!Py_IsInitialized())
    {
      return 0;
    }
  }

  telegramify_module = PyImport_ImportModule("telegramify_markdown");
  if (!telegramify_module)
  {
    PyErr_Print();
    return 0;
  }

  markdownify_func = PyObject_GetAttrString(telegramify_module, "markdownify");
  if (!markdownify_func || !PyCallable_Check(markdownify_func))
  {
    PyErr_Print();
    return 0;
  }

  PyObject *customize_module = PyImport_ImportModule("telegramify_markdown.customize");
  if (customize_module)
  {
    PyObject *cite_expandable = PyBool_FromLong(1);
    PyObject_SetAttrString(customize_module, "cite_expandable", cite_expandable);
    Py_DECREF(cite_expandable);

    PyObject *strict_markdown = PyBool_FromLong(0);
    PyObject_SetAttrString(customize_module, "strict_markdown", strict_markdown);
    Py_DECREF(strict_markdown);

    Py_DECREF(customize_module);
  }

  python_initialized = 1;
  return 1;
}

char *markdownify(const char *markdown_text)
{
  if (!markdown_text)
  {
    return NULL;
  }

  if (!init_python())
  {
    return strdup(markdown_text);
  }

  PyObject *py_text = PyUnicode_FromString(markdown_text);
  if (!py_text)
  {
    return strdup(markdown_text);
  }

  PyObject *args = PyTuple_New(1);
  PyTuple_SetItem(args, 0, py_text);

  PyObject *kwargs = PyDict_New();
  PyDict_SetItemString(kwargs, "max_line_length", Py_None);
  PyDict_SetItemString(kwargs, "normalize_whitespace", Py_True);

  PyObject *result = PyObject_Call(markdownify_func, args, kwargs);

  char *result_str = NULL;
  if (result && PyUnicode_Check(result))
  {
    const char *utf8_str = PyUnicode_AsUTF8(result);
    if (utf8_str)
    {
      result_str = strdup(utf8_str);
    }
  }

  Py_XDECREF(result);
  Py_DECREF(kwargs);
  Py_DECREF(args);

  return result_str ? result_str : strdup(markdown_text);
}

void free_result(char *result)
{
  if (result)
  {
    free(result);
  }
}