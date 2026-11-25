# MusicOps / GigOps — Autonomous Gig-to-Growth Contract (v1)

**Status:** Implementation design attached to existing draft PR #53 and engineering issue #41. **Not deployed or authorized to publish.**
**Decision date:** 2026-11-14
**Owner:** Balkazar bandleader; reusable concept for Fuzzy Wall and solo music only through separate tenants, credentials and approval policies.

## Product promise / North star

> Owner tells MusicOps about a gig **once**, in ordinary chat or by referring to a booking email or flyer. MusicOps researches and reconciles facts, creates one canonical gig record, prepares the artwork/copy, chooses fitting distribution channels, drafts/validates and where **explicitly and durably authorized** publishes events and social posts, cross-links/monitors everything, provides an actionable stage/recording plan, and turns show-day recordings into a reviewed post-show media campaign. The owner is contacted only for missing consequential facts, exceptional permissions, risk, budgets and physical work.

**No** blanket auto-publishing across arbitrary services; **no** inventing tickets, organizer status, photographs, videos, crowd numbers, permission or attendance. **No** new paid AWS/API/AI usage by default.

## 1. Intake and canonical gig record

The conversation layer transforms a user statement (text, screenshot, approved read-only booking message) into a proposed gig. The model proposes facts with provenance but **never treats a guess as confirmation**.

Fields (nullable until validated): gig_id; artist/tenant; title; confirmed vs hold vs proposal vs cancelled; booking evidence/source link; performer-vs-organizer role; organizer/venue/municipality; show date/start/end and IANA timezone; admission/ticket/RSVP constraints; official event URL; lineup and performance format; public language(s); audience capacity if verified; technical contact; media and publishing permissions; internal commercial terms *kept private*; current source fingerprint; owner-approved campaign profile.

- **Confirmed show** may advance to publication planning; tentative shows can be researched privately only.
- Don't make a new event if a date/venue/artist match already exists; reconcile updates against the same gig_id and each live platform ID.
- Treat a guest slot inside a festival differently from a band-organized ticketed show. Never create a fake festival organizer or duplicate ticket-sale page.
- One source of truth for each class of fact: booking/correspondence for confirmed logistics, website event JSON-LD for public representation, MusicOps private Gig/Media catalog for workflow state, ClickUp for work/ownership, GitHub for code/site source, third-party websites for *their* live listing IDs.
- Ask the owner **one compact question only for a genuinely blocking unknown**. Otherwise continue reversible work with explicit "unknown" fields.

## 2. Outcome, audience and channel selection

Each gig sets desired outcomes, not just post counts: attenders from local scene, opt-in LINE/Instagram/Bandsintown follows, social shares and quality fan photos, new private/corporate booking inquiries, photographer-approved footage for the EPK, and repeat gigs.

Audience segments: (a) Taiwan indie/Balkan/folk/ska/roots niche, (b) Taipei general music and cultural-events audience, (c) overseas visitors, expats and tourists, (d) prospective festival/booker/corporate clients.

