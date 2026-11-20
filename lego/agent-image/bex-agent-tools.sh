# Shell setup for the agent image's baked toolchains (w10/024), installed as
# /etc/profile.d/bex-agent-tools.sh. Login shells read it through /etc/profile,
# interactive shells through /etc/bash.bashrc, and every other bash through
# BASH_ENV. A non-interactive shell starts no process here and runs nothing
# that can fail, so it costs nothing and is safe under `set -eu`.

[ -n "${__bex_agent_tools-}" ] && return 0
__bex_agent_tools=1

# Debian's /etc/profile resets PATH in a login shell. Put the image's
# toolchains back in front, in the order the image's PATH has them.
case ":$PATH:" in
*:/usr/local/cargo/bin:*) ;;
*) PATH="/usr/local/cargo/bin:/opt/java/openjdk/bin:/usr/local/go/bin:$PATH" ;;
esac
export PATH

if [ -n "${BASH_VERSION-}" ] && [ -s "${NVM_DIR-}/nvm.sh" ]; then
	case $- in
	*i*)
		# As on Devin's base image: nvm loads the way its installer sets it
		# up, so `nvm alias default` applies, and direnv loads an allowed
		# .envrc on each prompt.
		. "$NVM_DIR/nvm.sh"
		. "$NVM_DIR/bash_completion"
		eval "$(direnv hook bash)"
		;;
	*)
		# Any other shell loads nvm on its first call, so a setup step or an
		# agent's command can run `nvm install` or `nvm use` directly.
		nvm() {
			unset -f nvm
			. "$NVM_DIR/nvm.sh" --no-use
			nvm "$@"
		}
		;;
	esac
fi
