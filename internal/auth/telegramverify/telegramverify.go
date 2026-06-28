package telegramverify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TelegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsBot     bool   `json:"is_bot"`
}

func VerifyInitData(initData, botToken string) (TelegramUser, int64, bool, error) {
	var empty TelegramUser
	if initData == "" || botToken == "" {
		return empty, 0, false, fmt.Errorf("initData and botToken are required")
	}
	values, err := url.ParseQuery(initData)
	if err != nil {
		return empty, 0, false, err
	}
	hash := values.Get("hash")
	if hash == "" {
		return empty, 0, false, fmt.Errorf("hash is missing")
	}
	values.Del("hash")

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		for _, v := range values[k] {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
	}
	dataCheckString := strings.Join(pairs, "\n")

	secretKey := hmac.New(sha256.New, []byte("WebAppData"))
	secretKey.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secretKey.Sum(nil))
	mac.Write([]byte(dataCheckString))
	expectedHash := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedHash), []byte(hash)) {
		return empty, 0, false, fmt.Errorf("invalid initData signature")
	}

	authDate := values.Get("auth_date")
	if authDate != "" {
		ts, err := strconv.ParseInt(authDate, 10, 64)
		if err != nil {
			return empty, 0, false, fmt.Errorf("invalid auth_date")
		}
		if time.Since(time.Unix(ts, 0)) > 24*time.Hour {
			return empty, 0, false, fmt.Errorf("expired initData")
		}
	}

	userJSON := values.Get("user")
	if userJSON == "" {
		return empty, 0, false, fmt.Errorf("user field is missing")
	}
	var user TelegramUser
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil {
		return empty, 0, false, err
	}
	chatID := int64(0)
	if chatIDStr := values.Get("chat"); chatIDStr != "" {
		var chat struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(chatIDStr), &chat); err == nil {
			chatID = chat.ID
		}
	}
	return user, chatID, true, nil
}
