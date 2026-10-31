-module(bex_nif).
-on_load(init/0).
-export([add/2, digest/1]).

init() -> erlang:load_nif(filename:absname("priv/bex_nif"), 0).
add(_, _) -> erlang:nif_error(not_loaded).
digest(_) -> erlang:nif_error(not_loaded).
