package main

type Config struct {

	AccessToken string `json:"access_token"`
	JWTToken    string `json:"jwt_token"`

	CookieCheck bool   `json:"cookie_check"`
	CookiePath  string `json:"cookie_path,omitempty"`

	BanCheck             bool `json:"ban_check"`
	HypixelAPIKey        string `json:"hypixel_api_key"`
	HypixelCheck         bool   `json:"hypixel_check"`
	DonutCheck           bool `json:"donut_check"`
	IncludeHypixel       bool `json:"include_hypixel"`
	IncludeHypixelStats  bool `json:"include_hypixel_stats"`
	IncludeDonut         bool `json:"include_donut"`
	IncludeDonutStats    bool `json:"include_donut_stats"`
	MSRewards            bool `json:"ms_rewards"`
	XboxPerks            bool `json:"xbox_perks"`
	XboxCodes            bool `json:"xbox_codes"`
	NitroPromo           bool `json:"nitro_promo"`
	NitroValid           bool `json:"nitro_valid"`
	NitroTotal           int  `json:"nitro_total"`
	AttachMCToken        bool `json:"attach_mc_token"`
	Sniper               bool `json:"sniper"`

	GamepassPC           bool `json:"gamepass_pc"`
	GamepassUltimate     bool `json:"gamepass_ultimate"`
	MinecraftJavaGamepass bool `json:"minecraft_java_gamepass"`

	DonutBan           bool   `json:"donut_ban"`
	DonutUnbanned      bool   `json:"donut_unbanned"`
	DonutUnknown       bool   `json:"donut_unknown"`
	DonutAutoPay       bool   `json:"donut_auto_pay"`
	DonutAutopayTarget string `json:"donut_autopay_target"`

	HypixelBan      bool `json:"hypixel_ban"`
	HypixelUnban    bool `json:"hypixel_unban"`
	HypixelRanked   bool `json:"hypixel_ranked"`
	HypixelUnknown  bool `json:"hypixel_unknown"`

	Webhook                string `json:"webhook"`
	DefaultWebhook         string `json:"default_webhook"`
	HypixelBannedWebhook   string `json:"hypixel_banned_webhook"`
	HypixelUnbannedWebhook string `json:"hypixel_unbanned_webhook"`
	DonutBannedWebhook     string `json:"donut_banned_webhook"`
	DonutUnbannedWebhook   string `json:"donut_unbanned_webhook"`
	XboxHitsWebhook        string `json:"xbox_hits_webhook"`

	RateLimiting     bool `json:"rate_limiting"`
	RetryRateLimited bool `json:"retry_rate_limited"`

	ProxyMode string `json:"proxy_mode,omitempty"`

	ShowHits bool   `json:"show_hits"`
	Author   string `json:"author,omitempty"`

	HypixelPlanckeStats bool `json:"hypixel_plancke_stats"`
	OptifineCape        bool `json:"optifine_cape"`
	MinecraftCapes      bool `json:"minecraft_capes"`
	EmailAccess         bool `json:"email_access"`
	NamechangeCheck     bool `json:"namechange_check"`
	DonutSMPStats       bool `json:"donut_smp_stats"`
}
