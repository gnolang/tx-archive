# tx-archive — archived

> ⚠️ **This repository is archived.** ⚠️
>
> The source has moved into the main [`gnolang/gno`](https://github.com/gnolang/gno) monorepo.
> All new development happens there.

## Where it lives now

[`contribs/tx-archive/`](https://github.com/gnolang/gno/tree/master/contribs/tx-archive) in `gnolang/gno`.

## How to use it

```bash
git clone https://github.com/gnolang/gno.git
cd gno/contribs/tx-archive
go run ./cmd backup --help
go run ./cmd restore --help
```

Or, from any Go module that already depends on `github.com/gnolang/gno`:

```go
import (
    "github.com/gnolang/gno/contribs/tx-archive/backup"
    "github.com/gnolang/gno/contribs/tx-archive/backup/client/rpc"
    "github.com/gnolang/gno/contribs/tx-archive/backup/writer/standard"
)
```

## Issues & PRs

Please open them against the monorepo: <https://github.com/gnolang/gno/issues>.
