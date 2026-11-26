# Setting Your Ruby Version

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).


| Current default Ruby version | Minimum supported Ruby version |
| --- | --- |
| *`3.4.13`* Services created before *2026-11-19* have a different default version. [See below](<https://render.com/docs/ruby-version.md#history-of-default-ruby-versions>). | *`3.1.0`* |

*Set a different Ruby version in _any_ of the following ways* (in descending order of precedence):

1. Include a `Gemfile.lock` or a `gems.locked` file in the root of your repo that specifies the version to use under `RUBY VERSION`:

   ```ruby
   # Gemfile.lock
   RUBY VERSION
     ruby 3.3.0
   ```

   You can add this entry to an existing `Gemfile.lock` file or update its value by running:

   ```shell
   bundle update --ruby
   ```

2. Add a file named `.ruby-version` to the root of your repo. This file contains a single line with the version to use:

   ```text
   3.3.0
   ```

3. Add a file named `.tool-versions` to the root of your repo. This file can specify versions for multiple languages. To set the Ruby version, add a line like the following:

   ```text
   ruby 3.3.0
   ```

4. Set the `ruby` directive in your `Gemfile`.

To avoid version mismatches across environments, you can set your Ruby version in the `.ruby-version` file, then read the value from that file in your `Gemfile`:

```ruby
# Gemfile
ruby file: ".ruby-version"
```

## History of default Ruby versions

If you don't set a Ruby version for your service, Render's default version depends on when you originally created the service:

| Service Creation Date | Default Ruby Version |
|---|---|
| 2026-11-19 and later | `3.4.13` |
| 2026-11-03 | `3.4.12` |
| 2025-06-12 | `3.4.4` |
| 2024-11-24 | `3.3.6` |
| 2024-09-05 | `3.3.5` |
| 2024-07-11 | `3.3.4` |
| 2024-06-13 | `3.3.3` |
| 2024-06-03 | `3.3.2` |
| 2024-04-23 | `3.3.1` |
| 2024-03-18 | `3.3.0` |
| 2023-11-01 | `3.2.2` |
| Before 2023-11-01 | `2.6.8` |
