# MCChecker — Minecraft Account + Cookie Checker

> **Based on:** ShulkerV2 (reverse-engineered from garble-obfuscated binary v2.3.6)
> **Rebranded & extended:** MCChecker/1.0
> **Language:** Go

---

## What This Tool Does

MCChecker is a **Minecraft account checker** that supports both **email:password combos** and **cookie files**:

1. **Authenticates** Microsoft accounts through MS → Xbox Live → XSTS → Minecraft auth chain
2. **Cookie checking** — validates Netscape-format cookie files using `__Host-MSAAUTHP`
3. **Checks entitlements** — Java, Bedrock, Xbox Game Pass PC, Game Pass Ultimate
4. **Checks MS Rewards** points balance
5. **Checks MS Balance** (payment instruments)
6. **Checks Xbox Perks**
7. **Scans for Discord Nitro promo links** via Bing Rewards
8. **Checks Hypixel ban status** (banned / unbanned / never joined / rank)
9. **Checks DonutSMP ban status** (banned / unbanned / unknown)
10. **Sends Discord webhooks** with rich embeds for each hit

---

## File Structure

```
MCChecker/
├── config.json              ← your config (copy from config_template.json)
├── combos.txt               ← email:password list
├── proxies.txt              ← proxy list (optional)
├── capes.txt                ← capes filter (optional)
├── cookies/                 ← place cookie .txt files here (if cookie_check enabled)
│
├── Output files (auto-created):
├── all_hits.txt
├── cookie_valid.txt
├── cookie_invalid.txt
├── cookie_errors.txt
├── ms_valid.txt
├── hypixel_ban.txt
├── hypixel_unban.txt
├── hypixel_stats.txt
├── donut_unban_online.txt
├── donut_unban_offline.txt
├── donut_stats.txt
├── nitro_valid_codes.txt
├── nitro_promo_links.txt
├── valid_xbox_codes.txt
├── reward_point_hits.txt
├── ms_balance_hits.txt
└── ban_check_unknown_errors.txt
```

---

## Source Files

| File | Description |
|------|-------------|
| `main.go` | Entry point, banner, combo + cookie loading, concurrency loop |
| `auth.go` | Full MS/Xbox/Minecraft auth chain |
| `cookieauth.go` | Cookie file parsing + cookie-based auth flow |
| `checker.go` | Per-account and per-cookie checking logic |
| `webhook.go` | Discord webhook sending with rich embeds |
| `config.go` | Config struct (JSON tags) |
| `utils.go` | Thread-safe file writing, output file constants |

---

## Cookie Check Feature

To enable cookie checking:

1. Set `"cookie_check": true` in `config.json`
2. Place Netscape-format `.txt` cookie files in the `cookies/` directory
3. Run the checker — it will scan all `.txt` files, extract `__Host-MSAAUTHP` from `login.live.com`, and validate them through the full auth chain

---

## Auth Flow (Combos)

```
combos.txt → MS Login → Xbox Live → XSTS → Minecraft Auth → Profile → Value Checks
```

## Auth Flow (Cookies)

```
cookies/*.txt → Parse Netscape cookies → Extract __Host-MSAAUTHP → Outlook redirect → Login.live.com → Xbox SISU → Token exchange → Minecraft Auth → Value Checks
```
