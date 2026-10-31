-module(bex_probe).
-export([check/0]).

check() ->
    {ok, _} = application:ensure_all_started(ssl),
    Text = unicode:characters_to_binary([98,101,120,32,99,97,102,233,32,955,32,38634]),
    Digest = crypto:hash(sha256, Text),
    Digest = bex_nif:digest(Text),
    42 = bex_nif:add(17, 25),
    io_lib:format("~s|42|~s", [base64:encode(Text), binary:encode_hex(Digest)]).
