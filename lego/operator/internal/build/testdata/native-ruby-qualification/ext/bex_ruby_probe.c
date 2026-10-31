#include <ruby.h>
#include <ruby/version.h>

static VALUE compiled_api(VALUE self) {
    (void)self;
    return rb_ary_new_from_args(2, INT2NUM(RUBY_API_VERSION_MAJOR), INT2NUM(RUBY_API_VERSION_MINOR));
}

static VALUE runtime_version(VALUE self) {
    (void)self;
    return rb_str_new_cstr(ruby_version);
}

static VALUE echo(VALUE self, VALUE text) {
    (void)self;
    Check_Type(text, T_STRING);
    return rb_str_dup(text);
}

void Init_bex_ruby_probe(void) {
    VALUE probe = rb_define_module("BexRubyProbe");
    rb_define_singleton_method(probe, "compiled_api", compiled_api, 0);
    rb_define_singleton_method(probe, "runtime_version", runtime_version, 0);
    rb_define_singleton_method(probe, "echo", echo, 1);
}
