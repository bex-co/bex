#include <erl_nif.h>
#include <ffi.h>
#include <openssl/sha.h>
#include <stdint.h>

static int64_t add_values(int64_t a, int64_t b) { return a + b; }

static ERL_NIF_TERM add(ErlNifEnv *env, int argc, const ERL_NIF_TERM argv[]) {
    ErlNifSInt64 a, b;
    if (argc != 2 || !enif_get_int64(env, argv[0], &a) ||
        !enif_get_int64(env, argv[1], &b) || a < -1000000 || a > 1000000 ||
        b < -1000000 || b > 1000000) return enif_make_badarg(env);
    ffi_cif cif;
    ffi_type *types[] = {&ffi_type_sint64, &ffi_type_sint64};
    void *values[] = {&a, &b};
    int64_t result = 0;
    if (ffi_prep_cif(&cif, FFI_DEFAULT_ABI, 2, &ffi_type_sint64, types) != FFI_OK)
        return enif_make_badarg(env);
    ffi_call(&cif, FFI_FN(add_values), &result, values);
    return enif_make_int64(env, result);
}

static ERL_NIF_TERM digest(ErlNifEnv *env, int argc, const ERL_NIF_TERM argv[]) {
    ErlNifBinary input;
    if (argc != 1 || !enif_inspect_binary(env, argv[0], &input) ||
        input.size > 8388608) return enif_make_badarg(env);
    ERL_NIF_TERM term;
    unsigned char *output = enif_make_new_binary(env, SHA256_DIGEST_LENGTH, &term);
    if (output == NULL || SHA256(input.data, input.size, output) == NULL)
        return enif_make_badarg(env);
    return term;
}

static ErlNifFunc functions[] = {{"add", 2, add, 0}, {"digest", 1, digest, 0}};
ERL_NIF_INIT(bex_nif, functions, NULL, NULL, NULL, NULL)
