# req

[![License](https://img.shields.io/github/license/FollowTheProcess/req)](https://github.com/FollowTheProcess/req)
[![Go Report Card](https://goreportcard.com/badge/github.com/FollowTheProcess/req)](https://goreportcard.com/report/github.com/FollowTheProcess/req)
[![GitHub](https://img.shields.io/github/v/release/FollowTheProcess/req?logo=github&sort=semver)](https://github.com/FollowTheProcess/req)
[![CI](https://github.com/FollowTheProcess/req/workflows/CI/badge.svg)](https://github.com/FollowTheProcess/req/actions?query=workflow%3ACI)
[![codecov](https://codecov.io/gh/FollowTheProcess/req/branch/main/graph/badge.svg)](https://codecov.io/gh/FollowTheProcess/req)

Execute `.http` files from the command line

> [!WARNING]
> **req is in early development and is not yet ready for use**

![caution](./docs/img/caution.png)

## Project Description

`req` is a command line toolkit to execute `.http` files

```plaintext
// Comments can begin with slashes '/' or hashes '#' and last until the next newline character '\n'
# This is also a comment (I'll use '/' from now on but you are free to use both)

// Global variables (e.g. base url) can be defined with '@ident = <value>'
@base = https://api.company.com

// 3 '#' in a row mark a new HTTP request, with an optional comment e.g. "Deletes employee 1"
// This comment is effectively the description of the request
### [comment]
HTTP_METHOD <url>
Header-Name: <header value>

// You can also give them names like this, although names like this
// do not allow spaces e.g. 'Delete employee 1' must be 'DeleteEmployee1'
###
# @name <name>
# @name=<name>
# @name = <name>
HTTP_METHOD <url>
...

// Global variables are interpolated like this
### Get employee 1
GET {{ .base }}/employees/1

// Pass the body of requests like this
### Update employee 1 name
PATCH {{ .base }}/employees/1
Content-Type: application/json

{
  "name": "Namey McNamerson"
}
```

## Installation

Compiled binaries for all supported platforms can be found in the [GitHub release]. There is also a [homebrew] tap:

```shell
brew install --cask FollowTheProcess/tap/req
```

## Quickstart

Given a `.http` file containing 1 or more http requests like this:

```plaintext
// demo.http

@base = https://jsonplaceholder.typicode.com

### Simple demo request
# @name Demo
GET {{ .base }}/todos/1
Accept: application/json
```

You can invoke any one of them, like this...

```shell
# req do [file] [request name]
req do ./demo.http Demo
```

## Compatibility

While there is a strict specification for the format of pure HTTP requests ([RFC9110]). There is little/no formal specification for the evolution of the format used in this project, the
closest things available are:

- [JetBrains HTTP Request in Editor Spec]
- [JetBrains Syntax Guide]
- [VSCode REST Extension]

And careful inspection of them reveals a number of discrepancies and inconsistencies between them. As a result, knowing which features/syntax to support for this project
was... tricky. So this project is a best effort to support the syntax and features that I thought was most reasonable and achievable in a standalone command line tool
not built into an IDE.

Some of the more prominent differences are:

### Whitespace

The [JetBrains HTTP Request in Editor Spec] specifies exact whitespace requirements between different sections e.g. a single `\n` character *must* follow a request line.

See <https://github.com/JetBrains/http-request-in-editor-spec/blob/master/spec.md#23-whitespaces>

This project makes no such requirement, whitespace is entirely ignored meaning the formatting of `.http` files is up to convention and/or automatic formatting tools

### Response Handlers

The [JetBrains HTTP Request in Editor Spec] allows for custom JavaScript [Response Handlers](https://github.com/JetBrains/http-request-in-editor-spec/blob/master/spec.md#324-response-handler) (e.g. the `{% ... %}` blocks), that take the response and transform it in some way:

```plaintext
GET http://example.com/auth

> {% client.global.set("auth", response.body.token);%}
```

This is not supported in `req` as it relies on editor-specific context and requires a JavaScript runtime.

However, the version of this syntax where you dump the response body to a file *is* supported!

```plaintext
GET http://example.com/auth

> ./response.json
```

### Response Reference

The [JetBrains HTTP Request in Editor Spec] allows for a [Response Reference](https://github.com/JetBrains/http-request-in-editor-spec/blob/master/spec.md#325-response-reference), but doesn't actually
explain what that is or what should be done with it? So I've left it out for now 🤷🏻

```plaintext
GET http://example.com

<> previous-response.200.json
```

> [!NOTE]
> I can foresee a potential use for this syntax: Saving the first response to the filepath indicated and then the next time it runs, comparing the responses and generating a diff of the previous response vs the current one. This isn't
> implemented yet but it's in the back of my mind for the future 👀

### Templating Syntax

All the mentioned specs allow for some sort of templating inside the `.http` files e.g. declaring a base URL globally, then interpolating it in all request URLs

The implementations designed to run in IDEs likely leverage the native templating capability of the host language (Kotlin in JetBrains' case and JavaScript for VSCode). This presents me an issue, as `req` is written in Go, the native
templating syntax is a little different.

Instead of `{{ base }}`, Go's templating requires a `.` for attribute access and templated variables to be exported so the direct equivalent would be `{{ .base }}`. Likewise function calls like `{{ $random.uuid }}` would be something like `{{ .Random.UUID }}`

I've chosen to use Go's native templating syntax *for now* to get this project off the ground and ensure it's got all the features I want it to have. However, swapping out the limited templating available to `.http` files to a custom
implementation so that `req` may execute `.http` files compatible with both JetBrains IDEs and the VSCode REST Extension is *definitely* on my roadmap!

> [!WARNING]
> Until this is done, HTTP request files written for `req` will sadly not be compatible with other IDE based tools

### Credits

This package was created with [copier] and the [FollowTheProcess/go-template] project template.

[copier]: https://copier.readthedocs.io/en/stable/
[FollowTheProcess/go-template]: https://github.com/FollowTheProcess/go-template
[GitHub release]: https://github.com/FollowTheProcess/req/releases
[homebrew]: https://brew.sh
[JetBrains Syntax Guide]: https://www.jetbrains.com/help/idea/exploring-http-syntax.html
[RFC9110]: https://www.rfc-editor.org/rfc/rfc9110.html
[JetBrains HTTP Request in Editor Spec]: https://github.com/JetBrains/http-request-in-editor-spec
[VSCode REST Extension]: https://github.com/Huachao/vscode-restclient
