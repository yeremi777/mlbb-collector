# Counter Proof Workflow

This file defines how short human matchup notes become reviewed counter proof data for `mlbb-collector`.

Use the word **counters**.

Counter data is the mirror of synergy data, but the relationship is different:

- Counter data explains how one hero stops, punishes, or denies another hero.
- Synergy data explains how one hero enables, protects, follows up, amplifies, or completes another hero's game plan.

---

## Main Goal

Create small, reviewable counter data first before building a large pipeline.

Start with:

- 1-5 target heroes per batch
- max 5 counter heroes per target hero
- reviewed Markdown first
- JSON only after user approval
- no static numeric scores
- clear reasons, proof notes, works-best conditions, failure cases, and source notes

---

## Dataset Location

Counter data lives in this repository, one file per Target hero:

```txt
data/
  heroes.json
  counters/
    <target-hero-id>.json
  synergies/
    <anchor-hero-id>.json
```

`docs/specs/dataset.md` is the authority on the shape and validation of these files. The API serves them at `GET /api/heroes/{heroId}/counters`.

---

## Source Rules

Use sources only to verify mechanics.

Allowed:

- manual curation from project knowledge
- official Mobile Legends pages for basic hero facts
- community wiki or public references for skill, item, or mechanic verification
- source notes for what was verified

Not allowed:

- copying third-party counter rankings
- copying tier lists
- copying matchup scores
- copying hidden methodologies
- scraping or crawling in bulk unless explicitly approved
- using raw source data directly as production data

Manual curation is preferred for counter reasoning.

---

## Key Terms

### Target Hero

The hero we are countering.

```txt
Target Hero: Tigreal
```

### Counter Hero

The hero that works well against the target hero.

```txt
Counter Hero: Diggie
```

### Main Threat

What the target hero wants to do in lane, skirmish, teamfight, pickoff, or objective control.

```txt
Tigreal wants to start teamfights by grouping enemies with crowd control.
```

### Counter Proof

A reviewed explanation of why the counter works, when it works best, and when it can fail.

---

## Short Human Input Format

The user may provide short notes like:

```md
Target Hero: Alucard
Counters:
1. Franco: suppress, lock, pickoff
2. Kaja: suppress, anti-dive
3. Baxia: anti-heal, tank
4. Karrie: true-damage, item, kite
```

Treat short notes as draft evidence, not final data.

Short tags are clues only. Expand them into reviewed Markdown before JSON.

---

## Short Tag Meaning

Use this mapping when expanding short notes:

| Short Tag     | Meaning                                                |
| ------------- | ------------------------------------------------------ |
| `skills`      | Direct skill interaction or hero mechanic              |
| `items`       | Item power spike or item counter                       |
| `tank`        | Durability, peel, frontline, reflect, or anti-burst    |
| `range`       | Range advantage or poke                                |
| `burst`       | Damage-type advantage                                  |
| `escape`      | Mobility advantage or cooldown window                  |
| `mobility`    | Dash, blink, movement speed, dodge, reposition         |
| `sustain`     | Sustain or anti-sustain context                        |
| `anti-heal`   | Sustain anti-sustain proof                             |
| `true-damage` | Damage type advantage                                  |
| `max-hp`      | Max HP damage proof                                    |
| `penetration` | Damage type advantage                                  |
| `cc`          | Crowd-control counter or skill interaction             |
| `anti-cc`     | Cleanse, control immunity, or CC reduction             |
| `kite`        | Kiting proof                                           |
| `vision`      | Vision awareness proof                                 |
| `teamfight`   | Teamfight role counter                                 |
| `pickoff`     | Single-target isolation or assassination               |
| `disengage`   | Anti-engage, pushback, reset, or peel                  |
| `protect`     | Backline protection or ally safety                     |

---

## Recommended Counter Types

Use lowercase kebab-case.

```txt
anti-cc
cleanse
anti-engage
disengage
peel
poke
burst
tank-shred
true-damage
penetration
anti-heal
kite
pickoff
teamfight
mobility
range-advantage
vision-control
objective-control
laning-advantage
game-phase
cooldown-dependent
positioning-dependent
execution-dependent
```

Avoid vague types:

```txt
strong
good
meta
op
easy-win
free-win
```

---

## Allowed Counter Proof Categories

Use only these categories:

```txt
skill-interaction
crowd-control-counter
damage-type-advantage
item-power-spike
mobility-advantage
range-advantage
kiting
sustain-anti-sustain
positioning-requirement
cooldown-window
vision-awareness
teamfight-role-counter
game-phase
execution-difficulty
```

---

## Priority Rules

Use:

```txt
primary
secondary
condition
```

Guideline:

- `primary`: main reason the counter works
- `secondary`: supporting reason such as item, damage profile, range, mobility, or game phase
- `condition`: requirement such as positioning, cooldown, timing, vision, or execution

---

## Impact Rules

Use:

```txt
low
medium
high
```

Guideline:

- `high`: strongly affects the matchup
- `medium`: useful but depends on condition
- `low`: minor supporting factor

