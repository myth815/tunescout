# TuneScout（寻音）

TuneScout is a small, stateless federated music-search service. It queries
multiple third-party catalogs, normalizes their results, merges candidates
when the evidence is strong enough, and returns provenance with every hit.

It is intentionally **not** a music library, tag writer, downloader, player,
or autonomous agent. An agent such as OpenClaw can call TuneScout repeatedly,
expand ambiguous queries, and combine the returned evidence into a larger
curation workflow.

## What v0.1 does

- Search artists, recordings, and releases with text or structured clues.
- Accept a short audio upload for AcoustID and/or AudD recognition.
- Aggregate MusicBrainz, iTunes, LRCLIB, Audius, and optional Jamendo results.
- Expose legal preview, purchase, source-page, stream, and download offers when
  an upstream provider explicitly supplies them.
- Return an opaque, stateless `entity_ref` that can be expanded without a local
  catalog database.
- Isolate provider failures so one timeout can produce a partial result instead
  of failing the whole search.
- Apply optional `title`, `artist`, `release`/`album`, and duration filters after
  source normalization.

## Quick start

```bash
cp .env.example .env
docker compose up -d
curl 'http://127.0.0.1:8080/v1/search?q=王菲&types=artist,recording,release'
```

If `TUNESCOUT_API_KEY` is set, add `Authorization: Bearer <key>` to requests.

### Search with structured evidence

```bash
curl -X POST http://127.0.0.1:8080/v1/search \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "Heavenly Sword login screen music",
    "inputs": [
      {"type": "metadata", "fields": {"filename": "天剑.mp3"}},
      {"type": "text", "text": "PS3 game background music"}
    ],
    "types": ["recording"],
    "limit": 20
  }'
```

### Search with an audio clip

The multipart field `request` contains the same JSON request and `audio`
contains the clip. Keep clips short; TuneScout deletes the temporary upload as
soon as the request finishes.

```bash
curl -X POST http://127.0.0.1:8080/v1/search \
  -F 'request={"types":["recording"],"limit":10}' \
  -F 'audio=@sample.mp3'
```

### Expand a result

```bash
curl 'http://127.0.0.1:8080/v1/entities/<entity_ref>?include=lyrics,releases,artwork,offers'
```

## Providers

| Provider | Default | Capability |
|---|---:|---|
| MusicBrainz | on | artist, recording and release metadata |
| iTunes Search | on | catalog search, artwork, previews and store links |
| LRCLIB | on | recording metadata and lyrics lookup |
| Audius | on | independent/web music discovery |
| Jamendo | key required | independent music, licensed streams/downloads |
| AcoustID | key required | Chromaprint audio identification |
| AudD | token required | commercial audio recognition |

Keys are read only from environment variables and never returned by the API.
Use `/v1/providers` to inspect configured capabilities.
TuneScout serializes MusicBrainz requests at one request per second so multiple
search types and concurrent callers remain within the public service's usage
policy.

## OpenClaw integration

The reusable skill in [`integrations/openclaw/search-music`](integrations/openclaw/search-music)
keeps fuzzy reasoning outside this service. OpenClaw can search mechanically,
inspect candidate provenance, use an inexpensive model to expand weak clues,
and search again. Strong fingerprint matches do not need a model call.

## Architecture

Provider concurrency, failure isolation, entity references, and merge/ranking
orchestration live in a domain-neutral federation package. Music-specific
normalization and scoring are injected as a policy. See
[`docs/architecture.md`](docs/architecture.md) for the reuse boundary intended
for a future film/TV resolver.

## Development

TuneScout uses only the Go standard library. The runtime image adds the
official Chromaprint `fpcalc` bundle (including its audio decoding support) for
audio inputs.

```bash
go test ./...
go vet ./...
docker build -t tunescout:local .
```

The API contract is in [`api/openapi.yaml`](api/openapi.yaml).

## Rights and provider policy

TuneScout reports only the access offered by the upstream provider. It does not
bypass accounts, subscriptions, DRM, regional restrictions, or provider terms.
Free downloads are labeled as such only when the upstream API explicitly marks
them downloadable and supplies a license/source URL.

## License

MIT