Choose channels from the existing [15-destination MusicOps Distribution Channels registry](https://docs.google.com/spreadsheets/d/1BzP3pOfa2Zl2QPQMoOT2k8PSDklvZnopG3a-jSSraIE/edit#gid=1746261124): owned website and Event JSON-LD; relevant artist accounts; FB Events; Bandsintown/Songkick; StreetVoice; Taipei Travel/文化快遞; selective English listings (Roam Taipei, Taiwan What's On); MUSICO/performing-arts calendars; *only if eligible*. Blow is editorial pitching, not automatic event posting. ACCUPASS/KKTIX are for legitimate organizer-led RSVPs/ticketing, not arbitrary third-party festival promotion.

Each adapter stores: audience fit, eligibility, account/artist ownership, current ToS, exact source facts, lead time, publication method (documented API / official import / authenticated manual form), fee cap, rights, existing live/draft IDs, retry safety, outcome visibility, last verification. Skip unsuitable channels rather than spam.

## 3. Content brief and media production

Generate a *source-grounded* creative brief before making assets: brand, audience, emotion, factual date/venue/role, CTA, tone in EN and natural Taiwan Traditional Chinese, visual identity, strongest authentic footage, artist captions, accessibility/alt text, photographer credit, minors/recognizable-fan privacy restrictions and music composition/master rights.

From the existing private Drive source-drop and MusicOps **Media Assets** tab, rank assets by source truth, quality, aspect, engagement, rights clearance and freshness. File name alone is **not** a rights grant or proof of event/date.

Outputs to generate as applicable (not all mandatory):
- Event cover / social artwork (e.g. Facebook cover, StreetVoice landscape, square 1:1, 4:5 feed image, 9:16 Story/reel thumbnail), each size verified against current platform requirements.
- Event title and approved bilingual description with official booking/organizer distinction, venue, start/end, free/ticket info only when verified, attribution and real call to action.
- One genuine short 15–45s hook clip and one longer 45–90s performance trailer selected from rights-cleared originals; optional captions and 0.0 music rights uncertainty fallback to silent photo carousel.
- Clear stage banner / printed QR sign to evergreen **https://balkazar.org/follow/** with readable **@balkazarband** and short URL. Physical printing/installation is a separate approval/human task, not a website side effect.
- Private editable masters and known-size exports; only approved derivatives become upload candidates. Never invent a photo of the band to pass off as genuine performance footage.

## 4. Distribution and campaign engine

Dependency order for confirmed gigs:
1. Verify venue, slot, event owner, official source, artist identity, published-vs-draft duplicates, rights and account access.
2. Prepare/update one bilingual owned event page and Event JSON-LD; link from show listing and follow hub.
3. Export/create channel-specific *reviewable* event drafts only where eligible; update state and canonical live URLs after **actual** submission/readback; add reciprocal links when platforms allow.
4. Build reusable content pack (approved text/images/reels) and a **bounded** publishing schedule using the existing Metricool brand where supported. Differentiate Facebook Events from Facebook Page posts.
5. Invite organic awareness only through owner-approved channels; no unsolicited cold messages to organizers, friends, bandmates or media, and no fabricated co-hosts.
6. Create a physical show-production pack: tech/stage timeline, 90-minute fixed recording + independent audio/camera fallback, visitor QR plan, warm audience photo opportunity, band wardrobe ideas; mark physical operations human-only.
7. Post-show: ingest originals without deleting sources, verify master/backup and rights, cut best genuine reel + photos, publish approved recap, credit collaborators, offer future-show follow link, collect verified performance outcomes and lessons.

### Default lightweight promotional timeline (adaptive to available lead time)
- **T−28 to T−14:** publish/update authoritative listing and first organic announcement; niche/Taiwan/traveler calendar submissions early enough for editorial review.
- **T−14 to T−7:** best authentic performance hook and event/artist announcement; one audience-specific angle rather than duplicate crosspost everywhere.
- **T−7 to T−2:** one restrained reminder, updated event links, partner/event official posts if publicly available and permitted.
- **T−1 / day-of:** accurate time/location, weather/logistics if verified, one Story/reminder; reduce noise.
- **After show, T+1 to T+7:** photos/reel recap and an opt-in invitation for *next* show. Full show recording archived privately until publishing/rights approval.

This is a planning default, not an automatic post quota or authorization. Stop irrelevant scheduled posts on cancellation/reschedule; changed show info propagates to approved listings. Campaign must not conflict with the owner's sleep, job or band performance.

## 5. Fan acquisition and conversion, not just reach

One evergreen /follow/ QR and consistent @balkazarband, with alternatives to LINE for visitors, working real-device scans, readable signs and a non-pushy in-show cue. LINE OA @489cncyj remains on the free plan; a four-button fan-oriented Rich Menu is a candidate, not yet confirmed live.

Measure (only when verifiable) where visitors originated, clicks to owned page, website event CTA taps, LINE friend adds, consented email subscribers if later activated, Instagram follows, qualified booking inquiries, verified video views and genuinely observed attendance. UTM/ref parameter and basic privacy-preserving aggregated analytics preferred. Never infer attendance from platform impressions or invent attribution. Separate leading metrics from real paid bookings/net revenue.

## 6. Autonomous state machine and exceptions

A run is a **persistent, resumable workflow per gig**, not a giant one-shot LLM prompt. Store artifact/version/fingerprint and platform receipts for idempotency and rollback:

`intake -> fact_check -> audience_plan -> asset_rights_check -> create_drafts -> owner_approval_if_needed -> queue -> execute -> public_readback -> crosslink -> monitor -> after_show -> reviewed`

For each asset/post/listing, track:
`candidate | blocked_missing_fact | blocked_rights | blocked_account | not_applicable | drafted | approval_required | queued | submitted | in_review | published_verified | error | superseded | withdrawn`.

- **Retries:** retry idempotently with the same key and expected platform outcome; on ambiguous third-party response perform read-only check before retrying to avoid duplicates.
- **Approvals:** store a machine-readable **bounded standing grant** per artist/channel/content type/date or gig class, with spend=0 default, license/media scope, legal terms, recipient conditions, expiry and revocation. An unpublished/unapproved draft is never silently counted as live.
- **Normal authorized operations:** research, check pages, create internal drafts, choose approved rights-cleared assets, generate copy/media, queue approved posts and verify outcomes. Public writes only after explicit durable scoped permission and supported provider integration.
- **Exceptions only:** contradictory booking facts, unclaimed/blocked platform accounts, unavailable source rights, vendor ToS restrictions, unapproved purchases, publication outside granted scope, venues needing production approval, account login/MFA and physical operations.
- **Human-only:** performing, camera mounting/running/monitoring, soundboard coordination, wardrobe purchase/placement, printing and installing banner, meeting audience, signing legal/commercial commitments. Human is not asked to do automatable repetitive file, copy, or crosslink work.
- Security: isolate artist tenants, avoid exposing credentials in public GitHub or ChatGPT, minimize sensitive personal data and comply with platform permissions and consent; do not evade CAPTCHA, bot restrictions or paid plan terms. Use existing subscriptions and hard-capped compute; no automatic paid infrastructure.

## 7. MVP shipping sequence and objective acceptance gates

**Stage A — intake-to-plan (engineering next):** one natural-language gig message creates one canonical draft record, validates conflicts and outputs audience/rights/channel/media/timing/production plans. Deduplication and cancellation/change scenarios tested with no external writes. Can first use LLM+ClickUp, then a small Go typed job schema.

**Stage B — drafts and owned surfaces:** produce verified bilingual website PR + a private, rights-gated cover/photo/video pack; propose publishing queue. Owner only receives exceptions.

**Stage C — scoped distribution:** approved provider adapters for existing Metricool, supported event-listing workflows; reconcile published URLs and links, handle re-runs and reschedules safely. Each platform enabled independently after real integration checks.

**Stage D — continuous media and post-show:** Drive asset ingestion, ranking, captions, audio/video edits and approvals; import real filmed assets; success measured by time saved and genuine follow/booking outcomes, not content count.

**Exit criteria before claiming "tell us one gig and done":**
- 2 consecutive independently confirmed Balkazar gigs processed end-to-end with no duplicate live events, false organizer/ticket claims, secret leakage, or unapproved spend/publication.
- All eligible chosen channels either have a **verified live public URL** or an explicit blocked/skipped reason; no false "done" statuses.
- For one show, an end-to-end asset source→rights→draft→approved publish→public proof→post-show loop is validated.
- Owner required only for truly irreducible facts/permissions and physical work. Measure actual owner minutes saved and marginal audience/business outcomes.
- Then consider separate Fuzzy Wall/solo accounts without sharing media, fan lists or credentials.

## Existing work to extend (don't fork or rewrite)

- [Issue #41](https://github.com/marko-durasic/balkazar.github.io/issues/41) — reusable permission-gated gig listing exporter/browser worker.
- [Draft PR #53](https://github.com/marko-durasic/balkazar.github.io/pull/53) — Go planner, GitHub Actions review bundle, media source preflight and read-only MCP. **Nothing in this PR is deployed to the public website or enabled as an external publisher.**
- [Balkazar growth task](https://app.clickup.com/t/z91464myw2), [December 3 campaign](https://app.clickup.com/t/z91464juf2), [LINE](https://app.clickup.com/t/86eyd0ay9), [stage QR/banner](https://app.clickup.com/t/86eydby2t).
- [MusicOps artist bookings/releases and Distribution Channels Sheet](https://docs.google.com/spreadsheets/d/1BzP3pOfa2Zl2QPQMoOT2k8PSDklvZnopG3a-jSSraIE/edit#gid=1746261124).

**Scope of this document:** executable requirements and milestone gates **only**. Creating this file does not authorize or perform account logins, paid subscriptions, event submissions, external messages, public postings, website merges, scheduling, or credential transfers.
