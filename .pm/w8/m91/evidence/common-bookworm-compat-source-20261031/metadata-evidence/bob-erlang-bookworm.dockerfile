ARG OS_VERSION

FROM debian:${OS_VERSION} AS build

RUN apt-get update
RUN apt-get -y --no-install-recommends install \
  autoconf \
  dpkg-dev \
  gcc \
  g++ \
  make \
  libncurses-dev \
  unixodbc-dev \
  libssl-dev \
  libsctp-dev \
  wget \
  ca-certificates \
  pax-utils

ARG ERLANG

RUN mkdir -p /OTP/subdir
RUN wget -nv "https://github.com/erlang/otp/archive/OTP-${ERLANG}.tar.gz" && tar -zxf "OTP-${ERLANG}.tar.gz" -C /OTP/subdir --strip-components=1
WORKDIR /OTP/subdir
RUN ./otp_build autoconf
RUN ./configure --with-ssl --enable-dirty-schedulers
RUN make -j$(getconf _NPROCESSORS_ONLN)
RUN make -j$(getconf _NPROCESSORS_ONLN) install
RUN make -j$(getconf _NPROCESSORS_ONLN) docs DOC_TARGETS=chunks
RUN make -j$(getconf _NPROCESSORS_ONLN) install-docs DOC_TARGETS=chunks
RUN find /usr/local -regex '/usr/local/lib/erlang/\(lib/\|erts-\).*/\(man\|obj\|c_src\|emacs\|info\|examples\)' | xargs rm -rf
RUN find /usr/local -name src | xargs -r find | grep -v '\.hrl$' | xargs rm -v || true
RUN find /usr/local -name src | xargs -r find | xargs rmdir -vp || true
RUN scanelf --nobanner -E ET_EXEC -BF '%F' --recursive /usr/local | xargs -r strip --strip-all
RUN scanelf --nobanner -E ET_DYN -BF '%F' --recursive /usr/local | xargs -r strip --strip-unneeded

FROM debian:${OS_VERSION} AS final

RUN apt-get update && \
  apt-get -y --no-install-recommends install \
    ca-certificates \
    libodbc1 \
    libssl3 \
    libsctp1 \
    netbase && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /usr/local /usr/local
ENV LANG=C.UTF-8
