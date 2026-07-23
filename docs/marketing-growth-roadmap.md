# Balkazar marketing and platform roadmap

Last updated: 2026-07-24

## Objective

Generate more suitable, better-paid bookings in Taiwan, particularly:

- 尾牙／春酒 and company events
- festivals and cultural programs
- clubs and live houses
- bars and restaurants offering recurring bookings
- weddings and private events
- press, collaborations, and monetization opportunities

The website and bilingual booking funnel are complete. This document records
the next promotion work so it can be resumed without reconstructing prior
decisions.

## Current public channels

| Channel | URL | Primary purpose |
|---|---|---|
| Website | https://balkazar.org | Booking conversion and search visibility |
| 尾牙 landing page | https://balkazar.org/zh/weiya/ | Taiwan company-event advertising |
| Electronic press kit | https://balkazar.org/epk/ | Promoters, press, and partners |
| Facebook | https://www.facebook.com/Balkazarband/ | Taiwan reach, events, reviews, and paid ads |
| Instagram | https://www.instagram.com/balkazarband/ | Visual proof, Reels, and venue tagging |
| YouTube | https://www.youtube.com/channel/UCVjIs4RvBlSH6NbnDAoqJTQ | Full live proof and Shorts |
| StreetVoice | https://streetvoice.com/Balkazar/ | Taiwan independent-music discovery |
| SoundCloud | https://soundcloud.com/user-424179644 | Listening archive and embedded audio |

## Priority 1: LINE Official Account

### Why

Provide Taiwan organizers with a familiar, low-friction way to ask about dates,
send venue photos, share schedules, and discuss budgets.

### Prerequisites from Marko

- Create a LINE Official Account owned by Balkazar.
- Choose the public display name `Balkazar`.
- Supply the public LINE add-friend URL.
- Supply the official QR image, preferably PNG or SVG.
- Decide who will monitor and answer inquiries.

### Website implementation

- Add `LINE 詢問演出` beside email and Instagram on both Chinese booking forms.
- Add the LINE link and QR code to `/zh/weiya/`.
- Add LINE to the Chinese EPK and footer.
- Keep the QR image in `assets/images/` with descriptive alternative text.
- Track website clicks with a future analytics event named `line_booking_click`.

### Suggested welcome message

> 嗨！謝謝你聯絡 Balkazar。若是演出邀約，請告訴我們活動日期、城市／場地、活動類型、預計人數與預算範圍。我們會依場地與需求建議適合的演出編制。

### Definition of done

- Add-friend link works on desktop and mobile.
- QR code scans successfully from another device.
- A test inquiry receives a response.
- No personal LINE account or private QR code appears publicly.

## Priority 2: short vertical performance clips

### Purpose

Create reusable awareness content for Instagram Reels, Facebook Reels, YouTube
Shorts, and optionally TikTok without maintaining four different productions.

### First content batch

Prepare 8–12 clips from the strongest available footage:

1. Crowd begins dancing.
2. Trombone or instrumental hook.
3. Sudden tempo or rhythm change.
4. Full-band festival wide shot.
5. Audience clap or singalong.
6. A recognizable song such as `Bella Ciao`, `Kalinka`, or `Niška Banja`.
7. Celebration Canada crowd/stage moment.
8. Fast montage ending with the booking URL.
9. Optional: band introduction in Mandarin.
10. Optional: direct 尾牙 pitch in Mandarin.

### Export specification

- Vertical `9:16`, ideally `1080 × 1920`.
- 15–30 seconds for discovery clips.
- One clear moment per clip.
- Burned-in Traditional Chinese subtitles for spoken material.
- Keep essential text inside the central safe area.
- Use music recorded by or licensed to Balkazar.
- Export a clean master without platform watermarks.

### Reusable closing text

English:

> Balkan fire from Taiwan. Bookings: balkazar.org

Traditional Chinese:

> 讓全場動起來｜演出邀約 balkazar.org/zh/

尾牙 version:

> 今年尾牙，不要再只是背景音樂。  
> balkazar.org/zh/weiya/

### Publishing cadence

- Begin with two strong clips per week for six weeks.
- Reuse the clean master across all selected platforms.
- Change the caption and opening text for the platform/audience.
- Tag the venue, event, photographer, and participating musicians when known.
- Reply promptly to credible organizer or venue comments.

### Content tracking sheet fields

- Clip ID
- Source performance
- Hook/first frame
- Duration
- Caption EN
- Caption ZH-TW
- Platforms published
- Publication dates
- Views
- Shares/saves
- Profile visits
- Website inquiries
- Notes for the next edit

## Priority 3: Facebook and Instagram advertising

### Do not launch until

