# tcpkeepalive

**Known Issues:** Some problems with the implementation were [reported](https://groups.google.com/d/msg/golang-nuts/rRu6ibLNdeI/TIzShZCmbzwJ), I'll try to fix them when I get a chance, or if somebody sends a PR.

Package tcpkeepalive implements additional TCP keepalive control beyond what is
currently offered by the net pkg.

Only Linux \>= 2.4, DragonFly, FreeBSD, NetBSD and OS X \>= 10.8 are supported
at this point, but patches for additional platforms are welcome.

See also: http://felixge.de/2014/08/26/tcp-keepalive-with-golang.html

**License:** MIT

**Docs:** http://godoc.org/github.com/felixge/tcpkeepalive
