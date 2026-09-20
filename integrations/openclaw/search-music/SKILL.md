---
name: search-music
description: Search or identify music through TuneScout using text, metadata, lyrics, or an audio excerpt. Use when clues are incomplete or conflicting and a model-guided multi-round search would improve recall; do not use it to modify media files or invent metadata without provider evidence.
---

# Search music with TuneScout

Use TuneScout as the evidence source and the current model as the search
controller. TuneScout is read-only and returns third-party provenance; it does
not make file-management decisions.

Read [references/search-loop.md](references/search-loop.md) when the first query
does not yield a high-confidence, well-supported result.

## Required configuration

- `TUNESCOUT_BASE_URL`: service URL without a trailing slash.
- `TUNESCOUT_API_KEY`: optional bearer token; keep it out of prompts and reports.

## Basic workflow

1. Extract only the clues already available: filenames, embedded tags, user
   context, short lyric fragments, and relevant CUE/NFO fields.
2. Call `POST /v1/search`. Use plain text first unless audio recognition is
   necessary. Send a short audio excerpt rather than a complete lossless file.
3. Inspect `provider_status`, provenance, conflicts, `rank_score`, and any
   `identity_confidence`. A rank score alone is not proof of identity.
4. If results are weak or ambiguous, expand or translate the query and repeat
   with a bounded search loop. Do not let model recollection become evidence.
5. Return the candidates and why they match. Leave adoption, tagging, movement,
   and deletion to the calling workflow.

Use `scripts/tunescout_client.py` for calls so credentials stay in environment
variables. Examples:

```bash
python3 scripts/tunescout_client.py search "王菲" --types artist,recording,release
python3 scripts/tunescout_client.py search --audio /path/to/short-excerpt.mp3 --types recording
python3 scripts/tunescout_client.py entity ENTITY_REF --include lyrics,releases,artwork,offers
```

Only send an audio file when the user has put it in scope for identification.

Prefer independent corroboration. An exact audio fingerprint or commercial
recognition result is strong evidence for a recording, but it may not identify
the exact release or edition. A keyword hit from one provider is a lead, not a
fact.
