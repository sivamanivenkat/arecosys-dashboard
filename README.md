# AR ECOSYS operations dashboard

Open `index.html` for the interactive sample dashboard. The range selector changes the website and Instagram samples. The fleet and lead examples are a fixed snapshot. Sample lead stage changes and sample email replies last for the current tab session.

Run the local data server from this directory with Go 1.24 or later.

```powershell
go run server.go gmail.go
```

Open `http://127.0.0.1:8765`. The server listens on loopback. When enabling any private connection, set `ARECOSYS_DASHBOARD_PASSWORD` in the server environment before starting. Sign in as `arecosys`. Keep credentials out of the source tree and use an authenticated HTTPS deployment to share the dashboard with a team.

## Website and fleet data

The server can read three independent HTTPS endpoints. Configure `ARECOSYS_WEBSITE_URL`, `ARECOSYS_FLEET_URL` and `ARECOSYS_INSTAGRAM_URL` for the sources available to you. Each also accepts a corresponding optional `ARECOSYS_<SOURCE>_TOKEN` bearer credential. The server sends a `days` query parameter of `7`, `30` or `90` and expects the following JSON shapes. Website and Instagram responses may also include a `previous` object with the same numeric fields for the preceding period of equal length (`sessions`, `pageviews` and `leads` for the website; `reach`, `engagements` and `profileVisits` for Instagram). When present, the cards show the change against it. A missing connection uses clearly marked samples. A configured source that fails shows an unavailable state.

- **Website** returns `sessions`, `pageviews`, `leads`, `traffic`, `inquiries`, `channels`, `pages` and `actions`. Time series entries are `{ label, value }`. `channels` contains percentage shares in `organic`, `direct`, `social` and `referral`. `pages` contains `{ name, path, views }` and `actions` contains `{ name, count }`.
- **Fleet** returns `features`, an array of `{ id, name, metric, value, status, details }`, and optionally `wards`, an array of `{ name, completed, scheduled, missed }`, and `drilldowns`, an object keyed by feature `id` whose values are arrays of flat records (for example offline vehicles, missed stops by ward, weighbridge mismatches). The first six keys of the records become table columns in the feature's detail view, capped at 50 rows. `details` maps display labels to values. Use `On track` and `Needs attention` for the fleet filters. On the fleet page, connected data refreshes every 30 seconds while the tab is visible and its controls are idle. Report physical GPS freshness based on the timestamp of the latest vehicle ping. The ESG & recovery page reads an optional `esg` object: `periods`, an array of `{ label, Wet, Dry, Plastic, Reject }` in tonnes, and `pendingExports`, a count.
- **Instagram** returns `reach`, `engagements`, `followers`, `profileVisits` and `series`. `series` contains `{ label, value }` entries. Use the Meta Insights period total for unique reach. Summing daily reach can count the same person multiple times. Return missing fields as `null` when the account or API cannot supply a metric.

Instrument confirmed website signups and contact form submissions separately from outbound clicks on WhatsApp, phone, email and service CTAs. The page report includes the public site's main navigation, Modules and Impact. The fleet adapters need WasteFlow access to populate ward routes, bulk generator onboarding, GPS, offline sync, custody waybills, weighbridge reconciliation, waste streams, fuel alerts and EV charging.

## Lead intake

Set `ARECOSYS_LEAD_FILE` to a private writable JSON file path and `ARECOSYS_LEAD_TOKEN` to a secret for automated intake. The store is for a single server instance and persists leads and stage changes across restarts.

Send confirmed submissions from your website backend or a trusted integration with `POST /api/leads`, `Content-Type: application/json` and `Authorization: Bearer <lead token>`. Keep the token on the server. The request accepts `email` and optional `id`, `name`, `company`, `interest`, `source`, `campaign` and `note`. Reuse the submission's `id` when retrying to avoid duplicate leads. New leads appear in the Leads view on refresh. Each lead has an owner (set in the lead details) and a response timer: the target is one business day from receipt, weekends skipped, and moving a lead out of New records the first response. Overdue leads are counted on the Overview card. `PATCH /api/leads/{id}` accepts `status`, `assignee` and `note`. Every lead keeps a timeline of its arrival, stage and owner changes and notes, shown in the lead details. Intake accepts `phone` instead of `email` (at least one is required), so WhatsApp and phone leads can be stored. WhatsApp and Instagram clicks belong in analytics. A confirmed conversation from either channel can be forwarded to lead intake by its integration.

Suggested source values match the website's paths to contact the business, including Website contact form, Website signup, WhatsApp and Email. Use `interest` for ward pilots, EV fleet, ERP access, partnership inquiries or the selected service.

## Gmail inbox and replies