Do not use numeric scores.

Do not include:

```txt
score
counterScore
proof_score
scoreHint
proof.scoreHint
```

---

## Markdown Review Output Format

Use this format before JSON:

```md
# Target Hero: [Hero Name]

## Main Threat

[Explain what the target hero wants to do in fights.]

## Counters

### 1. [Counter Hero]

Counter Types:
- [counter-type-1]
- [counter-type-2]

Reasons:
- [Specific human-readable reason.]
- [Specific human-readable reason.]

Proof:
- Category: [allowed CounterProof category]
  Priority: [primary | secondary | condition]
  Impact: [low | medium | high]
  Summary: [Concrete interaction summary.]
  Works best when:
  - [Condition]
  - [Condition]
  Failure cases:
  - [Failure case]
  - [Failure case]

Research Notes:
- [What was verified.]
- Source refs:
  - [URL or manual-curation ref]
```

---

## Example Markdown Output

```md
# Target Hero: Tigreal

## Main Threat

Tigreal wants to start fights with an AoE crowd-control engage, grouping enemies so his team can follow up with damage.

## Counters

### 1. Diggie

Counter Types:
- anti-cc
- cleanse
- teamfight

Reasons:
- Diggie can reduce the value of Tigreal's AoE crowd-control engage.
- Diggie protects nearby allies during Tigreal's main setup window.

Proof:
- Category: skill-interaction
  Priority: primary
  Impact: high
  Summary: Tigreal wants to start fights with AoE crowd control, while Diggie can protect nearby allies with cleanse and control immunity during the engage window.
  Works best when:
  - Diggie saves ultimate for Tigreal's real engage.
  - Diggie stays close enough to protect the teammates Tigreal wants to catch.
  Failure cases:
  - Tigreal baits Diggie's ultimate before committing.
  - Tigreal catches Diggie out of position.

Research Notes:
- Verified as manual-curation counter logic.
- Source refs:
  - manual-curation:counter-v1
```

---

## JSON Conversion Rules

Convert to JSON only when the user asks.

Final JSON is an array of Counter objects as `docs/specs/dataset.md` defines them.

Do not include Markdown.

Do not include comments.

Do not include numeric scores.

---

## JSON Shape

Final JSON follows the Counter and Proof tables in `docs/specs/dataset.md`. Every key is required, and `proof`, `worksBestWhen`, and `failureCases` each hold at least one entry.

---

## Example JSON Shape

```json
[
  {
    "targetHeroId": "tigreal",
    "counterHeroId": "diggie",
    "reasons": [
      "Diggie can reduce the value of Tigreal's AoE crowd-control engage.",
      "Diggie protects nearby allies during Tigreal's main setup window."
    ],
    "counterTypes": ["anti-cc", "cleanse", "teamfight"],
    "proof": [
      {
        "id": "diggie-time-journey-vs-tigreal-engage",
        "category": "skill-interaction",
        "priority": "primary",
        "impact": "high",
        "summary": "Tigreal wants to start fights with AoE crowd control, while Diggie can protect nearby allies with cleanse and control immunity during the engage window.",
        "worksBestWhen": [
          "Diggie saves ultimate for Tigreal's real engage.",
          "Diggie stays close enough to protect the teammates Tigreal wants to catch."
        ],
        "failureCases": [
          "Tigreal baits Diggie's ultimate before committing.",
          "Tigreal catches Diggie out of position.",
          "Tigreal engages while Diggie's ultimate is on cooldown."
        ]
      }
    ]
  }
]
```

---

## Review Rules

Before accepting counter data:

- Target hero exists in hero catalog.
- Counter hero exists in hero catalog.
- Target and counter hero are not the same.
- Reasons are specific and readable.
- Counter types use lowercase kebab-case.
- Proof category is allowed.
- Proof priority is valid.
- Proof impact is valid.
- Proof summary explains the interaction.
- At least one works-best condition is included.
- At least one failure case is included.
- Prefer direct skill interaction as primary proof.
- Item timing should usually be secondary proof.
- Positioning, cooldown, vision, and execution should usually be condition proof.
- Data does not copy proprietary rankings or tier lists.
- Source notes are included when mechanics were verified.

---

## Checking Before Seeding

After writing a JSON file:

1. `go test ./internal/dataset/...` loads the whole of `data/` and fails on any rule in `docs/specs/dataset.md`.
2. `make seed` syncs it into Postgres.

---

## Output Rules For Assistant

When generating reviewed Markdown:

- Use max 5 counter heroes per target hero unless requested otherwise.
- Use the user's known counter candidates first when they are valid.
- Add missing counter heroes only when there is strong reviewable evidence.
- Order counters by strongest reviewed evidence first.
- Keep language simple and practical.
- Use source notes when mechanics were verified.
- Say when something needs more verification.
- Do not generate JSON until the user approves or asks for JSON.

When generating JSON:

- Output valid JSON only.
- Use the Counter shape from `docs/specs/dataset.md`.
- Do not include numeric scores.
- Do not include comments.
- Do not include Markdown.
