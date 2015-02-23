# Process Protocol Buffer definitions with text templates and JavaScript functions

This utility can be used to process a `.proto` source file with a [Go template](https://github.com/alecthomas/template) and associated JavaScript file.

1. The protocol buffer is compiled to a [FileDescriptorSet](https://code.google.com/p/protobuf/source/browse/trunk/src/google/protobuf/descriptor.proto).
2. CamelCase functions defined in the JavaScript file are exposed to the template (same export rules as Go).
3. The `FileDescriptorSet` is passed to the Go template as the top-level context.

## Installation

First, install protoc. This varies by platform, but with Homebrew it's something like:

```
$ brew install protobuf
```

Then install prototemplate:

```
$ go get -ldflags "-X main.TemplateDir ${GOPATH}/src/github.com/alecthomas/prototemplate/templates" github.com/alecthomas/prototemplate
```

The above incantation allows prototemplate to find its builtin templates.

## Example

The following example will list all messages and fields in all files while stripping any leading package names present.

Protocol buffer definition (test.proto):

```proto
package test;

// Defines what role a user has.
enum Role {
  EXECUTIVE = 1;
  MANAGER = 2;
  EMPLOYEE = 3;
}

message User {
  required string name = 1;
  optional int64 age = 2 [default=18];
  repeated Role roles = 3;
}

message Group {
  required User owner = 1;
  repeated User users = 2;
}
```

Javascript helper (test.js):

```js
function StripPackage(ref) {
  return ref.replace(/^.*\./, '');
}
```

And this template (test.got):

```
{{with .FileDescriptorSet}}\
{{range .File}}\
{{range .MessageType}}\
{{.Name|StripPackage}}
{{range .Field}}\
  {{.Name}} = {{.Number}}
{{end}}\
{{end}}\
{{end}}\
{{end}}\
```

*NOTE: prototemplate uses a fork of `text/template` that elides newlines when a closing delimiter is immediately followed by a `\` (as seen above).*

And invoke the utility like so:

```
$ prototemplate test.proto test.got test.js
User
  name = 1
  age = 2
  roles = 3
Group
  owner = 1
  users = 2
```

## Usage

```
$ prototemplate --help
usage: prototemplate [<flags>] <proto> <template> [<script>]

Flags:
  --help             Show help.
  --templates=/Users/alec/.go/src/github.com/alecthomas/prototemplate/templates
                     Root path to templates.
  --list             List builtin generators.
  --builtins         List builtin functions.
  -I, --include=DIR  List of include paths to pass to protoc.
  -o, --output=FILE  File to output generated template source to.

Args:
  <proto>     Protocol buffer definition to compile.
  <template>  Template file, or name of a builtin generator.
  [<script>]  A JavaScript file defining template helper functions.
```

## Included templates

### Swift

This generator currently includes a generator for [Swift](https://developer.apple.com/swift/), utilising the serialisation layer from the excellent [protobuf-swift](https://github.com/alexeyxo/protobuf-swift). This template has two goals:

1. To work around [a bug](https://github.com/alexeyxo/protobuf-swift/issues/38) in the existing generated code.
2. To generate more idiomatic code utilising Swift's optional types.


Usage:

```
$ prototemplate test.proto swift
```
