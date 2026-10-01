# dbt Language Server

LSP for dbt

## Features

- **Code Completion**
- **Hover Information**
- **Go to Definition**
- **Find References**
- **[Go to Schema](analysis/README.md)**
- **Function Documentation**
- **[Diagnostics](#diagnostics)**
| Resource | Go to Definition | Find References | Hover | Completion | Signature Help |
| --- | --- | --- | --- | --- | --- |
| Model References | x | x | x | x |   |
| CTEs | x | x |   |   |   |
| Jinja variables (`set`, `for`, macro parameters) | x | x |   |   |   |
| Sources | x |   | x | x |   |
| Seeds | x |   | x | x |   |
| Macros | x | x | x | x | x |
| Variables | x |   | x | x |   |
| Functions |   |   | x | x | x |
### Function Documentation
This is the only part of the LSP that is dialect specific. The rest is parsed 
using the file system and a very forgiving parser that is primarily focused on 
dbt specific syntax instead of attempting to be a full SQL parser.

Supported Dialects:
- Snowflake
- BigQuery

The dialect is the adapter `type` of the profile's default target, read from the
first `profiles.yml` found in dbt's own lookup order: `$DBT_PROFILES_DIR`, the
project root, then `~/.dbt`.

Hover (`textDocument/hover`) on a function shows its documentation; signature
help (`textDocument/signatureHelp`, triggered on `(` and `,`) shows the call
form with the argument under the cursor highlighted, for dialect functions and
project macros alike.

### Diagnostics
Every `ref()` and `source()` call whose arguments are plain string literals is
checked against the project, without running dbt:
- `ref('name')` must name a model, seed or snapshot in the project or an
  installed package. Versioned models count under their YAML name.
- `ref('package', 'name')` must name one in that package. Packages that are
  not installed (e.g. dbt Mesh projects) are skipped.
- `source('source', 'table')` must name a declared source and one of its tables.

Calls with computed arguments, such as `ref('stg_' ~ name)`, are not checked.
Outside a dbt project nothing is reported.

Clients that support pull diagnostics (`textDocument/diagnostic`) request them.
Other clients get `textDocument/publishDiagnostics` tagged with the document
version: for that document when it changes, and for every open document when
one is opened or saved, since the project is read again then. Pull clients
that accept `workspace/diagnostic/refresh` are sent it at the same points.
Closing a document clears its diagnostics.

The project is also read once at startup (`initialized`). Clients that accept
server-initiated progress see this as `$/progress` begin and end. Some clients,
such as omp, wait for that before their first diagnostics request.

### dbt Fusion Static Analysis
If you have dbt fusion installed, you can use it for static analysis and the 
results from compilation will be returned as diagnostics in the editor, after
the `ref()`/`source()` checks above. Fusion compiles a file when it is opened or saved.
All artifacts from the compilation will be written to a separate directory from 
the project you are editing.

Enabled via a cli argument.
```
-f, --fusion=[path]
```
If path to the dbt fusion executable is not provided, `dbt` will be used and will look for it in `$PATH`.

## Installation

Download [latest release](https://github.com/j-clemons/dbt-language-server/releases/latest) or install via curl
```
curl -fsSL https://j-clemons.com/dbt-language-server/install | bash
```

### Neovim

Add executable to $PATH
Configure with your LSP client (e.g., nvim-lspconfig):

```lua
require'lspconfig'.dbt.setup{
  cmd = { "dbt-language-server" },
  filetypes = { "sql", "yaml" },
  root_dir = require'lspconfig'.util.root_pattern("dbt_project.yml"),
}
```

### Helix

Add executable to $PATH and add to languages.toml

```toml
[language-server.dbt-language-server]
command = "dbt-language-server"

[[language]]
name = "dbt"
scope = "dbt_project.yml"
file-types = ["sql","yml","yaml"]
language-servers = ["dbt-language-server"]
```
