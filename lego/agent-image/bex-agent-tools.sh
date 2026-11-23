# Shell setup for the agent image's baked toolchains (w10/024), installed as
# /etc/profile.d/bex-agent-tools.sh. Login shells read it through /etc/profile,
# interactive bash through /etc/bash.bashrc, an interactive sh (the web
# Shell's and SSH's /bin/sh, dash) through ENV, and every other bash through
# BASH_ENV. A non-interactive shell starts no process here and runs nothing
# that can fail, so it costs nothing and is safe under `set -eu`.

[ -n "${__bex_agent_tools-}" ] && return 0
__bex_agent_tools=1

# Debian's /etc/profile resets PATH in a login shell. Put the image's
# toolchains back in front, in the order the image's PATH has them: first
# the session's Python, which the driver selects after checkout (w4/m544).
case ":$PATH:" in
*:/opt/bex-python/bin:*) ;;
*) PATH="/opt/bex-python/bin:/usr/local/cargo/bin:/opt/java/openjdk/bin:/usr/local/go/bin:$PATH" ;;
esac
export PATH

# The driver's files for terminals sit beside its status file.
__bex_agent_state=${BEX_AGENT_STATUS_FILE:-/var/run/bex-agent/status.json}
__bex_agent_state=${__bex_agent_state%/*}

# A full-tier session's Java tools (w10/032): the JVM reads no HTTPS_PROXY,
# so bex-api also gives the egress proxy as JVM system properties, and they
# go in front of JAVA_TOOL_OPTIONS. A value already there (a repository's
# own) stays after them, so its own properties win. /etc/mavenrc gives Maven
# settings naming the same proxy. The driver serves a loopback relay that
# adds the session claim, which the JDK sends only through an Authenticator
# a plain HTTP client does not install (w10/038), and java/env.sh names it in
# place of the gateway. A shell started before the relay put the gateway's
# properties in front; the relay's replace them there.
if [ -n "${BEX_AGENT_JAVA_PROXY_OPTIONS-}" ]; then
	__bex_java_proxy=$BEX_AGENT_JAVA_PROXY_OPTIONS
	if [ -r "$__bex_agent_state/java/env.sh" ]; then
		# shellcheck source=/dev/null
		. "$__bex_agent_state/java/env.sh"
	fi
	if [ "$__bex_java_proxy" != "$BEX_AGENT_JAVA_PROXY_OPTIONS" ]; then
		case "${JAVA_TOOL_OPTIONS-}" in
		"$__bex_java_proxy" | "$__bex_java_proxy "*)
			JAVA_TOOL_OPTIONS=${JAVA_TOOL_OPTIONS#"$__bex_java_proxy"}
			JAVA_TOOL_OPTIONS=${JAVA_TOOL_OPTIONS# }
			;;
		esac
	fi
	unset __bex_java_proxy
	case " ${JAVA_TOOL_OPTIONS-} " in
	*" $BEX_AGENT_JAVA_PROXY_OPTIONS "*) ;;
	*)
		JAVA_TOOL_OPTIONS="$BEX_AGENT_JAVA_PROXY_OPTIONS${JAVA_TOOL_OPTIONS:+ $JAVA_TOOL_OPTIONS}"
		export JAVA_TOOL_OPTIONS
		;;
	esac
fi

# A repo session's gh (w10/027): the driver writes the GH_* settings it gives
# the agent to gh/env.sh beside its status file, so gh in a terminal (the web
# Shell, SSH, Zed) reaches the session's repository as the agent's does. A
# session without a repository, or whose workspace turned gh off, has none.
if [ -r "$__bex_agent_state/gh/env.sh" ]; then
	# shellcheck source=/dev/null
	. "$__bex_agent_state/gh/env.sh"
fi
unset __bex_agent_state

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
