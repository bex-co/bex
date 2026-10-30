# action.yml
name: cosign-installer
author: sigstore
description: 'Installs cosign and includes it in your path'
branding:
  icon: 'package'
  color: 'blue'
# This is pinned to the last major release, we have to bump it for each action version.
inputs:
  cosign-release:
    description: 'cosign release version to be installed'
    required: false
    default: 'v3.0.6'
  install-dir:
    description: 'Where to install the cosign binary'
    required: false
    default: '$HOME/.cosign'
  use-sudo:
    description: 'set to true if install-dir location requires sudo privs'
    required: false
    default: 'false'
runs:
  using: "composite"
  steps:
    # We verify the version against a SHA **in the published action itself**, not in the GCS bucket.
    - shell: bash
      env:
        input_cosign_release: ${{ inputs.cosign-release }}
        input_install_dir: ${{ inputs.install-dir }}
        input_use_sudo: ${{ inputs.use-sudo }}
        runner_arch: ${{ runner.arch }}
        runner_os: ${{ runner.os }}
      run: |
        #!/bin/bash
        # Substitute environment variables in install-dir input
        install_dir=$(envsubst <<<"${input_install_dir}")

        # cosign install script
        shopt -s expand_aliases
        if [ -z "$NO_COLOR" ]; then
          alias log_info="echo -e \"\033[1;32mINFO\033[0m:\""
          alias log_error="echo -e \"\033[1;31mERROR\033[0m:\""
        else
          alias log_info="echo \"INFO:\""
          alias log_error="echo \"ERROR:\""
        fi
        set -e

        CURL_RETRIES=3

        # This function helps compare versions.
        # Returns 0 if version1 >= version2, 1 otherwise.
        # Usage: is_version_ge "3.0.0" "$version_num"
        is_version_ge() {
            [ "$(printf '%s\n' "$1" "$2" | sort -V | head -n1)" == "$1" ]
        }

        # Check for unsupported old versions (anything below v2.0.0)
        if [[ "${input_cosign_release}" != "main" ]]; then
          # Extract version without 'v' prefix for comparison
          version_num="${input_cosign_release}"
          version_num="${version_num#v}"

          # Check if version is less than v2.0.0
          if ! is_version_ge "2.0.0" "$version_num"; then
            log_error "cosign versions below v2.0.0 are no longer supported."
            log_error "Requested version: ${input_cosign_release}"
            log_error "Please use cosign v2.6.0 or later."
            log_error "See https://github.com/sigstore/cosign/releases for available versions."
            exit 1
          fi
        fi

        mkdir -p "${install_dir}"

        if [[ "${input_cosign_release}" == "main" ]]; then
          log_info "installing cosign via 'go install' from its main version"
          GOBIN=$(go env GOPATH)/bin
          go install github.com/sigstore/cosign/v3/cmd/cosign@main
          ln -s "$GOBIN/cosign" "${install_dir}/cosign"
          exit 0
        fi

        shaprog() {
          case ${runner_os} in
            Linux|linux)
              sha256sum "$1" | cut -d' ' -f1
              ;;
            macOS|macos)
              shasum -a256 "$1" | cut -d' ' -f1
              ;;
            Windows|windows)
              powershell -command "(Get-FileHash $1 -Algorithm SHA256 | Select-Object -ExpandProperty Hash).ToLower()"
              ;;
            *)
              log_error "unsupported OS ${runner_os}"
              exit 1
              ;;
          esac
        }

        ## curl -sL https://github.com/sigstore/cosign/releases/download/v3.0.6/cosign_checksums.txt |\
        ##   gawk 'match($2,/^cosign-([[:alnum:]]+)-([[:alnum:]]+)(\.[[:alnum:]]+)?$/,a){printf "bootstrap_%s_%s_sha=\"%s\"\n",a[1],a[2],$1}' |\
        ##   LANG=C sort
        bootstrap_version='v3.0.6'
        bootstrap_darwin_amd64_sha="4c3e7af8372d3ca3296e62fa56f23fcbb5721cc6ac1827900d398f110d7cd280"
        bootstrap_darwin_arm64_sha="5fadd012ae6381a6a29ff86a7d39aa873878852f1073fc90b15995961ecfb084"
        bootstrap_linux_amd64_sha="c956e5dfcac53d52bcf058360d579472f0c1d2d9b69f55209e256fe7783f4c74"
        bootstrap_linux_arm64_sha="bedac92e8c3729864e13d4a17048007cfafa79d5deca993a43a90ffe018ef2b8"
        bootstrap_linux_arm_sha="67bd25d32daff5664caf51208c95defcb2ad7ac1296f394fa677bb8bacee62f5"
        bootstrap_linux_ppc64le_sha="08c3e5e0a09c440f49e9a69d8639d37fbec522ec8c5c0ac805243b098e6ea512"
        bootstrap_linux_riscv64_sha="e25952e798958b0f9168d044153ccc353f5469ca4b71a1707dffad0534d27017"
        bootstrap_linux_s390x_sha="3cf4b769258ed9cc3c2a93268c0d5c1cc3fbd094af8df21035cbac8fb0d7c088"
        bootstrap_windows_amd64_sha="9b85a88ebff2d9dd30ff4984a6f61f2cedc232dd87d81fa7f2ff3c0ed96c241c"

        cosign_executable_name=cosign

        trap "popd >/dev/null" EXIT

        pushd "${install_dir}" > /dev/null

        case ${runner_os} in
          Linux|linux)
            case ${runner_arch} in
              X64|amd64)
                bootstrap_filename='cosign-linux-amd64'
                bootstrap_sha=${bootstrap_linux_amd64_sha}
                desired_cosign_filename='cosign-linux-amd64'
                ;;

              ARM|arm)
                bootstrap_filename='cosign-linux-arm'
                bootstrap_sha=${bootstrap_linux_arm_sha}
                desired_cosign_filename='cosign-linux-arm'
                ;;

              ARM64|arm64)
                bootstrap_filename='cosign-linux-arm64'
                bootstrap_sha=${bootstrap_linux_arm64_sha}
                desired_cosign_filename='cosign-linux-arm64'
                ;;

              *)
                log_error "unsupported architecture ${runner_arch}"
                exit 1
                ;;
            esac
            ;;

          macOS|macos)
            case ${runner_arch} in
              X64|amd64)
                bootstrap_filename='cosign-darwin-amd64'
                bootstrap_sha=${bootstrap_darwin_amd64_sha}
                desired_cosign_filename='cosign-darwin-amd64'
                ;;

              ARM64|arm64)
                bootstrap_filename='cosign-darwin-arm64'
                bootstrap_sha=${bootstrap_darwin_arm64_sha}
                desired_cosign_filename='cosign-darwin-arm64'
                ;;

              *)
                log_error "unsupported architecture ${runner_arch}"
                exit 1
                ;;
            esac
            ;;

          Windows|windows)
            case ${runner_arch} in
              X64|amd64)
                bootstrap_filename='cosign-windows-amd64.exe'
                bootstrap_sha=${bootstrap_windows_amd64_sha}
                desired_cosign_filename='cosign-windows-amd64.exe'
                cosign_executable_name=cosign.exe
                ;;
              *)
                log_error "unsupported architecture ${runner_arch}"
                exit 1
                ;;
            esac
            ;;
          *)
            log_error "unsupported os ${runner_os}"
            exit 1
            ;;
        esac

        SUDO=
        if [[ "${input_use_sudo}" == "true" ]] && command -v sudo >/dev/null; then
          SUDO=sudo
        fi

        expected_bootstrap_version_digest=${bootstrap_sha}
        log_info "Downloading bootstrap version '${bootstrap_version}' of cosign to verify version to be installed...\n      https://github.com/sigstore/cosign/releases/download/${bootstrap_version}/${bootstrap_filename}"
        $SUDO curl --retry "${CURL_RETRIES}" -fsSL "https://github.com/sigstore/cosign/releases/download/${bootstrap_version}/${bootstrap_filename}" -o "${cosign_executable_name}"
        shaBootstrap=$(shaprog "${cosign_executable_name}")
        if [[ "$shaBootstrap" != "${expected_bootstrap_version_digest}" ]]; then
          log_error "Unable to validate cosign version: '${input_cosign_release}'"
          exit 1
        fi
        $SUDO chmod +x "${cosign_executable_name}"

        # If the bootstrap and specified `cosign` releases are the same, we're done.
        if [[ "${input_cosign_release}" == "${bootstrap_version}" ]]; then
          log_info "bootstrap version successfully verified and matches requested version so nothing else to do"
          exit 0
        fi

        semver='^v([0-9]+\.){0,2}(\*|[0-9]+)(-?r?c?)(\.[0-9]+)$'
        if [[ "${input_cosign_release}" =~ $semver ]]; then
          log_info "Custom cosign version '${input_cosign_release}' requested"
        else
          log_error "Unable to validate requested cosign version: '${input_cosign_release}'"
          exit 1
        fi

        # Download custom cosign
        log_info "Downloading platform-specific version '${input_cosign_release}' of cosign...\n      https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${desired_cosign_filename}"
        $SUDO curl --retry "${CURL_RETRIES}" -fsSL "https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${desired_cosign_filename}" -o "cosign_${input_cosign_release}"
        shaCustom=$(shaprog "cosign_${input_cosign_release}");

        # same hash means it is the same release
        if [[ "$shaCustom" != "$shaBootstrap" ]]; then
          log_info "Downloading cosign public key '${input_cosign_release}' of cosign...\n    https://raw.githubusercontent.com/sigstore/cosign/${input_cosign_release}/release/release-cosign.pub"
          RELEASE_COSIGN_PUB_KEY=https://raw.githubusercontent.com/sigstore/cosign/${input_cosign_release}/release/release-cosign.pub
          RELEASE_COSIGN_PUB_KEY_SHA='f4cea466e5e887a45da5031757fa1d32655d83420639dc1758749b744179f126'

          log_info "Verifying public key matches expected value"
          $SUDO curl --retry "${CURL_RETRIES}" -fsSL "$RELEASE_COSIGN_PUB_KEY" -o public.key
          sha_fetched_key=$(shaprog public.key)
          if [[ "$sha_fetched_key" != "$RELEASE_COSIGN_PUB_KEY_SHA" ]]; then
            log_error "Fetched public key does not match expected digest, exiting"
            exit 1
          fi

          if is_version_ge "3.0.1" "$version_num"; then
            # we're trying to get something greater than or equal to v3.0.1
            keyless_signature_file=${desired_cosign_filename}.sigstore.json
            log_info "Downloading keyless verification bundle for platform-specific '${input_cosign_release}' of cosign...\n      https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${keyless_signature_file}"
            $SUDO curl --retry "${CURL_RETRIES}" -fsSLO "https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${keyless_signature_file}"

            log_info "Using bootstrap cosign to verify keyless signature of desired cosign version"
            "./${cosign_executable_name}" verify-blob --certificate-identity=keyless@projectsigstore.iam.gserviceaccount.com --certificate-oidc-issuer=https://accounts.google.com --bundle "${keyless_signature_file}" "cosign_${input_cosign_release}"

            if is_version_ge "3.0.3" "$version_num"; then
              # we're trying to get something greater than or equal to v3.0.3
              kms_signature_file=${desired_cosign_filename}-kms.sigstore.json
              log_info "Downloading KMS verification bundle for platform-specific '${input_cosign_release}' of cosign...\n      https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${kms_signature_file}"
              $SUDO curl --retry "${CURL_RETRIES}" -fsSLO "https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${kms_signature_file}"

              log_info "Using bootstrap cosign to verify signature of desired cosign version"
              "./${cosign_executable_name}" verify-blob --key public.key --bundle "${kms_signature_file}" "cosign_${input_cosign_release}"
            fi
          else
            signature_file=${desired_cosign_filename}.sig
            log_info "Downloading detached signature for platform-specific '${input_cosign_release}' of cosign...\n      https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${signature_file}"
            $SUDO curl --retry "${CURL_RETRIES}" -fsSLO "https://github.com/sigstore/cosign/releases/download/${input_cosign_release}/${signature_file}"

            log_info "Using bootstrap cosign to verify signature of desired cosign version"
            "./${cosign_executable_name}" verify-blob --key public.key --signature "${signature_file}" "cosign_${input_cosign_release}"
          fi

          $SUDO rm "${cosign_executable_name}"
          $SUDO mv "cosign_${input_cosign_release}" "${cosign_executable_name}"
          $SUDO chmod +x "${cosign_executable_name}"
          log_info "Installation complete!"
        fi

    - if: ${{ runner.os == 'Linux' || runner.os == 'macOS' }}
      shell: bash
      env:
        input_install_dir: ${{ inputs.install-dir }}
      run: envsubst <<<"${input_install_dir}" >> "$GITHUB_PATH"

    - if: ${{ runner.os == 'Windows' }}
      shell: pwsh
      env:
        input_install_dir: ${{ inputs.install-dir }}
      run: |
        $install_dir = $ExecutionContext.InvokeCommand.ExpandString("${env:input_install_dir}")
        echo "${install_dir}" | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append
