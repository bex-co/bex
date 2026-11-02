defmodule BexMix.MixProject do
  use Mix.Project

  def project do
    [app: :bex_mix, version: "1.0.0", deps: [], erlc_paths: []]
  end

  def application do
    [extra_applications: [:bex_probe]]
  end
end
