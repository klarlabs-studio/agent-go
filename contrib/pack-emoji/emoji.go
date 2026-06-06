// Package emoji provides emoji utilities for agents.
package emoji

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the emoji utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("emoji").
		WithDescription("Emoji utilities").
		AddTools(
			extractTool(),
			removeTool(),
			countTool(),
			replaceTool(),
			hasEmojiTool(),
			toShortcodeTool(),
			fromShortcodeTool(),
			listTool(),
			categoryTool(),
			sentimentTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Common emoji mappings
var emojiToShortcode = map[string]string{
	"😀": ":grinning:", "😃": ":smiley:", "😄": ":smile:", "😁": ":grin:",
	"😆": ":laughing:", "😅": ":sweat_smile:", "🤣": ":rofl:", "😂": ":joy:",
	"🙂": ":slightly_smiling_face:", "😉": ":wink:", "😊": ":blush:",
	"😍": ":heart_eyes:", "🥰": ":smiling_face_with_hearts:", "😘": ":kissing_heart:",
	"😋": ":yum:", "😎": ":sunglasses:", "🤔": ":thinking:",
	"😐": ":neutral_face:", "😑": ":expressionless:", "😶": ":no_mouth:",
	"😏": ":smirk:", "😒": ":unamused:", "🙄": ":roll_eyes:", "😬": ":grimacing:",
	"😔": ":pensive:", "😕": ":confused:", "🙁": ":slightly_frowning_face:",
	"☹️": ":frowning:", "😢": ":cry:", "😭": ":sob:", "😤": ":triumph:",
	"😠": ":angry:", "😡": ":rage:", "🤬": ":face_with_symbols_on_mouth:",
	"😱": ":scream:", "😨": ":fearful:", "😰": ":cold_sweat:",
	"🤗": ":hugs:", "🤭": ":hand_over_mouth:", "🤫": ":shushing_face:",
	"👍": ":thumbsup:", "👎": ":thumbsdown:", "👏": ":clap:", "🙌": ":raised_hands:",
	"👋": ":wave:", "✋": ":hand:", "🤝": ":handshake:", "🙏": ":pray:",
	"❤️": ":heart:", "💔": ":broken_heart:", "💕": ":two_hearts:", "💖": ":sparkling_heart:",
	"⭐": ":star:", "🌟": ":star2:", "✨": ":sparkles:", "💫": ":dizzy:",
	"🔥": ":fire:", "💯": ":100:", "✅": ":white_check_mark:", "❌": ":x:",
	"⚠️": ":warning:", "❗": ":exclamation:", "❓": ":question:",
	"🎉": ":tada:", "🎊": ":confetti_ball:", "🎁": ":gift:", "🎂": ":birthday:",
	"☀️": ":sunny:", "🌙": ":crescent_moon:", "⛅": ":partly_sunny:", "🌧️": ":cloud_with_rain:",
	"🐶": ":dog:", "🐱": ":cat:", "🐭": ":mouse:", "🐰": ":rabbit:",
	"🍎": ":apple:", "🍕": ":pizza:", "🍔": ":hamburger:", "☕": ":coffee:",
}

var shortcodeToEmoji = make(map[string]string)

func init() {
	for emoji, shortcode := range emojiToShortcode {
		shortcodeToEmoji[shortcode] = emoji
	}
}

// Emoji categories
var emojiCategories = map[string][]string{
	"faces": {"😀", "😃", "😄", "😁", "😆", "😅", "🤣", "😂", "🙂", "😉", "😊",
		"😍", "🥰", "😘", "😋", "😎", "🤔", "😐", "😑", "😶", "😏", "😒", "🙄"},
	"gestures": {"👍", "👎", "👏", "🙌", "👋", "✋", "🤝", "🙏"},
	"hearts":   {"❤️", "💔", "💕", "💖", "💗", "💘", "💝"},
	"symbols":  {"⭐", "🌟", "✨", "💫", "🔥", "💯", "✅", "❌", "⚠️", "❗", "❓"},
	"nature":   {"☀️", "🌙", "⛅", "🌧️", "🌈", "🌸", "🌺", "🌻", "🌴"},
	"animals":  {"🐶", "🐱", "🐭", "🐰", "🐻", "🐼", "🐨", "🦁", "🐯"},
	"food":     {"🍎", "🍕", "🍔", "🍟", "🌮", "🍣", "☕", "🍺", "🎂"},
}

// Emoji sentiment mapping
var emojiSentiment = map[string]int{
	"😀": 2, "😃": 2, "😄": 2, "😁": 2, "😆": 2, "😅": 1, "🤣": 2, "😂": 2,
	"🙂": 1, "😉": 1, "😊": 2, "😍": 3, "🥰": 3, "😘": 2, "😋": 1, "😎": 1,
	"👍": 1, "❤️": 3, "💕": 2, "⭐": 1, "🌟": 1, "✨": 1, "🔥": 1, "💯": 2,
	"✅": 1, "🎉": 2, "🎊": 2,
	"😐": 0, "😑": 0, "😶": 0, "🤔": 0, "❓": 0,
	"😏": -1, "😒": -1, "🙄": -1, "😬": -1, "😔": -1, "😕": -1, "🙁": -1,
	"😢": -2, "😭": -2, "😤": -1, "😠": -2, "😡": -2, "🤬": -3,
	"😱": -2, "😨": -2, "😰": -1, "💔": -2, "❌": -1, "⚠️": -1, "👎": -1,
}

func extractTool() tool.Tool {
	return tool.NewBuilder("emoji_extract").
		WithDescription("Extract emojis from text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			emojis := extractEmojis(params.Text)

			// Get unique emojis
			seen := make(map[string]bool)
			var unique []string
			for _, e := range emojis {
				if !seen[e] {
					seen[e] = true
					unique = append(unique, e)
				}
			}

			result := map[string]any{
				"emojis":       emojis,
				"unique":       unique,
				"count":        len(emojis),
				"unique_count": len(unique),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractEmojis(text string) []string {
	var emojis []string
	for _, r := range text {
		if isEmoji(r) {
			emojis = append(emojis, string(r))
		}
	}
	return emojis
}

func isEmoji(r rune) bool {
	// Check common emoji ranges
	return (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
		(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols and Pictographs
		(r >= 0x1F680 && r <= 0x1F6FF) || // Transport and Map
		(r >= 0x1F700 && r <= 0x1F77F) || // Alchemical Symbols
		(r >= 0x1F780 && r <= 0x1F7FF) || // Geometric Shapes Extended
		(r >= 0x1F800 && r <= 0x1F8FF) || // Supplemental Arrows-C
		(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols and Pictographs
		(r >= 0x1FA00 && r <= 0x1FA6F) || // Chess Symbols
		(r >= 0x1FA70 && r <= 0x1FAFF) || // Symbols and Pictographs Extended-A
		(r >= 0x2600 && r <= 0x26FF) || // Misc symbols
		(r >= 0x2700 && r <= 0x27BF) || // Dingbats
		(r >= 0xFE00 && r <= 0xFE0F) || // Variation Selectors
		(r >= 0x1F1E0 && r <= 0x1F1FF) // Flags
}

func removeTool() tool.Tool {
	return tool.NewBuilder("emoji_remove").
		WithDescription("Remove emojis from text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var cleaned strings.Builder
			removedCount := 0

			for _, r := range params.Text {
				if !isEmoji(r) {
					cleaned.WriteRune(r)
				} else {
					removedCount++
				}
			}

			result := map[string]any{
				"original":      params.Text,
				"cleaned":       cleaned.String(),
				"removed_count": removedCount,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countTool() tool.Tool {
	return tool.NewBuilder("emoji_count").
		WithDescription("Count emojis in text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			counts := make(map[string]int)
			total := 0

			for _, r := range params.Text {
				if isEmoji(r) {
					counts[string(r)]++
					total++
				}
			}

			result := map[string]any{
				"total":        total,
				"unique_count": len(counts),
				"counts":       counts,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func replaceTool() tool.Tool {
	return tool.NewBuilder("emoji_replace").
		WithDescription("Replace emojis with text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text         string `json:"text"`
				Replacement  string `json:"replacement,omitempty"`
				UseShortcode bool   `json:"use_shortcode,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var replaced strings.Builder
			replacedCount := 0

			for _, r := range params.Text {
				if isEmoji(r) {
					if params.UseShortcode {
						if shortcode, ok := emojiToShortcode[string(r)]; ok {
							replaced.WriteString(shortcode)
						} else {
							replaced.WriteString(params.Replacement)
						}
					} else {
						replaced.WriteString(params.Replacement)
					}
					replacedCount++
				} else {
					replaced.WriteRune(r)
				}
			}

			result := map[string]any{
				"original":       params.Text,
				"replaced":       replaced.String(),
				"replaced_count": replacedCount,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hasEmojiTool() tool.Tool {
	return tool.NewBuilder("emoji_has").
		WithDescription("Check if text contains emojis").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			hasEmoji := false
			firstEmoji := ""

			for _, r := range params.Text {
				if isEmoji(r) {
					hasEmoji = true
					if firstEmoji == "" {
						firstEmoji = string(r)
					}
				}
			}

			result := map[string]any{
				"has_emoji":   hasEmoji,
				"first_emoji": firstEmoji,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func toShortcodeTool() tool.Tool {
	return tool.NewBuilder("emoji_to_shortcode").
		WithDescription("Convert emoji to shortcode").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Emoji string `json:"emoji"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			shortcode, found := emojiToShortcode[params.Emoji]

			result := map[string]any{
				"emoji":     params.Emoji,
				"shortcode": shortcode,
				"found":     found,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromShortcodeTool() tool.Tool {
	return tool.NewBuilder("emoji_from_shortcode").
		WithDescription("Convert shortcode to emoji").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Shortcode string `json:"shortcode"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			shortcode := params.Shortcode
			if !strings.HasPrefix(shortcode, ":") {
				shortcode = ":" + shortcode
			}
			if !strings.HasSuffix(shortcode, ":") {
				shortcode = shortcode + ":"
			}

			emoji, found := shortcodeToEmoji[shortcode]

			result := map[string]any{
				"shortcode": params.Shortcode,
				"emoji":     emoji,
				"found":     found,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("emoji_list").
		WithDescription("List available emojis").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Category string `json:"category,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Category != "" {
				if emojis, ok := emojiCategories[params.Category]; ok {
					result := map[string]any{
						"category": params.Category,
						"emojis":   emojis,
						"count":    len(emojis),
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
				result := map[string]any{
					"error":                "unknown category",
					"available_categories": getCategoryNames(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"categories": getCategoryNames(),
				"all":        emojiCategories,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getCategoryNames() []string {
	names := make([]string, 0, len(emojiCategories))
	for name := range emojiCategories {
		names = append(names, name)
	}
	return names
}

func categoryTool() tool.Tool {
	return tool.NewBuilder("emoji_category").
		WithDescription("Get category of an emoji").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Emoji string `json:"emoji"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var foundCategory string
			for category, emojis := range emojiCategories {
				for _, e := range emojis {
					if e == params.Emoji {
						foundCategory = category
						break
					}
				}
				if foundCategory != "" {
					break
				}
			}

			result := map[string]any{
				"emoji":    params.Emoji,
				"category": foundCategory,
				"found":    foundCategory != "",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sentimentTool() tool.Tool {
	return tool.NewBuilder("emoji_sentiment").
		WithDescription("Analyze emoji sentiment").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text,omitempty"`
				Emoji string `json:"emoji,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Emoji != "" {
				score, found := emojiSentiment[params.Emoji]
				sentiment := scoreTolabel(score)
				result := map[string]any{
					"emoji":     params.Emoji,
					"score":     score,
					"sentiment": sentiment,
					"found":     found,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Analyze text
			totalScore := 0
			emojiCount := 0
			for _, r := range params.Text {
				if isEmoji(r) {
					if score, ok := emojiSentiment[string(r)]; ok {
						totalScore += score
						emojiCount++
					}
				}
			}

			avgScore := 0.0
			if emojiCount > 0 {
				avgScore = float64(totalScore) / float64(emojiCount)
			}

			result := map[string]any{
				"emoji_count":   emojiCount,
				"total_score":   totalScore,
				"average_score": avgScore,
				"sentiment":     scoreTolabel(int(avgScore)),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func scoreTolabel(score int) string {
	switch {
	case score >= 2:
		return "very_positive"
	case score == 1:
		return "positive"
	case score == 0:
		return "neutral"
	case score == -1:
		return "negative"
	default:
		return "very_negative"
	}
}

// Unused but required for import
var _ = regexp.Compile
var _ = unicode.IsLetter
