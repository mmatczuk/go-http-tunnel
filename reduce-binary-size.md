# Initial state

```sh
$ go version

go version go1.13 linux/amd64

$ mkdir /tmp/helloworld; cd /tmp/helloworld;
$ cat > main.go <<EOF
package main

import "fmt"

func main() {
        fmt.Println("Hello world!")
}
EOF

$ export CGO_ENABLED=0
$ go build; stat -c %s helloworld

2012745
```

# Simple tricks

### 1. Strip


Add flag `-ldflags="-w -s"`:
```sh
$ go build -ldflags="-w -s"; stat -c %s helloworld

1437696
```

### 2. Disable function inlining

Add flag `-gcflags=all=-l`:
```sh
$ go build -ldflags="-w -s" -gcflags=all=-l; stat -c %s helloworld

1437696
```

It's not helpful on hello-world example, but it's helpful on big projects.
It could save ~10%

### 3. Disable bounds checks

Add flag `-gcflags=all=-B`:
```sh
$ go build -a -gcflags=all="-l -B" -ldflags="-w -s"; stat -c %s helloworld

1404928
```

### 4. Disable write-barrier

Add flag `-gcflags=all=-wb=false` to disable using of [write barriers](https://programming.vip/docs/deep-understanding-of-go-garbage-recycling-mechanism.html):
```sh
$ go build -a -gcflags=all="-l -B -wb=false" -ldflags="-w -s"; stat -c %s helloworld

1380352
```

**Danger!** If you disable write barriers then GC
[won't work correctly](https://github.com/golang/go/issues/36597),
and you may need to disable it while running the application with
environment variable `GOGC=off`. So **don't use it unless you
really know what are you doing!**

**Note!** This flag
[will be removed in Go1.15](https://github.com/golang/go/issues/36597#issuecomment-575231548).

### 5. Other `gcflags`

Somebody may also try to add `-C`.

### 6. Compress the binary

You can compress the binary using [UPX](https://github.com/upx/upx).

```sh
$ apt-get install -f upx
$ go build -a -gcflags=all="-B" -ldflags="-w -s"
$ upx helloworld
$ stat -c %s helloworld

531500

$ go build -a -gcflags=all="-l -B" -ldflags="-w -s"
$ upx helloworld
$ stat -c %s helloworld

498528

$ go build -a -gcflags=all="-l -B" -ldflags="-w -s"
$ upx --best --ultra-brute helloworld
$ stat -c %s helloworld

391292
```

But:
* The binary will be much slower.
* It will consume more RAM.
* It will be almost useless if you already store your binary in a compressed
  state (for example in `initrd`, compressed by `xz`).

Note: It looks like if you disable inlining then the code
becomes more patterny and more compressible.

Just keep in mind (if you're store the binary in compressed image):
```sh
$ go build -a -gcflags=all="-l -B" -ldflags="-w -s"
$ xz -9e helloworld
$ stat -c %s helloworld.xz

409212

$ go build -a -gcflags=all="-l -B" -ldflags="-w -s"
$ upx --best --ultra-brute helloworld
$ xz -9e helloworld
$ stat -c %s helloworld.xz

390304
```

So it's still useful a little bit...

### 7. 32bits instead of 64bits

```sh
$ go env GOARCH

amd64

$ GOARCH=386 go build -a -gcflags=all="-l -B" -ldflags="-w -s"; stat -c %s helloworld

1204224

$ upx --best --ultra-brute helloworld; stat -c %s helloworld

381184

$ ./helloworld

Hello world!
```

But it has obvious limitations:
* 32bit address space.
* 32bit integers.
* Less registers (less performance in some cases).
* 32bit syscalls (for example there's no `kexec_file_load`).

These last two points could've been avoided if Golang would support
[x32 ABI](https://en.wikipedia.org/wiki/X32_ABI). Formally speaking
Golang supports x32 ABI, but only for `GOOS=nacl` and it takes
even more space than just `linux/amd64`:

```sh
$ GOOS=nacl GOARCH=amd64p32 go build -a -ldflags="-w -s" -gcflags=all=-l; stat -c %s helloworld

1703936

$ ./helloworld

Segmentation fault (core dumped)
```

### 8. Strip function names

**Warning!** It breaks some functional of Golang (like `log` and `testing`).

A Golang binary contains a lot of metadata information about each function,
for garbage collector and to be able to print stack-traces and so on.
We could try to remove function names from there (which should not
affect garbage collection).

```sh
$ TOOL_LINK="$(readlink -f "$(go env GOROOT)"/pkg/tool/*/link)"
$ pushd "$(go env GOROOT)"/src/cmd/link
$ sed -re 's/(start := len\(ftab.P\))/\1; return int32(start)+1/' \
    -i "$(go env GOROOT)"/src/cmd/link/internal/ld/pcln.go
$ go build
$ sudo mv "$TOOL_LINK" "$TOOL_LINK".orig
$ sudo mv link "$TOOL_LINK"
$ popd
$ GOARCH=386 go build -a -gcflags=all="-l -B" -ldflags="-w -s"
$ sudo mv "$TOOL_LINK".orig "$TOOL_LINK"
$ stat -c %s helloworld

1105920

$ upx --best --ultra-brute helloworld
$ stat -c %s helloworld

354996

$ ./helloworld # yeah, it still works:

Hello world!
```

See also discussion
"[runtime.pclntab strippping](https://groups.google.com/forum/#!msg/golang-nuts/hEdGYnqokZc/zQojaoWlAgAJ)".

### 9. Use GCCGo

```sh
$ go build -a -compiler gccgo -gccgoflags=all='-flto -Os -fdata-sections -ffunction-sections -Wl,--gc-sections,-s'; stat -c %s helloworld

23184

$ upx helloworld
$ stat -c %s helloworld

10752
```

**But!**
```sh
$ go build -a -compiler gccgo -gccgoflags=all='-flto -Os -fdata-sections -ffunction-sections -Wl,--gc-sections,-s'
$ ldd ./helloworld
        linux-vdso.so.1 (0x00007ffdb1296000)
        libgo.so.13 => /usr/lib/x86_64-linux-gnu/libgo.so.13 (0x00007f26013ff000)
        libm.so.6 => /lib/x86_64-linux-gnu/libm.so.6 (0x00007f26012ba000)
        libgcc_s.so.1 => /lib/x86_64-linux-gnu/libgcc_s.so.1 (0x00007f26012a0000)
        libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f26010e0000)
        /lib64/ld-linux-x86-64.so.2 (0x00007f2602c16000)
        libz.so.1 => /lib/x86_64-linux-gnu/libz.so.1 (0x00007f2600ec2000)
        libpthread.so.0 => /lib/x86_64-linux-gnu/libpthread.so.0 (0x00007f2600ea1000)
```
So it's actually will require all this libraries. which takes a lot of space.
If you will compile it with `-static` it will take a lot of MiBs.

You can try to recompile `libgo` with `-fdata-sections -ffunction-sections` and
utilize `musl` instead of GNU libc, but this is very difficult to achieve.

---

# Dead code elimination (DCE) considerations

### Avoid visibility of `reflect.Call`

[It appears](https://github.com/u-root/u-root/issues/1477#issuecomment-562660955)
DCE cannot eliminate a lot of code if some `reflect` functions are used
(like `Call`). So if you will remove dependencies on `reflect` it may reduce the
size of your binary.

### GCCGo

Some tests shows that GCCGo's DCE may sometimes works more effective for Go code.
But still it's not effective when linking will `libgo` and `libc`. So the
total size of a static binary is higher.

### Downgrade version of Golang

See
[cockroachlabs.com: Why are my Go executable files so large?](https://www.cockroachlabs.com/blog/go-file-size/)
and
[runtime: pclntab is too big](https://github.com/golang/go/issues/36313).

### Upgrade version of Golang

See tasks related to size-optimization:

* [runtime: pclntab is too big](https://github.com/golang/go/issues/36313).
Golang community is working on optimizing the size of pclntab. So
may be some progress was already achieved when you read this tips.
* [cmd/compile: static init maps should never generate code](https://github.com/golang/go/issues/2559).
* [text/template: avoid a global map to help the linker's deadcode elimination](https://go-review.googlesource.com/c/go/+/210284/).
* and so on.

---

# Other considerations

### Project "TinyGo"

```sh
$ tinygo build -o helloworld main.go
$ stat -c %s helloworld

167888
```
Nice, **but** it's dynamic:
```sh
$ ldd ./helloworld

        linux-vdso.so.1 (0x00007ffe98b12000)
        libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007fb5d6a8e000)
        /lib64/ld-linux-x86-64.so.2 (0x00007fb5d6c55000)
```
So you need to get `libc` with the binary (which is huge).

Trying `-static` (to avoid dependency on `libc`):
```sh
$ tinygo build -ldflags '-static' -o helloworld main.go
$ stat -c %s helloworld

891168
```
Huge. Reducing it:

```sh
$ apt-get install -f musl-dev
$ tinygo build -ldflags \
  '-flto -Os -fdata-sections -ffunction-sections -Wl,--gc-sections -static -specs /usr/lib/x86_64-linux-musl/musl-gcc.specs' \
  -o helloworld main.go
$ stat -c %s helloworld

177376
```

Another garbage collector:
```sh
$ tinygo build -gc leaking -ldflags \
  '-flto -Os -fdata-sections -ffunction-sections -Wl,--gc-sections -static -specs /usr/lib/x86_64-linux-musl/musl-gcc.specs' \
  -o helloworld main.go
$ stat -c %s helloworld

162592
```


Good, but `upx` won't work here (it seems to be not compatible with musl):
```sh
$ upx helloworld | grep Exception
  upx: helloworld: EOFException: premature end of file
```

Trying to make `upx` work (see [the ticket](https://github.com/upx/upx/issues/309)):
```sh
$ sudo apt-get install libucl-dev libz-dev
$ git clone --recursive -b devel https://github.com/upx/upx
$ make -C upx all
$ ./upx/src/upx.out ./helloworld
$ stat -c %s helloworld

65256
```

OK, nice. **BUT!** TinyGo has a lot of limitations, few examples:

* It has non-full support of CGo. For example sometimes it
  processes `#define` wrong.
* It does not implement a lot of stuff from standard Golang packages.
  So the most of the project will not compile due to `undefined function`
  or something like that.
* If I remember correctly it does not support Go's plan9 assembly langauge.
* It works much slower.
* It my case it was just panicking on compiling some code.

So it's definitely worth a try for a small project. However on a big project
it could require too much time to port the project.

There're also other LLVM-based Go compilers, but they were unable
to compile the project I tried as well.

### Remove `runtime.pcntltab`

It's a continuation of point "Strip function names" (see above).
If somebody will remove/disable GC and all usages
of `runtime.Callers` and so on at all, then they may
try to remove `runtime.pcntltab`.

```sh
$ go build -a -gcflags=all=-l; go tool nm -size -sort size helloworld 2>/dev/null | head -10
  4da2e0     454552 r runtime.pclntab
  5698e0      65744 D runtime.trace
  46f4b0      20065 T unicode.init
  565a20      16064 D runtime.semtable
  563340       9952 D runtime.mheap_
  561340       8192 D runtime.timers
  55f3c0       8048 D runtime.cpuprof
  44d9b0       6785 T runtime.gentraceback
  57a9c0       5976 D runtime.memstats
  491950       5806 T fmt.(*pp).printValue

$ echo "454552 / $(stat -c %s helloworld)" | bc -l

.23134560045765053048
```
So the rought estimation is `-20%`. See also ticket
[runtime: pclntab is too big](https://github.com/golang/go/issues/36313).

---

# Going deeper

### Find the causer: `go tool nm`

An example:
```sh
$ go tool nm -size -sort size helloworld 2>/dev/null | head -10
```

### Find the causer: `go tool objdump`

An example:
```sh
$ go tool objdump helloworld | awk '{print $1}' | sort | uniq -c | sort -rn | head -10
   9095 <autogenerated>:1

   2375 tables.go:3522
   2341 TEXT
   2340
    591 tables.go:9
    567 tables.go:5512
    256 asm.s:40
    254 error.go:197
    235 print.go:664
    128 debugcall.go:52

$ go tool objdump helloworld | less -p "tables.go:3522"
$ go tool objdump helloworld 2>/dev/null | awk '{if($1=="TEXT"){path=$3; next} if($1=="tables.go:3522"){print path; exit}}'

/home/experiment0/.gimme/versions/go1.13.linux.amd64/src/unicode/tables.go

$ tail -n +3522 /home/experiment0/.gimme/versions/go1.13.linux.amd64/src/unicode/tables.go | head -20

var Scripts = map[string]*RangeTable{
        "Adlam":                  Adlam,
        "Ahom":                   Ahom,
        "Anatolian_Hieroglyphs":  Anatolian_Hieroglyphs,
        "Arabic":                 Arabic,
```

So you may consider to reduce this map for your program:

```
$ vim /home/experiment0/.gimme/versions/go1.13.linux.amd64/src/unicode/tables.go +3522
$ go build -a -gcflags=all=-l; stat -c %s helloworld

  1913600 (instead of 1964818)
```

Just an experiment to avoid `fmt` (and consensually `unicode`):

```
$ cat > main.go <<EOF
package main

import "os"

func main() {
        f, _ := os.OpenFile("/dev/stdout", os.O_WRONLY, 0)
        f.Write([]byte("Hello world!\n"))
        f.Close()
}
EOF

$ go build -a -gcflags=all=-l; stat -c %s helloworld

1329647 (instead of 1964818)

$ tinygo build -gc leaking -ldflags \
  '-flto -Os -fdata-sections -ffunction-sections -Wl,--gc-sections -static -specs /usr/lib/x86_64-linux-musl/musl-gcc.specs' \
  -o helloworld main.go
$ stat -c %s helloworld

32192
```

OK, but `upx` does not work, again:
```sh
$ ./upx/src/upx.out -f ./helloworld | grep Exception

upx.out: ./helloworld: NotCompressibleException
```

Fixing it:
```sh
$ patch -p1 <<EOF
--- a/upx/src/packer.h
+++ b/upx/src/packer.h
@@ -182,7 +182,7 @@ protected:
                              const unsigned overlap_range,
                              const upx_compress_config_t *cconf,
                              int filter_strategy = 0,
-                             bool inhibit_compression_check = false);
+                             bool inhibit_compression_check = true);
     void compressWithFilters(Filter *ft,
                              const unsigned overlap_range,
                              const upx_compress_config_t *cconf,
@@ -191,7 +191,7 @@ protected:
                              unsigned compress_ibuf_off,
                              unsigned compress_obuf_off,
                              const upx_bytep hdr_ptr, unsigned hdr_len,
-                             bool inhibit_compression_check = false);
+                             bool inhibit_compression_check = true);
     // real compression driver
     void compressWithFilters(upx_bytep i_ptr, unsigned i_len,
                              upx_bytep o_ptr,
@@ -201,7 +201,7 @@ protected:
                              const unsigned overlap_range,
                              const upx_compress_config_t *cconf,
                              int filter_strategy,
-                             bool inhibit_compression_check = false);
+                             bool inhibit_compression_check = true);

     // util for verifying overlapping decompresion
     //   non-destructive test
EOF

$ make -C upx/ all
$ ./upx/src/upx.out helloworld
$ stat -c %s helloworld

15240

$ ./helloworld

Hello world!
```

### Checking DCE

```sh
$ cat > main.go <<EOF
package main

import "fmt"

var DeadVariable = map[string]interface{}{
        "asd": map[string]interface{}{},
}

func main() {
        fmt.Println("Hello world!")
}
EOF

$ go build -a -gcflags=all=-l
$ go tool nm helloworld | grep DeadVariable

  55e1c0 D main.DeadVariable
```

While we don't use `DeadVariable` in any way. But:
```sh
$ cat > main.go <<EOF
package main

import "fmt"

type s struct {
    m map[string]interface{}
}

var DeadVariable = &s{map[string]interface{}{
    "asd": map[string]interface{}{},
}}

func main() {
    fmt.Println("Hello world!")
}
EOF
$ go build -a -gcflags=all=-l
$ go tool nm helloworld | grep -c DeadVariable

0
```
