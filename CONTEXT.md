# MLBB Collector

Collects Mobile Legends: Bang Bang hero data, from a hand-authored dataset and from Moonton's published statistics, and serves it to analyzers.

## Language

### Heroes

**Hero**:
A playable character, with one or more Roles and Lanes.

**Hero ID**:
A hero's slug, such as `tigreal`, the identity every authored record and API path uses.
_Avoid_: uid, slug

**Moonton ID**:
The number Moonton assigns a hero, used only to match measured data to a hero.
_Avoid_: mlid, main_heroid

**Role**:
A hero's combat class: tank, fighter, assassin, mage, marksman, or support.

**Lane**:
Where a hero is played: gold, exp, mid, roam, or jungle.

### Matchups

**Counter**:
An authored claim that one hero beats another, backed by Reasons and Proof.
_Avoid_: authored counter, counter pick

**Target hero**:
The hero a Counter beats.
_Avoid_: enemy hero

**Counter hero**:
The hero in a Counter that beats the Target hero.

**Synergy**:
An authored claim that two heroes are stronger together, backed by Reasons and Proof.

**Anchor hero**:
The hero a Synergy is written for.

**Synergy hero**:
The partner that makes the Anchor hero stronger.
_Avoid_: partner

**Reason**:
One sentence on why a Counter or Synergy holds.

**Counter type**:
A tag naming the kind of advantage a Counter gives, such as `anti-cc`.

**Synergy type**:
A tag naming the kind of advantage a Synergy gives, such as `engage-follow-up`.

**Proof**:
One concrete interaction supporting a Counter or Synergy, with a category, priority, impact, when it works best, and how it fails.

**Measured counter**:
A pair Moonton reports as one hero beating another, carrying a win-rate delta and no Proof.
_Avoid_: counter stat, sub hero

### Analysis

**Score**:
A 0-100 rating of how strong a Counter or Synergy is, produced by AI at request time and never authored.

**Confidence**:
A 0-100 rating of how well the Reasons and Proof support a Score.

### Statistics

**Rank tier**:
The player bracket Moonton reports on: all, epic, legend, mythic, honor, or glory.

**Window**:
How many days a statistic covers: 1, 3, 7, 15, or 30.

**Snapshot**:
Moonton's statistics for every hero in one Rank tier and Window, as fetched on one date.

**Win rate**:
The share of a hero's games that it won.

**Appearance share**:
A hero's share of all picks, summing to 1 across heroes.
_Avoid_: pick rate

**Ban rate**:
The share of games in which the hero was banned.

**Patch**:
One game release, identified by release date and version.

**Highlights**:
A Patch's headline changes as Liquipedia lists them.

## Relationships

- A **Hero** has one **Hero ID** and one **Moonton ID**
- A **Counter** has exactly one **Target hero** and one **Counter hero**; a **Synergy** has exactly one **Anchor hero** and one **Synergy hero**
- A **Counter** or **Synergy** has one or more **Reasons**, one or more types, and one or more **Proofs**
- A **Counter** names its heroes by **Hero ID**; a **Measured counter** names them by **Moonton ID**
- A **Score** and its **Confidence** belong to one **Counter** or **Synergy** and are never stored with it
- A **Snapshot** holds one **Win rate**, **Appearance share**, and **Ban rate** per **Hero** for its **Rank tier** and **Window**
- A **Snapshot** belongs to the latest **Patch** released on or before its date, and to none when it predates every known **Patch**

## Example dialogue

> **Dev:** "Moonton says Diggie beats Tigreal in today's glory Snapshot. Is that a **Counter**?"
> **Curator:** "No. It is a **Measured counter**. It becomes a **Counter** only when someone authors it with Reasons and Proof, with Tigreal as the **Target hero**."
>
> **Dev:** "Then can the AI raise the **Score** of the Diggie **Counter** because of it?"
> **Curator:** "Not today. A **Score** draws only on the **Counter**'s own Reasons and Proof."

## Flagged ambiguities

- "counter" meant both the authored pair the API serves and Moonton's `sub_hero` pair. Resolved: the authored pair is a **Counter**; Moonton's is a **Measured counter**.
- "hero id" meant both the slug and Moonton's number. Resolved: the slug is the **Hero ID**; the number is the **Moonton ID**. The field names `uid` and `mlid` remain in the dataset files, the tables, and the API.
- "pick rate" was used for `main_hero_appearance_rate`, which is a share of all picks, not a rate. Resolved: the term is **Appearance share**.
- "window" meant both how many days a statistic covers and how long a rate-limit counter lasts. Resolved: the days are the **Window**; the counter's span is the rate-limit window.
