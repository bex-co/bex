# A dependency-free HTTP responder for the native fixtures: every Debian-based
# toolchain image (Node, Python, Ruby, Go, Rust) ships Perl and IO::Socket::INET,
# so one server covers them all without a build step. It answers `200 ok` on
# the port given as its argument, else on PORT.
use strict;
use IO::Socket::INET;
my $port = $ARGV[0] || $ENV{PORT} || 10000;
my $server = IO::Socket::INET->new(LocalAddr => '0.0.0.0', LocalPort => $port, Listen => 16, ReuseAddr => 1)
  or die "listen on $port: $!\n";
$| = 1;
print "default-env-probe listening on $port\n";
while (my $client = $server->accept) {
  while (my $line = <$client>) { last if $line =~ /^\r?\n$/ }
  print $client "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 3\r\nConnection: close\r\n\r\nok\n";
  close $client;
}
