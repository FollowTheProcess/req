# req

[![License](https://img.shields.io/github/license/FollowTheProcess/req)](https://github.com/FollowTheProcess/req)
[![Go Report Card](https://goreportcard.com/badge/github.com/FollowTheProcess/req)](https://goreportcard.com/report/github.com/FollowTheProcess/req)
[![GitHub](https://img.shields.io/github/v/release/FollowTheProcess/req?logo=github&sort=semver)](https://github.com/FollowTheProcess/req)
[![CI](https://github.com/FollowTheProcess/req/workflows/CI/badge.svg)](https://github.com/FollowTheProcess/req/actions?query=workflow%3ACI)
[![codecov](https://codecov.io/gh/FollowTheProcess/req/branch/main/graph/badge.svg)](https://codecov.io/gh/FollowTheProcess/req)

Execute `.http` files from the command line

> [!WARNING]
> **req is in early development and is not yet ready for use**

![caution](./img/caution.png)

## Project Description

`req` is a command line toolkit to work with `.http` files as per the <https://github.com/JetBrains/http-request-in-editor-spec> specification. See <https://www.jetbrains.com/help/idea/exploring-http-syntax.html> for an overview of the syntax but the **TL;DR** is:

```plaintext
// Comments can begin with slashes '/' or hashes '#' and last until the next newline character '\n'
# This is also a comment (I'll use '/' from now on but you are free to use both)

// Global variables (e.g. base url) can be defined with '@ident = <value>'
@base = https://api.company.com

// 3 '#' in a row mark a new HTTP request, with an optional name e.g. 'Delete employee 1'
### [name]
HTTP_METHOD <url>
Header-Name: <header value>

// You can also give them names like this
###
# @name <name>
# @name=<name>
# @name = <name>
HTTP_METHOD <url>
...

// Global variables are interpolated like this
### Get employee 1
GET {{base}}/employees/1

// Pass the body of requests like this
### Update employee 1 name
PATCH {{base}}/employees/1
Content-Type: application/json

{
  "name": "Namey McNamerson"
}
```

> [!NOTE]
> The custom javascript portions (e.g. the `{% ... %}` blocks) of the spec are **not** implemented as these are editor specific and require a javascript runtime.

## Installation

Compiled binaries for all supported platforms can be found in the [GitHub release]. There is also a [homebrew] tap:

```shell
brew install FollowTheProcess/tap/req
```

## Quickstart

Given a `.http` file containing http requests like this:

```plaintext
// demo.http

@base = https://localhost:5167
 
### Create a new item
POST {{base}}/todoitems
Content-Type: application/json
 
{
  "id": "{{ $guid }}",
  "name":"walk dog",
  "isComplete":false
}
 
### Get All items
GET {{base}}/todoitems
 
### Update item
PUT {{base}}/todoitems/1
Content-Type: application/json
 
{
  "id": 1,
  "name":"walk dog",
  "isComplete": true
}
 
### Delete item
DELETE {{base}}/todoitems/1
```

You can invoke any one of them, like this...

```shell
req do ./demo.http --request "Get All items"
```

### Credits

This package was created with [copier] and the [FollowTheProcess/go_copier] project template.

[copier]: https://copier.readthedocs.io/en/stable/
[FollowTheProcess/go_copier]: https://github.com/FollowTheProcess/go_copier
[GitHub release]: https://github.com/FollowTheProcess/req/releases
[homebrew]: https://brew.sh
