## Overview

**cli-utils** repository contains mutliple packages consisting of the common code/business logic between `kuberactl` and `litmusctl`.

## Installation

```go
# Go Modules
require github.com/mayadata-io/cli-utils v0.0.0-20210119141112-84fe44fff4e7
```
`v0.0.0-20210119141112-84fe44fff4e7` being the version(commit hash in this case).

**Getting the latest commit**
To get the latest commit of this repo, run the following command
```go
$ go get github.com/mayadata-io/cli-utils@<commit-hash>
```

## Usage

The following sample depicts the way to import a package from this repo
```go
import "github.com/mayadata-io/cli-utils/chaos"
```