Enable the Gmail API in the Google project that owns the OAuth client. Authorize the intended mailbox with offline access and the `gmail.readonly` and `gmail.send` scopes. Supply the resulting client ID, client secret and refresh token with `ARECOSYS_GMAIL_CLIENT_ID`, `ARECOSYS_GMAIL_CLIENT_SECRET` and `ARECOSYS_GMAIL_REFRESH_TOKEN`. The [Gmail API guide](https://developers.google.com/workspace/gmail/api/guides) covers OAuth and the mailbox API.

The dashboard displays the most recent 15 inbox messages, using plaintext where available and the Gmail snippet for HTML-only mail. Replies are sent to the original message's Reply-To or From address in the same Gmail thread. Unread indicators clear locally when a message is opened. Gmail keeps the mailbox's read state and sent history. Provider credentials are consumed by the Go server.

## Design studio and Instagram publishing

The Design studio page holds an image library (folders), a post editor, and a post queue. Layouts are photo with headline at the bottom or centered, solid colour headline, big number (for a statistic), and image only. Headline type is sized automatically to fill the slide, an optional button text adds a call to action, and any library image can be set as the logo. Slides can be added, duplicated and reordered to build a carousel. Set `ARECOSYS_MEDIA_DIR` to a private writable directory to keep images and drafts. Without it the editor still works, but nothing is saved and the queue is hidden.

Workflow: upload brand images to a folder, pick one, choose a template, headline and caption, then **Save draft**. The editor exports a 1080 x 1350 JPEG to the Posts folder and adds the post to the queue. Posts move **Draft, Approved, Scheduled, Published**. Only approved posts can be scheduled or published, and **Publish now** asks for a second click to confirm. A post that fails shows its error and can be approved again. If the server stops while a post is publishing, that post is marked Failed instead of being retried, because it may already be on Instagram. Check Instagram before approving it again.

Publishing uses the Instagram API with Instagram Login and needs an Instagram professional (Business or Creator) account:

1. Create a Meta app with the Instagram API product and the `instagram_business_content_publish` permission, then generate a long-lived access token for the account. Meta's [content publishing guide](https://developers.facebook.com/docs/instagram-platform/content-publishing) covers the setup, test users and app review requirements.
2. Set `ARECOSYS_INSTAGRAM_USER_ID` and `ARECOSYS_INSTAGRAM_ACCESS_TOKEN` in the server environment.
3. Set `ARECOSYS_PUBLIC_BASE_URL` to the dashboard's public HTTPS address, for example `https://dashboard.example.com`. Instagram downloads each image from this address. The server hands out a signed link, valid for 15 minutes, that serves only the image of a post that is publishing at that moment. Everything else stays behind the dashboard password. Scheduling and publishing stay disabled until all three values are set, and publishing cannot work from `localhost`.

Long-lived tokens expire after about 60 days, and the dashboard does not refresh them yet. Replace the token before it expires. Meta also limits how many posts an account can publish through the API each day. A post can hold 1 to 10 slides. Several slides publish as an Instagram carousel, and all slides share the caption. Videos and Reels are not supported.
## Sign-in, roles and the activity log

By default one shared password (``ARECOSYS_DASHBOARD_PASSWORD``, user ``arecosys``) signs in as an admin. For individual accounts, set ``ARECOSYS_USERS_FILE`` to a JSON file listing each person:

```json
[
  { "name": "meera", "role": "editor", "hash": "pbkdf2-sha256$600000$..." },
  { "name": "board", "role": "viewer", "hash": "pbkdf2-sha256$600000$..." }
]
```

Create each hash with ``go run . hash-password`` and type the password when prompted (at least 10 characters). Run it interactively: piping a password in from Windows PowerShell 5.1 does not reach the program intact. Roles:

- **viewer** can read everything and change nothing.
- **editor** can also update leads, add notes, upload images and draft or approve posts.
- **admin** can additionally schedule or publish posts, send email replies, and open the Activity log.

After eight failed sign-ins within 15 minutes, that address is blocked for the rest of the window, even with the right password. Behind a reverse proxy set ``ARECOSYS_TRUST_PROXY=1`` so the client address comes from the proxy's ``X-Forwarded-For`` header (the Docker setup does this). Without a proxy, leave it unset, because the header can be forged.

The Activity log records failed sign-ins and every change request with who made it, the address, the endpoint and the result. It never records request bodies or query strings. It keeps the last 500 events in memory. Set ``ARECOSYS_AUDIT_FILE`` to also append every event to a file so the log survives restarts.

## Alerts and the weekly summary

Set ``ARECOSYS_ALERT_EMAIL`` (sent through the connected Gmail mailbox) and/or ``ARECOSYS_ALERT_WEBHOOK_URL`` (an HTTPS endpoint that accepts ``{"text": "..."}``, which suits Slack-compatible webhooks). Every five minutes the server checks for leads past the one-business-day response target and for fleet features that have moved to Needs attention, and sends one message listing only what is new. A problem that clears and returns alerts again. On start-up the server only records what is already wrong, so a restart does not repeat old alerts. A failed send is retried at the next check.

On Monday mornings (after 08:00 server time) the same channels receive a plain-text weekly summary: leads received and by stage, how many are past target, website sessions, Instagram reach, posts published and fleet exceptions. Alert state is kept next to the lead file, so a restart does not resend the day's summary.

## Built-in Google Analytics and Instagram Insights

Instead of writing an adapter, the website and Instagram sources can read the platforms directly. An explicit ``ARECOSYS_WEBSITE_URL`` or ``ARECOSYS_INSTAGRAM_URL`` always takes priority.

- **Google Analytics 4:** create a service account, add its email to the GA4 property as a Viewer, and download its JSON key. Set ``ARECOSYS_GA4_PROPERTY_ID`` (the numeric property ID) and ``ARECOSYS_GA4_CREDENTIALS_FILE``. Optionally set ``ARECOSYS_GA4_LEAD_EVENT`` (default ``generate_lead``) and ``ARECOSYS_GA4_ACTION_EVENTS``, a comma-separated list of event names to show under Inquiry activity, such as ``whatsapp_click,sign_up``. Channel shares cover Organic Search, Direct, Organic Social and Referral. Other channels count toward sessions but are not shown.
- **Instagram Insights:** set ``ARECOSYS_INSTAGRAM_INSIGHTS=1`` alongside the Instagram user ID and access token used for publishing. The token also needs the insights permission (``instagram_business_manage_insights``). Unique reach comes from the API's own period total. If the API refuses a period total for a long range such as 90 days, reach, engagements and profile visits show as empty rather than a guessed number.

The WasteFlow fleet source has no built-in reader, because it needs access to WasteFlow's own API. Point ``ARECOSYS_FLEET_URL`` at an endpoint that returns the shape described above.

## WhatsApp lead intake

Set ``ARECOSYS_WHATSAPP_VERIFY_TOKEN`` (any secret string you choose) and ``ARECOSYS_WHATSAPP_APP_SECRET`` (your Meta app secret). In the Meta app's WhatsApp configuration, set the webhook callback to ``https://your-domain/webhooks/whatsapp`` with that verify token, and subscribe to ``messages``. Every request is checked against the ``X-Hub-Signature-256`` signature before it is read. A first message creates a lead from the sender's name and number. Later messages from the same number add to that lead's timeline until the lead is Closed. Meta retries deliveries, and repeated messages are ignored. Only text is stored; other message types appear as a placeholder.

## Findings from the public site

- The dispatch-to-plant workflow adds monitoring for ward completion, QR-tagged generators, scale mismatches and recovery by waste stream. [ERP overview](https://www.arecosys.com/erp) · [System Modules](https://www.arecosys.com/modules) · [Operational Impact](https://www.arecosys.com/impact)
- The contact form accepts ward pilot, EV, ERP and partnership inquiries. The site also offers email signups and direct contact. A useful next addition is lead assignment with a first-response timer based on the site's one-business-day response target. [Homepage](https://www.arecosys.com/) · [Contact Us](https://www.arecosys.com/contact-us)
- Municipal and fleet reporting point to missed-stop recovery, vehicle maintenance, idle fuel use and charge readiness as useful next drill-downs. [NYC indicator definitions](https://www.nyc.gov/html/ops/downloads/pdf/pmmr2013/2013_idf.pdf) · [Geotab fleet reporting](https://support.geotab.com/help/mygeotab/reports/managing-reports/reporting-in-mygeotab) · [EV charge monitoring](https://support.geotab.com/help/mygeotab/reports/sustainability-reports/ev-charge-monitoring)

## Deployment

`docker-compose.yml` runs the dashboard behind Caddy, which handles HTTPS certificates. Copy `.env.example` to `.env`, set `DASHBOARD_DOMAIN`, the dashboard password and the lead token, point the domain's DNS at the host, then run `docker compose up -d --build`. Leads persist in the `leads` volume. The server listens on `ARECOSYS_HOST` (default `127.0.0.1`); the container sets `0.0.0.0` and the server refuses to start on a non-loopback address without `ARECOSYS_DASHBOARD_PASSWORD`. Only Caddy publishes ports. Put the users file, and the Google credentials file if used, in a `config` folder next to `docker-compose.yml` (it is mounted read-only at `/config`), and set `ARECOSYS_USERS_FILE=/config/users.json` in `.env`.