- `https://balkazar.org` loads without a certificate warning.
- HTTPS enforcement is enabled in GitHub Pages.
- The booking form and Gmail fallback have been tested on mobile.
- Meta Business Manager access and payment method are confirmed.
- At least three strong vertical ad clips are ready.

### Landing page

All 尾牙／春酒 ads should lead directly to:

`https://balkazar.org/zh/weiya/`

Do not send paid traffic to the general homepage.

### Initial campaign structure

Campaign objective: qualified booking inquiries.

Suggested audiences:

- Taiwan event planners and event-production professionals
- HR, office administration, employee-experience, and company-event interests
- Hotel, banquet, restaurant, wedding, and venue professionals
- Retargeting website visitors and engaged social followers when sufficient data exists

Start geographically with Taipei, New Taipei, Taoyuan, Hsinchu, Taichung, and
Kaohsiung. Expand only after inquiry quality is understood.

### First creative angles

1. `今年尾牙，不要再只是背景音樂。`
2. Real dancing crowd plus `看得到的帶場能力`.
3. Flexible lineup for venue and budget.
4. International/Balkan sound for a company event guests will remember.

### Required UTM format

Every ad must use tagged links:

```text
https://balkazar.org/zh/weiya/?utm_source=facebook&utm_medium=paid_social&utm_campaign=weiya_2026&utm_content=video_crowd_01
```

For Instagram, change `utm_source=instagram`. Keep campaign and creative names
lowercase, short, and stable. The website includes UTM details in booking emails.

### Initial test discipline

- Test creative before making large budget increases.
- Compare qualified inquiries, not likes or video views.
- Record spend, inquiry count, qualified inquiry count, quotes, and confirmed bookings.
- Pause misleading or low-quality creative even if it has cheap clicks.
- Do not install tracking pixels or consent-requiring analytics without updating
  the site’s privacy disclosures and consent behavior.

### Core metrics

- Cost per qualified inquiry
- Inquiry-to-quote rate
- Quote-to-booking rate
- Confirmed booking value
- Return on advertising spend
- Which city, event type, and creative generated each lead

## Priority 4: music and event platforms

These accounts should be added only when their prerequisites exist. Empty or
stale profiles reduce trust.

### Spotify and Apple Music

Use when Balkazar has release-ready original or properly licensed recordings.

Needed:

- High-quality mastered audio
- Cover artwork
- Accurate songwriter, performer, and production credits
- Release date and metadata
- A distributor such as one selected through the official Spotify provider directory

After distribution:

- Claim Spotify for Artists and Apple Music for Artists.
- Add photos, biography, website, and social links.
- Pitch eligible unreleased music before release.
- Add the official profile links to the website and EPK.

Primary value: credibility, discovery, analytics, and streaming royalties.
Streaming should not be treated as the main booking lead source.

### Bandcamp

Use when Balkazar can sell original music, live recordings, merchandise, or
supporter downloads.

Needed:

- Rights-cleared recordings and artwork
- Pricing and payout ownership decision
- Optional merchandise inventory and fulfillment plan

Primary value: direct fan support, email/follower relationships, downloads, and
merchandise revenue.

### Bandsintown

Use when Balkazar has regular confirmed public shows with reliable dates and
ticket links.

Needed for every event:

- Confirmed date and start time
- Venue and city
- Event image
- Ticket or reservation link
- Complete lineup

Primary value: maintaining one concert calendar that can feed discovery across
music and search services. Add a website events widget only after the calendar is
kept current.

### Optional later channels

- TikTok: worthwhile when the vertical-video workflow is sustainable.
- Threads: lightweight Taiwan-scene networking and reuse of Instagram material.
- LinkedIn: use Marko’s professional profile for event agencies, HR contacts,
  hotels, and corporate planners rather than maintaining an empty band page.
- KKTIX/ACCUPASS: use for Balkazar-produced ticketed shows, not ordinary private bookings.

## Recommended execution order

1. Wait for and enforce GitHub Pages HTTPS.
2. Create the LINE Official Account and add it to the site.
3. Produce the first 8–12 vertical clips.
4. Publish organically for several weeks and identify the strongest creative.
5. Launch a controlled Meta ad test to `/zh/weiya/`.
6. Review lead quality and confirmed booking revenue.
7. Add Spotify/Apple Music, Bandcamp, or Bandsintown only when prerequisites are met.

## Future-session handoff checklist

At the beginning of a future implementation session:

- Read this document.
- Check `balkazar.org` HTTPS and GitHub Pages status.
- Ask only for missing public account URLs/assets relevant to the selected step.
- Never request or commit passwords, access tokens, payment details, or recovery codes.
- Use a branch, pull request, and merge because `main` is protected.
- Verify English and Taiwan mobile layouts before deployment.

