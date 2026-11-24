# Documentation

Bastion's documentation is published as a GitHub Wiki in **German** and **English**:

- Deutsch: <https://github.com/Inboxes-CIG/Bastion/wiki/DE-Start>
- English: <https://github.com/Inboxes-CIG/Bastion/wiki/EN-Home>
- Index: <https://github.com/Inboxes-CIG/Bastion/wiki>

The source of truth is the `docs/wiki/` directory in this repository. Each Markdown file maps 1:1 to a wiki page (`Home.md` is the wiki home, `_Sidebar.md` and `_Footer.md` are special wiki pages).

## Editing and publishing

1. Edit the Markdown files in `docs/wiki/`.
2. Publish them to the GitHub wiki:

```sh
./docs/wiki/sync.sh
```

The script clones `git@github.com:Inboxes-CIG/Bastion.wiki.git`, replaces the `.md` pages, commits and pushes. It requires an SSH key that can push to the repository.

To use a different remote (for example when testing):

```sh
WIKI_REMOTE=git@github.com:someone/other.wiki.git ./docs/wiki/sync.sh
```

## Adding a page

1. Create `docs/wiki/DE-Your-Page.md` and `docs/wiki/EN-Your-Page.md`.
2. Link them from `Home.md` and `_Sidebar.md`.
3. Run `./docs/wiki/sync.sh`.

## Language conventions

- German pages use the `DE-` prefix, English pages use `EN-`.
- Page titles in links are the file names without `.md`, e.g. `[[DE-Sites-und-Templates|Sites & Templates]]`.
- Keep both languages in sync when changing behavior.
