# Architecture and reuse boundary

TuneScout separates a small federation kernel from the music domain.

```text
HTTP API
  -> music query policy
  -> federation engine
       -> provider adapters
       -> failure isolation
       -> entity clustering
       -> ranking
       -> stateless entity references
  -> normalized music response
```

## Federation kernel

`internal/federation` owns behavior that is independent of music:

- concurrent provider execution;
- provider selection and capability reporting;
- timeouts and partial-result semantics;
- result clustering through injected merge keys and identity guards;
- deterministic result finalization;
- opaque entity references and fan-out detail lookups.

The kernel does not know what an artist, album, actor, film, recording, or
release means.

## Music domain

`internal/music` interprets music inputs and defines entity merge/ranking rules.
`internal/provider` currently contains music adapters. Provider payloads are
normalized before they reach the federation engine.

## Future film/TV service

A film service should be a separate API/product, not a `domain=movie` switch in
TuneScout. It can reuse or extract the federation kernel and supply:

- film entities such as person, work, edition, episode, and release;
- TMDB/TVDB/OMDb or other authorized provider adapters;
- film-specific alias, runtime, year, credit, and edition matching;
- its own public API schema and agent skill.

This avoids a universal-media schema while preserving the expensive, proven
parts of provider orchestration.

## Agent boundary

The first release does not embed a model loop. Agents consume the API as a
tool: search, inspect evidence, expand the query, and search again. This keeps
provider secrets out of model context and lets the same deterministic service
work with OpenClaw, another agent, a script, or a future web UI.
