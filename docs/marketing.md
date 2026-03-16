# Getting Pike in Front of People

Pike's biggest sell is that it meets people where they already are — markdown files in a terminal. No new app to learn, no migration, no sync service. You're not asking anyone to change their workflow, just to see it better.

## Angles by Audience

### Terminal/neovim/obsidian users

Post in r/commandline, r/neovim, r/ObsidianMD, or Hacker News. A short "Show HN" with a GIF of the dashboard, filtering, and toggling a task — 10 seconds is enough. The hook: "I had 40 markdown files with tasks scattered everywhere. Pike scans them all and gives me a dashboard. No database, no syncing, just my files." These communities love single-purpose tools that do one thing well.

### PKM/Zettelkasten people

The "your files are the source of truth" philosophy is exactly what this crowd values. They're allergic to lock-in. Emphasize: Pike reads your files, never moves them, never reformats them, and works with any editor. Post in PKM forums, the Obsidian Discord, or Logseq communities.

### Developers

A `pike -q "open and @due < today" --json` one-liner in a cron job, a CI check, or a shell alias is compelling. Show it piped into `jq` or `wc -l`. Developers adopt tools that compose well with other tools.

## Action Items

1. Record a 30-second asciinema/VHS demo — dashboard, filter, toggle, tag search, back. Put it in the README.
2. Write a short blog post or dev.to article: "I built a terminal task dashboard that reads your markdown files"
3. Submit to Hacker News as "Show HN: Pike — terminal task dashboard for markdown notes"
4. Add pike to awesome-cli-apps, awesome-tui, and awesome-go lists via PR
5. Cross-post the GIF to Twitter/Mastodon/Bluesky with the one-liner install (`nix run github:zachthieme/pike`)

The tool sells itself once someone sees it working. The challenge is just getting that first 10-second look.
