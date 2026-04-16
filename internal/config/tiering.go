// Package config reads shared application settings from Viper.
package config

import (
	"github.com/spf13/viper"
)

// HotContentThreshold returns the minimum UTF-8 byte length of document content for the auto hot path
// when quota allows (callers compare len(string) in bytes).
func HotContentThreshold() int {
	t := viper.GetInt("documents.tiering.hot_content_threshold")
	if t <= 0 {
		return 5000
	}
	return t
}

// ColdPromotionThreshold returns the query-count threshold at or above which a cold document
// is eligible for promotion to hot.
func ColdPromotionThreshold() int {
	t := viper.GetInt("documents.tiering.cold_promotion_threshold")
	if t <= 0 {
		return 3
	}
	return t
}
