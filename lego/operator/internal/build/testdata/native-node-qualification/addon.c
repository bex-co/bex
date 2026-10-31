#include <node_api.h>
#include <node_version.h>
#include <stdio.h>

static napi_value identity(napi_env env, napi_callback_info info) {
  napi_value result, value;
  char version[48];
  snprintf(version, sizeof(version), "%d.%d.%d", NODE_MAJOR_VERSION, NODE_MINOR_VERSION, NODE_PATCH_VERSION);
  if (napi_create_object(env, &result) != napi_ok) return NULL;
  if (napi_create_string_utf8(env, version, NAPI_AUTO_LENGTH, &value) != napi_ok ||
      napi_set_named_property(env, result, "headers", value) != napi_ok) return NULL;
  if (napi_create_uint32(env, NODE_MODULE_VERSION, &value) != napi_ok ||
      napi_set_named_property(env, result, "module_abi", value) != napi_ok) return NULL;
  if (napi_create_string_utf8(env, "native Grüße λ 雪", NAPI_AUTO_LENGTH, &value) != napi_ok ||
      napi_set_named_property(env, result, "answer", value) != napi_ok) return NULL;
  return result;
}

static napi_value init(napi_env env, napi_value exports) {
  napi_value function;
  if (napi_create_function(env, "identity", NAPI_AUTO_LENGTH, identity, NULL, &function) != napi_ok ||
      napi_set_named_property(env, exports, "identity", function) != napi_ok) return NULL;
  return exports;
}
NAPI_MODULE(NODE_GYP_MODULE_NAME, init)
