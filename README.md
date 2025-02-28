This repository contains an experimental replacement for [Sandstorm][1].
See zenhack's [blog post][2] ([mirror][3]).

Currently, most of the sandbox setup code is built, and tempest
is capable of spawning sandstorm apps and plumbing http traffic to them
from the outside, though some http features are not yet implemented.

# Building

## Dependencies

### Basics

Building Tempest requires that you have the following available on your Linux system:

- [curl](https://curl.se)
- [Git](https://git-scm.com)
- `gunzip`
- `make`
- `mkdir`
- a POSIX-compatible shell
- `printf`
- `sha256sum` or `shasum`
- `rm`
- `sleep`
- `tar`

### Via Build Tool

The build tool will eventually take care of the entire process of installing the remaining dependencies and building Tempest. However it is in progress (see https://github.com/sandstorm-org/tempest/issues/2), so it will only take care of the first part of the build process.

Run `make toolchain` to install the following dependencies:

- [Go](https://go.dev/)
- [TinyGo](https://tinygo.org/)

#### Additional Steps

For now (until the build tool is complete), you will need to take a couple manual steps.

First, set the PATH to the newly installed Go and TinyGo (replacng x.y.z with the correct version in each):

```
PATH=/path/to/tempest/toolchain/go-x.y.z/bin:/path/to/tempest/toolchain/tinygo-x.y.z/bin:$PATH
go version # confirm that go is available
tiny version # confirm that tinygo is available
```

You can add the `PATH=` line to `.bashrc` or `.profile` if you want to continue using it in new sessions.

You will also need to copy this file:

```
cd toolchain/tinygo-0.35.0/
mkdir -p lib/tinygo/targets
cp -i targets/wasm_exec.js lib/tinygo/targets/
cd -
```

### Manual Install

*(This section will be removed when the build-tool handles this task.)*

You will need to manually install these packages.

See the specific instructions following this list.

- [Bison](https://www.gnu.org/software/bison/)
- [flex](https://github.com/westes/flex)
- `bpf_asm` from [the Linux kernel source tree](https://github.com/torvalds/linux/tree/master/tools/bpf)
- [capnpc-go](https://github.com/capnproto/go-capnp)
- [Cap'n Proto](https://capnproto.org/)
- [Clang](https://clang.llvm.org/)

#### Bison, flex, capnproto, clang

On Debian Bookworm, you can get these directly from apt:

```
sudo apt install bison flex capnproto clang
```

On other systems you may need to install these manually.

#### `bpf_asm`

`bpf_asm` is not packaged in all distributions; if your distro does
not have a package for it, you can install it from source. You will
additionally need `bison` and `flex` installed, and then you can
run:

```
curl https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.3.1.tar.xz > linux.tar.xz
tar -xvf linux.tar.xz
cd linux-*/tools/bpf
make bpf_asm
sudo install -Dm755 -t /usr/local/bin/ bpf_asm
cd -
```

#### capnpc-go

In addition to installing capnpc-go, we'll also need to have the source for go-capnp checked out. We'll leave it in a directory outside of tempest:

```
mkdir ../deps
git clone https://github.com/capnproto/go-capnp ../deps/go-capnp
cd ../deps/go-capnp/capnpc-go
go build
sudo install -Dm755 -t /usr/local/bin/ capnpc-go
cd -
```

## Tempest

*(This section will be removed when the build-tool handles this task.)*

Run the configure script and then `make build`. The configure script
accepts most of the same options as typical gnu packages. Additionally
you will need to supply the paths to the repository checked out above
via the `--with-go-capnp` flag.

It is possible to share grain & app storage with an existing
Sandstorm installation. If you want to do this, you will need to specify
the correct value for `--localstatedir`, and then see the next section
on importing data from Sandstorm:

```
./configure \
    --with-go-capnp=../deps/go-capnp \
    --localstatedir=/opt/sandstorm/var
make build
```

Then run `make install` to install tempest system wide.

If you do not want to share storage with Sandstorm, you can omit the
`--localstatedir` flag.

In addition to the files used by sandstorm, `tempest` will create a
couple extra things underneath that path, namely:

- a sqlite3 database at `/opt/sandstorm/var/sandstorm/sandstorm.sqlite3`
  (that's it for now, we will try to update as things progress)

# Importing data from Sandstorm

Tempest comes with a tool to import some data from a sandstorm
installation's database; after running `make`, there will be
an executable at `_build/sandstorm-import-tool`. (If you have a
brand new Sandstorm installation with no apps installed, this may
fail)

On a typical sandstorm server you can export the contents of the
database via:

```
sudo -u sandstorm -g sandstorm ./_build/sandstorm-import-tool --snapshot-dir /tmp/sandstormexport export
```

If your sandstorm installation is in a non-standard path or mongoDB is
listening on a different port, you may have to supply additional
options; see `sandstorm-import-tool --help` to see the full list.

You can then import the snapshot into tempest via:

```
sudo -u sandstorm -g sandstorm ./_build/sandstorm-import-tool --snapshot-dir /tmp/sandstormexport import
```

For some development, it can be useful to export & import from sandstorm
frequently. Therefore, we have a Makefile target for this:

```
sudo make export-import
```

...which will automate the above, using the default values to
sandstorm-import-tool's flags. It will also destroy the old database
and fix permissions on the new one.

# Running

`tempest` should be run as the user and group chosen by the via
the `--user` and `--group` flags to `./configure` (by default both
`sandstorm`).  The easiest way to do this is to run as root:

```
sudo -u sandstorm -g sandstorm ./_build/tempest
```

Tempest can be configured via environment variables; see
`./capnp/settings.capnp` for full documentation and `./env.sh.example`
for an example.

Note that for environment variables to be picked up by tempest when run
with sudo, you will have to pass the `--preserve-env`/`-E` flag:

```
sudo --preserve-env -u sandstorm -g sandstorm ./_build/tempest
```

For development purposes, the Makefile includes a `dev` target that will
rebuild, reinstall, and then spawn tempest; simply run:

```
sudo --preserve-env make dev
```

# Creating users

Out of the box, it is possible to login in via both email (if the
`SMTP_*` enviornment variables are set) and "developer accounts," which
are useful for testing. However, by default none of these accounts will
have any rights on the server. To create a user with the authority to do
interesting things, you can either:

- Import data from Sandstorm, per above. Users will have the same
  permissions they had in Sandstorm.
- Use the `tempest-make-user` command.

For the latter, run:

```
# for email users:
./_build/tempest-make-user --type email --id alice@example.com --role user
# for dev accounts:
./_build/tempest-make-user --type dev --id 'Alice Dev Admin' --role admin
```

Where `role` can be any of `visitor`, `user`, or `admin`, with the same
meanings as in Sandstorm:

- `visitor`s have the ability to list and interact with grains that have
  been shared with them, but otherwise have no authority on the server.
- `user`s can additionally install apps and create grains.
- `admin`s have full access to the server.

# Using

Visit the web interface (as defined by `BASE_URL`), and log in either
with a developer account or email.

Once you have logged in, the Grains link will display grains the user
has access to. Click the links to open the grains.

This will display the grain's UI within an iframe. Things like
offer iframes and anything that uses sandstorm specific APIs will not
work currently.

If your account has at least the `user` role, the Apps link will
allow you upload spk files to install apps, or create grains from
apps which are already installed.

[1]: https://sandstorm.io
[2]: https://zenhack.net/2023/01/06/introducing-tempest.html
[3]: https://web.archive.org/web/20230602123052/https://zenhack.net/2023/01/06/introducing-tempest.html
