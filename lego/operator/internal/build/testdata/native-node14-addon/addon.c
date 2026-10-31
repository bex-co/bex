#include <node_api.h>

static napi_value answer(napi_env env, napi_callback_info info) {
  napi_value value;
  if (napi_create_string_utf8(env, "native addon: Grüße ∑ 42", NAPI_AUTO_LENGTH, &value) != napi_ok) return NULL;
  return value;
}

static napi_value init(napi_env env, napi_value exports) {
  napi_value function;
  if (napi_create_function(env, "answer", NAPI_AUTO_LENGTH, answer, NULL, &function) != napi_ok) return NULL;
  if (napi_set_named_property(env, exports, "answer", function) != napi_ok) return NULL;
  return exports;
}

NAPI_MODULE(NODE_GYP_MODULE_NAME, init)
