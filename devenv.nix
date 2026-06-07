{ pkgs, ... }:

{
  cachix.enable = false;
  devenv.warnOnNewVersion = false;

  packages = [
    pkgs.go
    pkgs.jq
    pkgs.sqlite
  ];

  enterShell = ''
    export GOBIN="$PWD/.devenv/bin"
    export GOCACHE="$PWD/.devenv/go-cache"
    export GOMODCACHE="$PWD/.devenv/go-mod-cache"
    mkdir -p "$GOBIN" "$GOCACHE" "$GOMODCACHE"
    export PATH="$GOBIN:$PATH"
  '';
}
