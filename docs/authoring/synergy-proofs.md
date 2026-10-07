# Synergy Proof Workflow

This file defines how short human synergy notes become reviewed synergy proof data for `mlbb-collector`.

Use the word **synergies**, not **allies**.

Synergy data is similar to counter data, but the relationship is different:

- Counter data explains how one hero stops, punishes, or denies another hero.
- Synergy data explains how one hero enables, protects, follows up, amplifies, or completes another hero's game plan.

---

## Main Goal

Create small, reviewable synergy data first before building a large pipeline.

Start with:

- 1-5 anchor heroes per batch
- max 5 synergy heroes per anchor hero
- reviewed Markdown first
- JSON only after user approval
- no static numeric scores
- clear reasons, proof notes, works-best conditions, failure cases, and source notes

---

## Dataset Location

Synergy data lives in this repository, one file per Anchor hero:

```txt
data/
  heroes.json
  counters/
    <target-hero-id>.json
  synergies/
    <anchor-hero-id>.json
```

`docs/specs/dataset.md` is the authority on the shape and validation of these files. The API serves them at `GET /api/heroes/{heroId}/synergies`.

---

## Source Rules

Use sources only to verify mechanics.

Allowed:

- manual curation from project knowledge
- official Mobile Legends pages for basic hero facts
- community wiki or public references for skill, item, or mechanic verification
- source notes for what was verified

Not allowed:

- copying third-party synergy rankings
- copying tier lists
- copying matchup scores
- copying hidden methodologies
- scraping or crawling in bulk unless explicitly approved
- using raw source data directly as production data

Manual curation is preferred for synergy reasoning.

---

## Key Terms

### Anchor Hero

The hero we are building around.

```txt
Anchor Hero: Tigreal
```

### Synergy Hero

The hero that works well with the anchor hero.

```txt
Synergy Hero: Pharsa
```

### Main Game Plan

What the anchor hero wants to do in lane, skirmish, teamfight, pickoff, or objective control.

```txt
Tigreal wants to start teamfights by grouping enemies with crowd control.
```

### Synergy Proof

A reviewed explanation of why the synergy works, when it works best, and when it can fail.

---

## Short Human Input Format

The user may provide short notes like:

```md
Anchor Hero: Tigreal
Synergies:
1. Pharsa: follow-up, range, burst, teamfight
2. Odette: combo, cc-chain, teamfight
3. Claude: setup, AoE damage, engage follow-up
```

Treat short notes as draft evidence, not final data.

Short tags are clues only. Expand them into reviewed Markdown before JSON.

---

## Short Tag Meaning

Use this mapping when expanding short notes:

| Short Tag     | Meaning                                                        |
| ------------- | -------------------------------------------------------------- |
| `setup`       | One hero creates an opening for another hero                   |
| `follow-up`   | One hero can immediately continue after the anchor hero starts |
| `cc`          | Crowd control interaction                                      |
| `cc-chain`    | Multiple crowd-control effects chained together               |
| `burst`       | Fast damage follow-up                                          |
| `poke`        | Long-range damage pressure                                     |
| `range`       | Range advantage or safe follow-up                             |
| `protect`     | Backline protection or ally safety                            |
| `peel`        | Stops enemies from diving an important teammate               |
| `frontline`   | Creates space for damage dealers                             |
| `dive`        | Helps enter the enemy backline                              |
| `mobility`    | Helps chase, reposition, or join fights                     |
| `vision`      | Helps reveal, scout, or control map areas                   |
| `sustain`     | Healing, shielding, or extended fight value                  |
| `shield`      | Shielding or damage absorption                              |
| `heal`        | Healing or recovery support                                 |
| `teamfight`   | Strong combined value in grouped fights                     |
| `pickoff`     | Strong single-target catch or isolation combo               |
| `siege`       | Helps pressure towers or objectives from range              |
| `objective`   | Helps turtle, lord, turret, or zone control                 |
| `early-game`  | Synergy is strongest early                                  |
| `late-game`   | Synergy is strongest late                                   |
| `cooldown`    | Depends on key skill cooldowns                              |
| `positioning` | Depends on spacing or formation                            |
| `execution`   | Requires timing or coordinated play                        |

---

## Recommended Synergy Types

Use lowercase kebab-case.

```txt
engage-follow-up
cc-chain
setup-combo
burst-follow-up
poke-siege
pickoff-combo
teamfight-combo
protect
peel
frontline-enabler
backline-protection
mobility-enable
dive-support
vision-setup
healing-sustain
shielding
objective-control
laning-synergy
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
best-combo
```

---

## Allowed Synergy Proof Categories

Use only these categories:

