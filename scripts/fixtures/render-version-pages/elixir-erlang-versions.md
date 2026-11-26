# Setting Your Elixir and Erlang Versions

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).


| Current defaults | Minimum supported |
| --- | --- |
| *Elixir `1.18.4`* *Erlang/OTP `28.0.2`* Services created before *2025-06-12* have different default versions. [See below](<https://render.com/docs/elixir-erlang-versions.md#history-of-default-elixir-versions>). | *Elixir `1.12.0`* *Erlang/OTP `24.3.4`* |

Elixir version `1.18.4` and Erlang/OTP version `28.0.2` are the defaults for Render services created on or after *2025-06-12*.

You can specify different Elixir and Erlang/OTP versions by setting the following [environment variables](<https://render.com/docs/configure-environment-variables.md>) for your service:

| Variable | Example Value |
| --- | --- |
| `ELIXIR_VERSION` | `1.19.5` |
| `ERLANG_VERSION` | `28.3` |

> *Your Elixir version must be compatible with your Erlang/OTP version!*
>
> Otherwise, your service's builds will fail. For compatibility information, see the [Elixir documentation](https://hexdocs.pm/elixir/compatibility-and-deprecations.html#between-elixir-and-erlang-otp).

## History of default Elixir versions

If you don't set an Elixir version for your service, Render's default version depends on when you originally created the service:

| Service Creation Date | Default Elixir Version |
|---|---|
| 2025-06-12 and later | `1.18.4` |
| 2024-03-05 | `1.16.1` |
| 2023-11-01 | `1.15.6` |
| Before 2023-11-01 | `1.9.4` |
