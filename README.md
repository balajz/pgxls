# pgxls: PostgreSQL Language Server

An implementation of the Language Server Protocol for PostgreSQL.

## Note

This project is a fork of [sqls](https://github.com/sqls-server/sqls), refactored to be exclusively PostgreSQL-specific.

## Features

pgxls aims to provide advanced intelligence for you to edit PostgreSQL SQL in your own editor.

### Support RDBMS

- PostgreSQL([pgx](https://github.com/jackc/pgx))

### Language Server Features

#### Auto Completion

- DML(Data Manipulation Language)
    - [x] SELECT
        - [x] Sub Query
    - [x] INSERT
    - [x] UPDATE
    - [x] DELETE
- DDL(Data Definition Language)
    - [ ] CREATE TABLE
    - [ ] ALTER TABLE

#### Join completion
If the tables are connected with a foreign key pgxls can complete ```JOIN``` statements

#### CodeAction

- [x] Execute SQL
- [ ] Explain SQL
- [x] Switch Connection(Selected Database Connection)
- [x] Switch Database

#### Hover

#### Signature Help

#### Document Formatting

## Installation

```shell
go install github.com/sqls-server/sqls@latest
```

## Editor Plugins

- [sqls.vim](https://github.com/sqls-server/sqls.vim)
- [vscode-sqls](https://github.com/lighttiger2505/vscode-sqls)
- [sqls.nvim](https://github.com/nanotee/sqls.nvim)
- [Emacs LSP mode](https://emacs-lsp.github.io/lsp-mode/page/lsp-sqls/)

## DB Configuration

The connection to the PostgreSQL is essential to take advantage of the functionality provided by `pgxls`.

### Configuration Methods

1. Configuration file specified by the `-config` flag
1. `workspace/configuration` set to LSP client
1. Configuration file located in the following location
    - `$XDG_CONFIG_HOME`/sqls/config.yml ("`$HOME`/.config" is used instead of `$XDG_CONFIG_HOME` if it's not set)

### Configuration file sample

```yaml
# Set to true to use lowercase keywords instead of uppercase.
lowercaseKeywords: false
connections:
  - alias: local_postgres
    driver: postgresql
    user: postgres
    passwd: mysecretpassword1234
    host: 127.0.0.1
    port: 5432
    dbName: dvdrental
    params:
      sslmode: disable
```

### Workspace configuration Sample

- setting example with nvim-lspconfig.

```lua
require'lspconfig'.sqls.setup{
  settings = {
    sqls = {
      connections = {
        {
          driver = 'postgresql',
          dataSourceName = 'host=127.0.0.1 port=15432 user=postgres password=mysecretpassword1234 dbname=dvdrental sslmode=disable',
        },
      },
    },
  },
}
```

## Contributors

This project exists thanks to all the people who contribute to the original `sqls` project.

## Inspired

I created sqls inspired by the following OSS.

- [dbcli Tools](https://github.com/dbcli)
    - [pgcli](https://www.pgcli.com/)