```txt
skill-interaction
crowd-control-chain
engage-follow-up
setup-combo
damage-amplification
protection
peel
frontline-enabler
mobility-enabler
vision-setup
healing-sustain
shielding
poke-siege
pickoff-combo
teamfight-combo
objective-control
laning-synergy
game-phase
positioning-requirement
cooldown-window
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

- `primary`: main reason the synergy works
- `secondary`: supporting reason such as damage profile, protection, range, item timing, or game phase
- `condition`: requirement such as positioning, cooldown, timing, vision, team coordination, or execution

---

## Impact Rules

Use:

```txt
low
medium
high
```

Guideline:

- `high`: strongly affects the synergy
- `medium`: useful but depends on condition
- `low`: minor supporting factor

Do not use numeric scores.

Do not include:

```txt
score
synergyScore
proof_score
scoreHint
proof.scoreHint
```

---

## Markdown Review Output Format

Use this format before JSON:

```md
# Anchor Hero: [Hero Name]

## Main Game Plan

[Explain what the anchor hero wants to do.]

## Synergies

### 1. [Synergy Hero]

Synergy Types:
- [synergy-type-1]
- [synergy-type-2]

Reasons:
- [Specific human-readable reason.]
- [Specific human-readable reason.]

Proof:
- Category: [allowed SynergyProof category]
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
# Anchor Hero: Tigreal

## Main Game Plan

Tigreal wants to start teamfights by pulling or grouping enemies into a crowd-control window so his team can follow up with damage.

## Synergies

### 1. Pharsa

Synergy Types:
- engage-follow-up
- teamfight-combo
- range-follow-up

Reasons:
- Tigreal can create a grouped enemy setup, giving Pharsa a clear window to land long-range burst damage.
- Pharsa can follow Tigreal's engage from safer distance, reducing the need to stand inside the initial fight.

Proof:
- Category: engage-follow-up
  Priority: primary
  Impact: high
  Summary: Tigreal creates the crowd-control opening, while Pharsa can use long-range burst to punish enemies caught in the setup.
  Works best when:
  - Tigreal engages after Pharsa is close enough to follow up.
  - Pharsa saves her burst for the real crowd-control window.
  Failure cases:
  - Tigreal engages too far away from Pharsa.
  - Pharsa is forced to reposition before the follow-up damage lands.
  - The enemy spreads out and avoids grouped engage.

Research Notes:
- Verified as manual-curation synergy logic.
- Source refs:
  - manual-curation:synergy-v1
```

---

## JSON Conversion Rules

Convert to JSON only when the user asks.

Final JSON is an array of Synergy objects as `docs/specs/dataset.md` defines them.

Do not include Markdown.

Do not include comments.

Do not include numeric scores.

---

## JSON Shape

Final JSON follows the Synergy and Proof tables in `docs/specs/dataset.md`. Every key is required, and `proof`, `worksBestWhen`, and `failureCases` each hold at least one entry.

---

## Example JSON Shape

```json
[
  {
    "anchorHeroId": "tigreal",
    "synergyHeroId": "pharsa",
    "reasons": [
      "Tigreal can create a grouped crowd-control setup that gives Pharsa a clear long-range damage window.",
      "Pharsa can follow Tigreal's engage from safer distance instead of standing inside the initial fight."
    ],
    "synergyTypes": ["engage-follow-up", "teamfight-combo", "range-follow-up"],
    "proof": [
      {
        "id": "tigreal-setup-for-pharsa-burst",
        "category": "engage-follow-up",
        "priority": "primary",
        "impact": "high",
        "summary": "Tigreal creates the crowd-control opening, while Pharsa can use long-range burst to punish enemies caught in the setup.",
        "worksBestWhen": [
          "Tigreal engages after Pharsa is close enough to follow up.",
          "Pharsa saves her burst for the real crowd-control window."
        ],
        "failureCases": [
          "Tigreal engages too far away from Pharsa.",
          "Pharsa is forced to reposition before the follow-up damage lands.",
          "The enemy spreads out and avoids grouped engage."
        ]
      }
    ]
  }
]
```

---

## Review Rules

Before accepting synergy data:

- Anchor hero exists in hero catalog.
- Synergy hero exists in hero catalog.
- Anchor and synergy hero are not the same.
- Reasons are specific and readable.
- Synergy types use lowercase kebab-case.
- Proof category is allowed.
- Proof priority is valid.
- Proof impact is valid.
- Proof summary explains the interaction.
- At least one works-best condition is included.
- At least one failure case is included.
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

- Use max 5 synergy heroes per anchor hero unless requested otherwise.
- Use the user's known synergy candidates first when they are valid.
- Add missing synergy heroes only when there is strong reviewable evidence.
- Order synergies by strongest reviewed evidence first.
- Keep language simple and practical.
- Use source notes when mechanics were verified.
- Say when something needs more verification.
- Do not generate JSON until the user approves or asks for JSON.

When generating JSON:

- Output valid JSON only.
- Use the Synergy shape from `docs/specs/dataset.md`.
- Do not include numeric scores.
- Do not include comments.
- Do not include Markdown.
