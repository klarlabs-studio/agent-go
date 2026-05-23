// Package slack provides Slack integration tools for agent-go.
//
// Tools include messaging, channel management, user lookup, and file operations.
package slack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/slack-go/slack"
)

// Pack returns the Slack tool pack.
func Pack(token string) *pack.Pack {
	p := &slackPack{token: token}

	return pack.NewBuilder("slack").
		WithDescription("Slack integration tools for messaging, channels, and collaboration").
		WithVersion("1.0.0").
		AddTools(
			p.postMessageTool(),
			p.postEphemeralTool(),
			p.updateMessageTool(),
			p.deleteMessageTool(),
			p.addReactionTool(),
			p.removeReactionTool(),
			p.getChannelTool(),
			p.listChannelsTool(),
			p.createChannelTool(),
			p.archiveChannelTool(),
			p.inviteToChannelTool(),
			p.getChannelHistoryTool(),
			p.getUserTool(),
			p.listUsersTool(),
			p.lookupUserByEmailTool(),
			p.uploadFileTool(),
			p.searchMessagesTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type slackPack struct {
	token  string
	client *slack.Client
}

func (p *slackPack) getClient() *slack.Client {
	if p.client == nil {
		p.client = slack.New(p.token)
	}
	return p.client
}

// ============================================================================
// Message Tools
// ============================================================================

func (p *slackPack) postMessageTool() tool.Tool {
	return tool.NewBuilder("slack_post_message").
		WithDescription("Post a message to a Slack channel").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel     string `json:"channel"` // channel ID or name
				Text        string `json:"text"`
				ThreadTS    string `json:"thread_ts,omitempty"`
				Unfurl      bool   `json:"unfurl_links,omitempty"`
				UnfurlMedia bool   `json:"unfurl_media,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			opts := []slack.MsgOption{
				slack.MsgOptionText(in.Text, false),
			}
			if in.ThreadTS != "" {
				opts = append(opts, slack.MsgOptionTS(in.ThreadTS))
			}
			if in.Unfurl {
				opts = append(opts, slack.MsgOptionEnableLinkUnfurl())
			}

			_, ts, err := client.PostMessageContext(ctx, in.Channel, opts...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to post message: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"channel":   in.Channel,
				"timestamp": ts,
				"success":   true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) postEphemeralTool() tool.Tool {
	return tool.NewBuilder("slack_post_ephemeral").
		WithDescription("Post an ephemeral message visible only to a specific user").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel string `json:"channel"`
				User    string `json:"user"`
				Text    string `json:"text"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			ts, err := client.PostEphemeralContext(ctx, in.Channel, in.User,
				slack.MsgOptionText(in.Text, false))
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to post ephemeral: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"timestamp": ts,
				"success":   true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) updateMessageTool() tool.Tool {
	return tool.NewBuilder("slack_update_message").
		WithDescription("Update an existing message").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel   string `json:"channel"`
				Timestamp string `json:"timestamp"`
				Text      string `json:"text"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			_, _, _, err := client.UpdateMessageContext(ctx, in.Channel, in.Timestamp,
				slack.MsgOptionText(in.Text, false))
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to update message: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) deleteMessageTool() tool.Tool {
	return tool.NewBuilder("slack_delete_message").
		WithDescription("Delete a message").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel   string `json:"channel"`
				Timestamp string `json:"timestamp"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			_, _, err := client.DeleteMessageContext(ctx, in.Channel, in.Timestamp)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete message: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Reaction Tools
// ============================================================================

func (p *slackPack) addReactionTool() tool.Tool {
	return tool.NewBuilder("slack_add_reaction").
		WithDescription("Add a reaction to a message").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel   string `json:"channel"`
				Timestamp string `json:"timestamp"`
				Emoji     string `json:"emoji"` // without colons
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			err := client.AddReactionContext(ctx, in.Emoji, slack.ItemRef{
				Channel:   in.Channel,
				Timestamp: in.Timestamp,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to add reaction: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) removeReactionTool() tool.Tool {
	return tool.NewBuilder("slack_remove_reaction").
		WithDescription("Remove a reaction from a message").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel   string `json:"channel"`
				Timestamp string `json:"timestamp"`
				Emoji     string `json:"emoji"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			err := client.RemoveReactionContext(ctx, in.Emoji, slack.ItemRef{
				Channel:   in.Channel,
				Timestamp: in.Timestamp,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to remove reaction: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Channel Tools
// ============================================================================

func (p *slackPack) getChannelTool() tool.Tool {
	return tool.NewBuilder("slack_get_channel").
		WithDescription("Get channel information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel string `json:"channel"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			ch, err := client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
				ChannelID: in.Channel,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get channel: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":           ch.ID,
				"name":         ch.Name,
				"topic":        ch.Topic.Value,
				"purpose":      ch.Purpose.Value,
				"is_private":   ch.IsPrivate,
				"is_archived":  ch.IsArchived,
				"member_count": ch.NumMembers,
				"created":      ch.Created,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) listChannelsTool() tool.Tool {
	return tool.NewBuilder("slack_list_channels").
		WithDescription("List channels in the workspace").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ExcludeArchived bool `json:"exclude_archived"`
				Limit           int  `json:"limit"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Limit == 0 {
				in.Limit = 100
			}

			client := p.getClient()
			channels, _, err := client.GetConversationsContext(ctx, &slack.GetConversationsParameters{
				ExcludeArchived: in.ExcludeArchived,
				Limit:           in.Limit,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list channels: %w", err)
			}

			result := make([]map[string]any, len(channels))
			for i, ch := range channels {
				result[i] = map[string]any{
					"id":          ch.ID,
					"name":        ch.Name,
					"is_private":  ch.IsPrivate,
					"is_archived": ch.IsArchived,
					"topic":       ch.Topic.Value,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":    len(result),
				"channels": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) createChannelTool() tool.Tool {
	return tool.NewBuilder("slack_create_channel").
		WithDescription("Create a new channel").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Name      string `json:"name"`
				IsPrivate bool   `json:"is_private"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			ch, err := client.CreateConversationContext(ctx, slack.CreateConversationParams{
				ChannelName: in.Name,
				IsPrivate:   in.IsPrivate,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create channel: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":         ch.ID,
				"name":       ch.Name,
				"is_private": ch.IsPrivate,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) archiveChannelTool() tool.Tool {
	return tool.NewBuilder("slack_archive_channel").
		WithDescription("Archive a channel").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel string `json:"channel"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			err := client.ArchiveConversationContext(ctx, in.Channel)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to archive channel: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) inviteToChannelTool() tool.Tool {
	return tool.NewBuilder("slack_invite_to_channel").
		WithDescription("Invite users to a channel").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel string   `json:"channel"`
				Users   []string `json:"users"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			_, err := client.InviteUsersToConversationContext(ctx, in.Channel, in.Users...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to invite users: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) getChannelHistoryTool() tool.Tool {
	return tool.NewBuilder("slack_get_channel_history").
		WithDescription("Get message history from a channel").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channel string `json:"channel"`
				Limit   int    `json:"limit"`
				Oldest  string `json:"oldest,omitempty"`
				Latest  string `json:"latest,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Limit == 0 {
				in.Limit = 50
			}

			client := p.getClient()
			params := &slack.GetConversationHistoryParameters{
				ChannelID: in.Channel,
				Limit:     in.Limit,
			}
			if in.Oldest != "" {
				params.Oldest = in.Oldest
			}
			if in.Latest != "" {
				params.Latest = in.Latest
			}

			history, err := client.GetConversationHistoryContext(ctx, params)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get history: %w", err)
			}

			messages := make([]map[string]any, len(history.Messages))
			for i, msg := range history.Messages {
				messages[i] = map[string]any{
					"timestamp": msg.Timestamp,
					"user":      msg.User,
					"text":      msg.Text,
					"type":      msg.Type,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":    len(messages),
				"messages": messages,
				"has_more": history.HasMore,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// User Tools
// ============================================================================

func (p *slackPack) getUserTool() tool.Tool {
	return tool.NewBuilder("slack_get_user").
		WithDescription("Get user information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				User string `json:"user"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			user, err := client.GetUserInfoContext(ctx, in.User)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get user: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":        user.ID,
				"name":      user.Name,
				"real_name": user.RealName,
				"email":     user.Profile.Email,
				"title":     user.Profile.Title,
				"is_admin":  user.IsAdmin,
				"is_bot":    user.IsBot,
				"timezone":  user.TZ,
				"deleted":   user.Deleted,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) listUsersTool() tool.Tool {
	return tool.NewBuilder("slack_list_users").
		WithDescription("List users in the workspace").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Limit int `json:"limit"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Limit == 0 {
				in.Limit = 100
			}

			client := p.getClient()
			users, err := client.GetUsersContext(ctx, slack.GetUsersOptionLimit(in.Limit))
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list users: %w", err)
			}

			result := make([]map[string]any, 0, len(users))
			for _, u := range users {
				if u.Deleted {
					continue
				}
				result = append(result, map[string]any{
					"id":        u.ID,
					"name":      u.Name,
					"real_name": u.RealName,
					"is_admin":  u.IsAdmin,
					"is_bot":    u.IsBot,
				})
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(result),
				"users": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *slackPack) lookupUserByEmailTool() tool.Tool {
	return tool.NewBuilder("slack_lookup_user_by_email").
		WithDescription("Look up a user by email address").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Email string `json:"email"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			user, err := client.GetUserByEmailContext(ctx, in.Email)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to lookup user: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":        user.ID,
				"name":      user.Name,
				"real_name": user.RealName,
				"email":     user.Profile.Email,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// File Tools
// ============================================================================

func (p *slackPack) uploadFileTool() tool.Tool {
	return tool.NewBuilder("slack_upload_file").
		WithDescription("Upload a file to Slack").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Channels []string `json:"channels"`
				Content  string   `json:"content"`
				Filename string   `json:"filename"`
				Title    string   `json:"title"`
				Comment  string   `json:"initial_comment"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client := p.getClient()
			// slack-go v0.23 renamed FileUploadParameters to
			// UploadFileParameters and changed the return type to
			// *FileSummary (narrower: ID + Title only).
			// v0.23 collapsed Channels[] to a single Channel field on
			// the upload params. Pick the first requested channel; the
			// caller can issue additional shares via files.share if
			// they need multi-channel uploads.
			var channel string
			if len(in.Channels) > 0 {
				channel = in.Channels[0]
			}
			file, err := client.UploadFileContext(ctx, slack.UploadFileParameters{
				Channel:        channel,
				Content:        in.Content,
				Filename:       in.Filename,
				Title:          in.Title,
				InitialComment: in.Comment,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to upload file: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":    file.ID,
				"title": file.Title,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Search Tools
// ============================================================================

func (p *slackPack) searchMessagesTool() tool.Tool {
	return tool.NewBuilder("slack_search_messages").
		WithDescription("Search for messages in Slack").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query string `json:"query"`
				Count int    `json:"count"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Count == 0 {
				in.Count = 20
			}

			client := p.getClient()
			results, err := client.SearchMessagesContext(ctx, in.Query, slack.SearchParameters{
				Count: in.Count,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to search: %w", err)
			}

			matches := make([]map[string]any, len(results.Matches))
			for i, m := range results.Matches {
				matches[i] = map[string]any{
					"channel":   m.Channel.Name,
					"user":      m.User,
					"text":      m.Text,
					"timestamp": m.Timestamp,
					"permalink": m.Permalink,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"total":   results.Total,
				"count":   len(matches),
				"matches": matches,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
