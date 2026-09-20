# Bounded model-guided search

Use this loop only when the first TuneScout response is incomplete, ambiguous,
or empty.

1. Separate known facts from guesses. Preserve user-confirmed context as a
   query constraint; treat filenames, old tags, NFO text, and model deductions
   as weaker clues.
2. Generate at most three materially different query variants per round:
   aliases/translations, title-and-artist combinations, or context such as a
   game/film name plus `soundtrack`/`BGM`.
3. Query TuneScout again and compare stable provider identifiers, duration,
   artist, release, and lyrics evidence. Avoid repeating equivalent queries.
4. Stop when independent sources converge, when the result remains ambiguous
   after three rounds, or when provider failures prevent meaningful progress.

For an unidentified audio source, try in this order:

- TuneScout audio recognition using a representative excerpt;
- user/context keywords plus any detected language or lyrics;
- alternate-language names, transliterations, soundtrack/game/film context;
- a final deep comparison of the leading candidates.

Never manufacture a provider ID, direct media URL, license, or confidence
score. Report `unidentified` when the evidence does not support a match.
