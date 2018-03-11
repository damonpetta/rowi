# rowi

Read-Only markdown wiki server

# Running

## Docker

```
docker built -t rowi
docker run -ti -p3000:3000 -e GITHUB_WIKI_URL=https://github.com/damonpetta/rowi.wiki.git rowi:latest
```
