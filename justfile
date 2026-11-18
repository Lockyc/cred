# `default` pipes `just --list` through a small stock-perl filter that clips long recipe
# docs to your terminal width (…) instead of wrapping. Self-contained — no external files;
# falls back to plain `just --list` where perl is absent. Edit the recipes below, not this.
# List available recipes
default:
    @if command -v perl >/dev/null 2>&1; then just --color always --list | perl -CS -Mutf8 -lpe 'BEGIN{($w)=`stty size 2>/dev/null </dev/tty`=~/ (\d+)/; $w||=100; $col=(-t STDOUT && !exists $ENV{NO_COLOR})} s/\e\[[0-9;]*m//g unless $col; (my $v=$_)=~s/\e\[[0-9;]*m//g; if(length($v)>$w){my($o,$n)=("",0); while(length && $n<$w-1){ if($col && s/^(\e\[[0-9;]*m)//){$o.=$1}else{s/^(.)//;$o.=$1;$n++} } $_=$o."…".($col?"\e[0m":"")}'; else just --list; fi

# Build the cred binary
build:
    go build -o cred .

# Run the test suite
test:
    go test ./...

# Read cred's .env output back through python-dotenv (needs uvx; not in the gate)
readers:
    CRED_READERS=1 go test -count=1 -run PythonDotenv ./internal/store

# Fuzz each parser for TIME apiece (seeds already run in the gate; not in the gate)
fuzz TIME="60s":
    go test -run '^$' -fuzz '^FuzzQuoteEnv$' -fuzztime {{TIME}} ./internal/store
    go test -run '^$' -fuzz '^FuzzSetEnvKey$' -fuzztime {{TIME}} ./internal/store
    go test -run '^$' -fuzz '^FuzzFeed$' -fuzztime {{TIME}} ./internal/secret

# Mutation-test PKG, only lines changed since BASE when given; triage survivors by hand (not in the gate)
mutants PKG="./..." BASE="":
    go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash --invert-logical -S lt {{ if BASE != "" { "-D " + BASE } else { "" } }} {{PKG}}

# Format all Go files in place
fmt:
    gofmt -w .

# Install to the Go bin dir (on PATH), then print the installed version
install:
    #!/usr/bin/env bash
    set -euo pipefail
    go install .
    bin="$(go env GOBIN)"; [ -n "$bin" ] || bin="$(go env GOPATH)/bin"
    "$bin/cred" version

# non-mutating pre-release gate: gofmt check + vet + tests
gate:
    #!/usr/bin/env bash
    set -euo pipefail
    test -z "$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
    go vet ./...
    go test ./...
