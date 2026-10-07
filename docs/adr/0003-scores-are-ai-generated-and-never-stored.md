# Scores are AI-generated and never stored

A Score and its Confidence are produced by an AI provider at request time, from the Reasons and Proof of the Counter or Synergy being scored, and returned without being written anywhere durable. The authored files reject any `score` or `scoreHint` field. Authored scores, as in the first static MVP, were rejected because a hand-picked number cannot be checked against the Proof behind it, and the Proof is what gets reviewed. Storing generated Scores was rejected because nothing reads a past Score; results are held in memory for a bounded time only to spare the provider quota.

## Consequences

Analysis fails when no AI provider is configured; local development uses the `mock` provider. The same Counter can receive a different Score once its cached result expires.
